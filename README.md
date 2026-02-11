# AI Terminal TUI

An AI-powered terminal application built with Go and the Bubble Tea framework. This TUI (Terminal User Interface) combines a traditional shell with an AI assistant that can generate and execute shell commands from natural language descriptions.

## Features

- 🖥️ **Full Terminal Emulator** - Runs a real shell (bash/zsh) in a PTY with full terminal emulation
- 🤖 **AI Command Generation** - Press `Ctrl+K` to open the AI prompt and describe what you want to do
- ⌨️ **Keyboard Shortcuts** - Intuitive controls for toggling AI assistant and navigation
- 🎨 **Beautiful UI** - Green-themed styling with syntax highlighting using Lipgloss
- ⚡ **LiteLLM Integration** - Works with any LiteLLM-compatible API endpoint
- 🔄 **Real-time Output** - Streams shell output to the screen in real-time
- 📐 **Responsive** - Handles terminal resizing gracefully
- 🔒 **Clean Shutdown** - Properly terminates shell process on exit

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/ai-terminal-tui.git
cd ai-terminal-tui

# Build the binary
go build -o ai-terminal-tui .

# Move to a location in your PATH
sudo mv ai-terminal-tui /usr/local/bin/
```

### Requirements

- Go 1.22 or later
- A LiteLLM-compatible API endpoint (or OpenAI-compatible API)
- bash or zsh shell

## Configuration

Create a configuration file at `~/.config/ai-terminal-tui/config.json`:

```json
{
  "litellm_url": "http://localhost:4000",
  "litellm_token": "your-api-token-here",
  "model": "gpt-4",
  "shell": "/bin/bash"
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `litellm_url` | Base URL for the LiteLLM API | `http://localhost:4000` |
| `litellm_token` | Bearer token for API authentication | `""` |
| `model` | Model name to use for completions | `gpt-4` |
| `shell` | Shell to spawn in the terminal | `$SHELL` or `/bin/bash` |

## Usage

### Basic Usage

```bash
ai-terminal-tui
```

### Controls

| Key | Action |
|-----|--------|
| `Ctrl+K` | Toggle AI prompt overlay |
| `Enter` | Submit AI query (when prompt is open) |
| `Esc` | Close AI prompt without submitting |
| `Ctrl+C` | Send interrupt to shell |
| `Ctrl+D` | Send EOF to shell |
| `Ctrl+L` | Clear screen |

### AI Command Generation

1. Press `Ctrl+K` to open the AI prompt
2. Type a natural language description of what you want to do
3. Press `Enter` to submit
4. The AI will generate and execute the appropriate shell command

#### Examples

- "list all files modified in the last 24 hours"
- "find all Python files in the current directory"
- "show git log with graph and one line per commit"
- "compress all PNG files in the images folder"

## Architecture

The application is built using:

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - The Elm Architecture-inspired TUI framework for Go
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** - Style definitions for terminal applications
- **[Bubbles](https://github.com/charmbracelet/bubbles)** - Common TUI components (text input)
- **[creack/pty](https://github.com/creack/pty)** - PTY (pseudo-terminal) wrapper for spawning shells

## Development

### Building

```bash
go build -o ai-terminal-tui .
```

### Running in Development

```bash
go run .
```

### Testing

```bash
go test ./...
```

## API Compatibility

The application uses the OpenAI-compatible `/v1/chat/completions` endpoint. It works with:

- OpenAI API
- LiteLLM Proxy
- Local LLM servers (like Ollama with OpenAI compatibility)
- Any other OpenAI-compatible API endpoint

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Troubleshooting

### Shell not spawning

- Check that your shell path in the configuration is correct
- Ensure the shell binary exists and is executable

### API connection errors

- Verify the `litellm_url` is correct and accessible
- Check that the `litellm_token` is valid if authentication is required
- Ensure the LiteLLM server is running and accessible

### Terminal display issues

- The application requires a terminal with Unicode support
- For best results, use a modern terminal emulator (iTerm2, Windows Terminal, GNOME Terminal, etc.)

## Acknowledgments

Built with love using the [Charm](https://charm.sh) ecosystem of Go libraries.
