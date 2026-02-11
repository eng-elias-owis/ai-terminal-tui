//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// PTY represents a pseudo-terminal interface
type PTY struct {
	file   *os.File
	cmd    *exec.Cmd
	width  int
	height int
}

// PTYInterface defines the cross-platform PTY interface
type PTYInterface interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Close() error
	Resize(width, height int) error
	SetSize(width, height int) error
}

// NewPTY creates a new PTY with the specified shell
func NewPTY(shell string) (*PTY, error) {
	cmd := exec.Command(shell)

	// Start the command with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &PTY{
		file: ptmx,
		cmd:  cmd,
	}, nil
}

// Read reads from the PTY
func (p *PTY) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

// Write writes to the PTY
func (p *PTY) Write(buf []byte) (int, error) {
	return p.file.Write(buf)
}

// Close closes the PTY
func (p *PTY) Close() error {
	if p.file != nil {
		p.file.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	return nil
}

// Resize resizes the PTY
func (p *PTY) Resize(width, height int) error {
	p.width = width
	p.height = height
	return pty.Setsize(p.file, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
}

// SetSize sets the PTY size (alias for Resize)
func (p *PTY) SetSize(width, height int) error {
	return p.Resize(width, height)
}

// GetDefaultShell returns the default shell for Unix systems
func GetDefaultShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell
}

// IsTerminal checks if the given file descriptor is a terminal
func IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

// GetTerminalSize returns the size of the terminal
func GetTerminalSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}

// MakeRaw puts the terminal into raw mode
func MakeRaw(fd int) (*term.State, error) {
	return term.MakeRaw(fd)
}

// Restore restores the terminal to a previous state
func Restore(fd int, state *term.State) error {
	return term.Restore(fd, state)
}

// SetupTerminal prepares the terminal for the TUI
func SetupTerminal() (*term.State, error) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	return oldState, nil
}

// RestoreTerminal restores the terminal to normal mode
func RestoreTerminal(state *term.State) error {
	if state != nil {
		return term.Restore(int(os.Stdin.Fd()), state)
	}
	return nil
}

// KillProcess kills a process by sending the appropriate signal
func KillProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

// IsWindows returns false for Unix systems
func IsWindows() bool {
	return false
}

// PathSeparator returns the OS-specific path separator
func PathSeparator() string {
	return "/"
}
