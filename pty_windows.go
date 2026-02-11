//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

// PTY represents a pseudo-terminal interface for Windows
// On Windows, we use a simpler approach since ConPTY requires complex setup
type PTY struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	cmd    *exec.Cmd
	width  int
	height int
}

// NewPTY creates a new PTY with the specified shell on Windows
func NewPTY(shell string) (*PTY, error) {
	// On Windows, we use cmd.exe or PowerShell
	if shell == "" {
		shell = GetDefaultShell()
	}

	cmd := exec.Command(shell)

	// Get pipes for stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	return &PTY{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		cmd:    cmd,
	}, nil
}

// Read reads from the PTY (from stdout)
func (p *PTY) Read(buf []byte) (int, error) {
	return p.stdout.Read(buf)
}

// Write writes to the PTY (to stdin)
func (p *PTY) Write(buf []byte) (int, error) {
	return p.stdin.Write(buf)
}

// Close closes the PTY
func (p *PTY) Close() error {
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.stderr != nil {
		p.stderr.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	return nil
}

// Resize resizes the PTY
// Note: On Windows without ConPTY, this is a no-op
func (p *PTY) Resize(width, height int) error {
	p.width = width
	p.height = height
	// ConPTY would be required for proper resizing on Windows
	// For now, we accept the dimensions but don't resize the console
	return nil
}

// SetSize sets the PTY size (alias for Resize)
func (p *PTY) SetSize(width, height int) error {
	return p.Resize(width, height)
}

// GetDefaultShell returns the default shell for Windows
func GetDefaultShell() string {
	// Try to find PowerShell first
	psPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := os.Stat(psPath); err == nil {
		return psPath
	}

	// Fall back to cmd.exe
	cmdPath := `C:\Windows\System32\cmd.exe`
	if _, err := os.Stat(cmdPath); err == nil {
		return cmdPath
	}

	// Last resort: try to find in PATH
	if ps, err := exec.LookPath("powershell.exe"); err == nil {
		return ps
	}
	if cmd, err := exec.LookPath("cmd.exe"); err == nil {
		return cmd
	}

	return `C:\Windows\System32\cmd.exe`
}

// IsTerminal checks if the given file descriptor is a terminal on Windows
func IsTerminal(fd int) bool {
	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(fd), &mode)
	return err == nil
}

// GetTerminalSize returns the size of the terminal on Windows
func GetTerminalSize(fd int) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	err = windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info)
	if err != nil {
		return 0, 0, err
	}

	width = int(info.Window.Right - info.Window.Left + 1)
	height = int(info.Window.Bottom - info.Window.Top + 1)
	return width, height, nil
}

// WindowsTerminalState stores the previous console mode
type WindowsTerminalState struct {
	stdinMode  uint32
	stdoutMode uint32
}

// MakeRaw puts the terminal into raw mode on Windows
func MakeRaw(fd int) (*WindowsTerminalState, error) {
	var state WindowsTerminalState

	// Get current console mode for stdin
	err := windows.GetConsoleMode(windows.Handle(fd), &state.stdinMode)
	if err != nil {
		return nil, err
	}

	// Set raw mode (disable echo, line input, etc.)
	rawMode := state.stdinMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT)
	rawMode |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	err = windows.SetConsoleMode(windows.Handle(fd), rawMode)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

// Restore restores the terminal to a previous state on Windows
func Restore(fd int, state *WindowsTerminalState) error {
	if state == nil {
		return nil
	}
	return windows.SetConsoleMode(windows.Handle(fd), state.stdinMode)
}

// SetupTerminal prepares the terminal for the TUI on Windows
func SetupTerminal() (*WindowsTerminalState, error) {
	return MakeRaw(int(os.Stdin.Fd()))
}

// RestoreTerminal restores the terminal to normal mode on Windows
func RestoreTerminal(state *WindowsTerminalState) error {
	return Restore(int(os.Stdin.Fd()), state)
}

// KillProcess kills a process on Windows
func KillProcess(process *os.Process) error {
	return process.Kill()
}

// IsWindows returns true for Windows systems
func IsWindows() bool {
	return true
}

// PathSeparator returns the OS-specific path separator
func PathSeparator() string {
	return "\\"
}
