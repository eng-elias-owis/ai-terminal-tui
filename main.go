package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
)

// Config represents the application configuration
type Config struct {
	LiteLLMURL    string `json:"litellm_url"`
	LiteLLMToken  string `json:"litellm_token"`
	Model         string `json:"model"`
	Shell         string `json:"shell"`
}

// Default configuration
func defaultConfig() Config {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return Config{
		LiteLLMURL:   "http://localhost:4000",
		LiteLLMToken: "",
		Model:        "gpt-4",
		Shell:        shell,
	}
}

// LoadConfig loads configuration from file or returns defaults
func LoadConfig() Config {
	config := defaultConfig()
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return config
	}
	
	configPath := filepath.Join(homeDir, ".config", "ai-terminal-tui", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config
	}
	
	json.Unmarshal(data, &config)
	return config
}

// Model represents the Bubble Tea application state
type Model struct {
	config      Config
	pty         *os.File
	cmd         *exec.Cmd
	output      []byte
	width       int
	height      int
	showPrompt  bool
	input       textinput.Model
	aiResponse  string
	loading     bool
	err         error
	cursorX     int
	cursorY     int
}

// Messages
type (
	ptyMsg      []byte
	aiResponse  string
	errMsg      error
	resizeMsg   struct{ width, height int }
)

// NewModel creates a new application model
func NewModel() Model {
	config := LoadConfig()
	
	ti := textinput.New()
	 ti.Placeholder = "Describe what you want to do..."
	 ti.Focus()
	 ti.CharLimit = 200
	 ti.Width = 50
	 
	return Model{
		config:  config,
		input:   ti,
		output:  make([]byte, 0),
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.initPTY(),
		tick(),
	)
}

// initPTY initializes the PTY and shell
func (m *Model) initPTY() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(m.config.Shell)
		
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return errMsg(err)
		}
		
		m.pty = ptmx
		m.cmd = cmd
		
		// Read from PTY
		return m.readPTY()
	}
}

// readPTY reads output from the PTY
func (m *Model) readPTY() tea.Msg {
	if m.pty == nil {
		return nil
	}
	
	buf := make([]byte, 4096)
	n, err := m.pty.Read(buf)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return errMsg(err)
	}
	
	return ptyMsg(buf[:n])
}

// tick creates a command that reads from PTY periodically
func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return t
	})
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle Ctrl+K to toggle AI prompt
		if msg.Type == tea.KeyCtrlK {
			m.showPrompt = !m.showPrompt
			if m.showPrompt {
				m.input.Focus()
			} else {
				m.input.Blur()
				// Return focus to terminal
				if m.pty != nil {
					m.pty.Write([]byte{0})
				}
			}
			return m, nil
		}
		
		// Handle escape to close prompt
		if msg.Type == tea.KeyEsc && m.showPrompt {
			m.showPrompt = false
			m.input.Blur()
			return m, nil
		}
		
		// Handle enter in AI prompt
		if msg.Type == tea.KeyEnter && m.showPrompt {
			query := m.input.Value()
			if query != "" {
				m.loading = true
				m.input.SetValue("")
				return m, m.queryAI(query)
			}
			m.showPrompt = false
			return m, nil
		}
		
		// Pass keys to text input when prompt is shown
		if m.showPrompt {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		
		// Pass keys to PTY when prompt is not shown
		if m.pty != nil {
			if key := teaKeyToBytes(msg); key != nil {
				m.pty.Write(key)
			}
		}
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Resize PTY
		if m.pty != nil {
			pty.Setsize(m.pty, &pty.Winsize{
				Rows: uint16(m.height - 3),
				Cols: uint16(m.width),
			})
		}
		
	case ptyMsg:
		m.output = append(m.output, msg...)
		// Keep output buffer manageable
		if len(m.output) > 100000 {
			m.output = m.output[len(m.output)-50000:]
		}
		return m, m.readPTY
		
	case aiResponse:
		m.aiResponse = string(msg)
		m.loading = false
		// Execute the command in the shell
		if m.pty != nil && m.aiResponse != "" {
			cmd := strings.TrimSpace(m.aiResponse)
			if cmd != "" {
				m.pty.Write([]byte(cmd + "\n"))
			}
		}
		m.showPrompt = false
		m.input.Blur()
		return m, nil
		
	case errMsg:
		m.err = msg
		return m, nil
		
	case time.Time:
		// Periodic tick for PTY reading
		return m, tea.Batch(m.readPTY, tick())
	}
	
	return m, nil
}

// teaKeyToBytes converts a Bubble Tea key message to terminal escape sequences
func teaKeyToBytes(k tea.KeyMsg) []byte {
	switch k.Type {
	case tea.KeyEnter:
		return []byte{13}
	case tea.KeyBackspace:
		return []byte{127}
	case tea.KeyTab:
		return []byte{9}
	case tea.KeyEsc:
		return []byte{27}
	case tea.KeyUp:
		return []byte{27, 91, 65}
	case tea.KeyDown:
		return []byte{27, 91, 66}
	case tea.KeyLeft:
		return []byte{27, 91, 68}
	case tea.KeyRight:
		return []byte{27, 91, 67}
	case tea.KeyHome:
		return []byte{27, 91, 72}
	case tea.KeyEnd:
		return []byte{27, 91, 70}
	case tea.KeyDelete:
		return []byte{27, 91, 51, 126}
	case tea.KeyPgUp:
		return []byte{27, 91, 53, 126}
	case tea.KeyPgDown:
		return []byte{27, 91, 54, 126}
	case tea.KeySpace:
		return []byte{32}
	case tea.KeyCtrlC:
		return []byte{3}
	case tea.KeyCtrlD:
		return []byte{4}
	case tea.KeyCtrlZ:
		return []byte{26}
	case tea.KeyCtrlL:
		return []byte{12}
	case tea.KeyCtrlA:
		return []byte{1}
	case tea.KeyCtrlE:
		return []byte{5}
	case tea.KeyCtrlU:
		return []byte{21}
	case tea.KeyCtrlK:
		// Handled separately
		return nil
	case tea.KeyRunes:
		return []byte(string(k.Runes))
	default:
		if len(k.Runes) > 0 {
			return []byte(string(k.Runes))
		}
		return nil
	}
}

// queryAI sends a query to the LiteLLM API
func (m Model) queryAI(query string) tea.Cmd {
	return func() tea.Msg {
		prompt := fmt.Sprintf(
			"You are a helpful assistant that converts natural language descriptions into shell commands. "+
			"Respond with ONLY the command, no explanations, no markdown formatting, no quotes. "+
			"If you're unsure, provide the most likely command.\n\n"+
			"User request: %s\n\n"+
			"Shell command:",
			query,
		)
		
		requestBody := map[string]interface{}{
			"model": m.config.Model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"temperature": 0.1,
			"max_tokens":  200,
		}
		
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return errMsg(err)
		}
		
		url := strings.TrimSuffix(m.config.LiteLLMURL, "/") + "/v1/chat/completions"
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return errMsg(err)
		}
		
		req.Header.Set("Content-Type", "application/json")
		if m.config.LiteLLMToken != "" {
			req.Header.Set("Authorization", "Bearer "+m.config.LiteLLMToken)
		}
		
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return errMsg(err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body)))
		}
		
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return errMsg(err)
		}
		
		if len(result.Choices) > 0 {
			content := strings.TrimSpace(result.Choices[0].Message.Content)
			// Remove any markdown code block formatting
			content = strings.TrimPrefix(content, "```bash")
			content = strings.TrimPrefix(content, "```sh")
			content = strings.TrimPrefix(content, "```shell")
			content = strings.TrimPrefix(content, "```")
			content = strings.TrimSuffix(content, "```")
			return aiResponse(strings.TrimSpace(content))
		}
		
		return aiResponse("")
	}
}

// View renders the UI
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	
	// Terminal output
	termHeight := m.height
	if m.showPrompt {
		termHeight = m.height - 5
	}
	
	// Truncate and format output
	output := string(m.output)
	lines := strings.Split(output, "\n")
	if len(lines) > termHeight-2 {
		lines = lines[len(lines)-(termHeight-2):]
	}
	
	// Style the terminal area
	terminalStyle := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(termHeight - 2).
		Padding(0, 1)
	
	terminalContent := terminalStyle.Render(strings.Join(lines, "\n"))
	
	// Show AI prompt overlay if active
	if m.showPrompt {
		// Prompt box styling
		promptStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("10")).
			Background(lipgloss.Color("0")).
			Padding(1, 2).
			Width(m.width - 4)
		
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)
		
		var promptContent string
		if m.loading {
			promptContent = "Generating command..."
		} else {
			promptContent = fmt.Sprintf(
				"%s\n%s\n\n%s",
				titleStyle.Render("AI Command Generator (Ctrl+K to toggle, Enter to send, Esc to cancel)"),
				m.input.View(),
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Describe what you want to do and press Enter"),
			)
		}
		
		promptBox := promptStyle.Render(promptContent)
		
		// Stack terminal and prompt
		return lipgloss.JoinVertical(
			lipgloss.Left,
			terminalContent,
			promptBox,
		)
	}
	
	return terminalContent
}

// Cleanup performs cleanup on exit
func (m *Model) Cleanup() {
	if m.pty != nil {
		m.pty.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
}

func main() {
	// Ensure config directory exists
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configDir := filepath.Join(homeDir, ".config", "ai-terminal-tui")
		os.MkdirAll(configDir, 0755)
	}
	
	model := NewModel()
	
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	
	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	// Cleanup
	if finalModel, ok := m.(Model); ok {
		finalModel.Cleanup()
	}
}
