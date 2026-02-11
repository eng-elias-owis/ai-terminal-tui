package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// Version information - these are set by the build process
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const AppName = "ai-terminal-tui"

// Config represents the application configuration
type Config struct {
	LiteLLMURL   string `json:"litellm_url"`
	LiteLLMToken string `json:"litellm_token"`
	Model        string `json:"model"`
	Shell        string `json:"shell"`
}

// Default configuration
func defaultConfig() Config {
	return Config{
		LiteLLMURL:   "http://localhost:4000",
		LiteLLMToken: "",
		Model:        "gpt-4",
		Shell:        GetDefaultShell(),
	}
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// On Windows, use a different path
	if runtime.GOOS == "windows" {
		return getWindowsConfigPath()
	}

	return filepath.Join(homeDir, ".config", "ai-terminal-tui", "config.json")
}

// getWindowsConfigPath returns the config path for Windows
func getWindowsConfigPath() string {
	// Try APPDATA first
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "ai-terminal-tui", "config.json")
	}

	// Fall back to LOCALAPPDATA
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "ai-terminal-tui", "config.json")
	}

	// Last resort: use UserProfile
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		return filepath.Join(userProfile, ".config", "ai-terminal-tui", "config.json")
	}

	return ""
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	var configDir string
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			configDir = filepath.Join(appData, "ai-terminal-tui")
		} else if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			configDir = filepath.Join(localAppData, "ai-terminal-tui")
		} else {
			configDir = filepath.Join(homeDir, ".config", "ai-terminal-tui")
		}
	} else {
		configDir = filepath.Join(homeDir, ".config", "ai-terminal-tui")
	}

	return os.MkdirAll(configDir, 0755)
}

// LoadConfig loads configuration from file or returns defaults
func LoadConfig() Config {
	config := defaultConfig()

	configPath := GetConfigPath()
	if configPath == "" {
		return config
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config
	}

	json.Unmarshal(data, &config)
	return config
}

// SaveConfig saves the configuration to file
func SaveConfig(config Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configPath := GetConfigPath()
	if configPath == "" {
		return fmt.Errorf("unable to determine config path")
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// UpdateConfigKey updates a single configuration key
func UpdateConfigKey(key, value string) error {
	config := LoadConfig()

	switch key {
	case "litellm_url":
		config.LiteLLMURL = value
	case "litellm_token":
		config.LiteLLMToken = value
	case "model":
		config.Model = value
	case "shell":
		config.Shell = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return SaveConfig(config)
}

// DisplayConfig prints the current configuration
func DisplayConfig() {
	config := LoadConfig()
	configPath := GetConfigPath()

	fmt.Printf("Configuration file: %s\n\n", configPath)
	fmt.Printf("  litellm_url:   %s\n", config.LiteLLMURL)
	fmt.Printf("  litellm_token: %s\n", maskToken(config.LiteLLMToken))
	fmt.Printf("  model:         %s\n", config.Model)
	fmt.Printf("  shell:         %s\n", config.Shell)
}

// maskToken masks the token for display
func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// IsTTY returns true if stdout is a terminal
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// Model represents the Bubble Tea application state
type Model struct {
	config     Config
	pty        *PTY
	output     []byte
	width      int
	height     int
	showPrompt bool
	input      textinput.Model
	aiResponse string
	loading    bool
	err        error
}

// Messages
type (
	ptyMsg     []byte
	aiResponseMsg string
	errMsg     error
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
		config: config,
		input:  ti,
		output: make([]byte, 0),
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
		pty, err := NewPTY(m.config.Shell)
		if err != nil {
			return errMsg(err)
		}

		m.pty = pty

		// Read from PTY
		return func() tea.Msg {
			return m.readPTYMsg()
		}
	}
}

// readPTYMsg reads output from the PTY and returns a message
func (m *Model) readPTYMsg() tea.Msg {
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

// readPTY returns a command that reads from the PTY
func (m *Model) readPTY() tea.Cmd {
	return func() tea.Msg {
		return m.readPTYMsg()
	}
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
			m.pty.Resize(m.width, m.height-3)
		}

	case ptyMsg:
		m.output = append(m.output, msg...)
		// Keep output buffer manageable
		if len(m.output) > 100000 {
			m.output = m.output[len(m.output)-50000:]
		}
		return m, m.readPTY()

	case aiResponseMsg:
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
		return m, tea.Batch(m.readPTY(), tick())
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
		response, err := GenerateCommand(m.config, query)
		if err != nil {
			return errMsg(err)
		}
		return aiResponseMsg(response)
	}
}

// GenerateCommand generates a shell command from a natural language query
func GenerateCommand(config Config, query string) (string, error) {
	prompt := fmt.Sprintf(
		"You are a helpful assistant that converts natural language descriptions into shell commands. "+
			"Respond with ONLY the command, no explanations, no markdown formatting, no quotes. "+
			"If you're unsure, provide the most likely command.\n\n"+
			"User request: %s\n\n"+
			"Shell command:",
		query,
	)

	requestBody := map[string]interface{}{
		"model": config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  200,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	url := strings.TrimSuffix(config.LiteLLMURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if config.LiteLLMToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.LiteLLMToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		content := strings.TrimSpace(result.Choices[0].Message.Content)
		// Remove any markdown code block formatting
		content = strings.TrimPrefix(content, "```bash")
		content = strings.TrimPrefix(content, "```sh")
		content = strings.TrimPrefix(content, "```shell")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		return strings.TrimSpace(content), nil
	}

	return "", fmt.Errorf("no response from AI")
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
}

// printVersion prints version information
func printVersion() {
	fmt.Printf("%s version %s\n", AppName, Version)
	fmt.Printf("  Build time: %s\n", BuildTime)
	fmt.Printf("  Git commit: %s\n", GitCommit)
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// printHelp prints the help message
func printHelp() {
	fmt.Printf(`%s - AI Terminal TUI with headless support

Version: %s

USAGE:
  ai-terminal-tui [COMMAND] [OPTIONS]

COMMANDS:
  version                   Show version information
  setup                     Interactive setup wizard
  config                    Show current configuration
  config --show             Same as 'config'
  config --set-key KEY VALUE  Set a configuration value
  generate "QUERY"          Generate shell command from description (headless)
  --help, -h                Show this help message
  --version, -v             Show version information

CONFIGURATION KEYS:
  litellm_url    - LiteLLM API URL (default: http://localhost:4000)
  litellm_token  - LiteLLM API token
  model          - Model to use (default: gpt-4)
  shell          - Shell to use (default: auto-detected)

EXAMPLES:
  # Run TUI mode (requires TTY)
  ai-terminal-tui

  # Show version
  ai-terminal-tui version

  # Setup wizard
  ai-terminal-tui setup

  # Configure API URL
  ai-terminal-tui config --set-key litellm_url "http://localhost:4000"

  # Configure API token
  ai-terminal-tui config --set-key litellm_token "sk-..."

  # Generate command (headless mode)
  ai-terminal-tui generate "list all files"

  # Generate and execute command
  ai-terminal-tui generate "show disk usage" | sh

MODES:
  TTY Mode    - When run in a terminal with TTY, starts the interactive TUI
  CLI Mode    - When run without TTY or with arguments, uses CLI commands

`, AppName, Version)
}

// runSetupWizard runs the interactive setup wizard
func runSetupWizard() {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║     AI Terminal TUI - Setup Wizard                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	config := LoadConfig()

	// LiteLLM URL
	fmt.Printf("LiteLLM URL [%s]: ", config.LiteLLMURL)
	var url string
	fmt.Scanln(&url)
	if url != "" {
		config.LiteLLMURL = url
	}

	// LiteLLM Token
	fmt.Printf("LiteLLM Token [%s]: ", maskToken(config.LiteLLMToken))
	var token string
	fmt.Scanln(&token)
	if token != "" {
		config.LiteLLMToken = token
	}

	// Model
	fmt.Printf("Model [%s]: ", config.Model)
	var model string
	fmt.Scanln(&model)
	if model != "" {
		config.Model = model
	}

	// Shell
	fmt.Printf("Shell [%s]: ", config.Shell)
	var shell string
	fmt.Scanln(&shell)
	if shell != "" {
		config.Shell = shell
	}

	fmt.Println()

	// Save configuration
	if err := SaveConfig(config); err != nil {
		fmt.Printf("Error saving configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Configuration saved successfully!")
	fmt.Printf("  Location: %s\n", GetConfigPath())
	fmt.Println()
	fmt.Println("You can now run 'ai-terminal-tui' to start the TUI.")
}

// handleConfigCommand handles the config subcommand
func handleConfigCommand(args []string) {
	if len(args) == 0 {
		// Show config
		DisplayConfig()
		return
	}

	// Parse flags
	setKey := ""
	setValue := ""
	showFlag := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--show":
			showFlag = true
		case "--set-key":
			if i+2 < len(args) {
				setKey = args[i+1]
				setValue = args[i+2]
				i += 2
			} else {
				fmt.Println("Error: --set-key requires KEY and VALUE arguments")
				os.Exit(1)
			}
		}
	}

	if showFlag {
		DisplayConfig()
		return
	}

	if setKey != "" {
		if err := UpdateConfigKey(setKey, setValue); err != nil {
			fmt.Printf("Error updating config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Updated %s = %s\n", setKey, setValue)
		return
	}

	// If no recognized flags, show help
	fmt.Println("Usage: ai-terminal-tui config [--show] [--set-key KEY VALUE]")
}

// handleGenerateCommand handles the generate subcommand
func handleGenerateCommand(query string) {
	if query == "" {
		fmt.Println("Error: generate command requires a query string")
		fmt.Println("Usage: ai-terminal-tui generate \"your query here\"")
		os.Exit(1)
	}

	config := LoadConfig()

	// Validate config
	if config.LiteLLMURL == "" {
		fmt.Println("Error: litellm_url not configured. Run 'ai-terminal-tui setup' first.")
		os.Exit(1)
	}

	response, err := GenerateCommand(config, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(response)
}

// runTUIMode starts the TUI application
func runTUIMode() {
	// Check if we actually have a TTY
	if !IsTTY() {
		fmt.Println("Error: No TTY detected. Cannot run TUI mode.")
		fmt.Println("Use CLI commands instead:")
		fmt.Println("  ai-terminal-tui --help")
		fmt.Println("  ai-terminal-tui generate \"your query\"")
		os.Exit(1)
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

func main() {
	// Ensure config directory exists
	EnsureConfigDir()

	// Check if running with arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			printHelp()
			os.Exit(0)

		case "--version", "-v", "version":
			printVersion()
			os.Exit(0)

		case "setup":
			runSetupWizard()
			os.Exit(0)

		case "config":
			handleConfigCommand(os.Args[2:])
			os.Exit(0)

		case "generate":
			if len(os.Args) > 2 {
				handleGenerateCommand(os.Args[2])
			} else {
				handleGenerateCommand("")
			}
			os.Exit(0)

		default:
			// Check if it's a flag we don't recognize
			if strings.HasPrefix(os.Args[1], "-") {
				fmt.Printf("Unknown option: %s\n\n", os.Args[1])
				printHelp()
				os.Exit(1)
			}
			// Treat as generate command
			handleGenerateCommand(os.Args[1])
			os.Exit(0)
		}
	}

	// No arguments - check for TTY and run appropriate mode
	if IsTTY() {
		runTUIMode()
	} else {
		// No TTY and no arguments - show help
		fmt.Println("AI Terminal TUI - Headless/CLI Mode")
		fmt.Println()
		fmt.Println("No TTY detected and no command specified.")
		fmt.Println()
		printHelp()
	}
}
