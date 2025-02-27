# Tama Code

<div align="center">
  <img src="https://via.placeholder.com/200x100?text=Tama+Code" alt="Tama Code Logo" width="200"/>
  
  <p>A powerful terminal interface for interacting with large language models</p>

  ![License](https://img.shields.io/badge/license-MIT-blue.svg)
  ![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)
</div>

## Overview

Tama Code provides a clean, distraction-free terminal interface for interacting with large language models (LLMs) like OpenAI's GPT and local Ollama models. Designed for developers who prefer working in the terminal, Tama Code offers a seamless experience to leverage AI assistance while staying in your workflow.

## Key Features

- 🖥️ **Terminal-native experience** - Designed for keyboard-driven productivity
- 🔄 **Multiple LLM providers** - Support for OpenAI API and local Ollama
- 🌈 **Colorful, clean UI** - Intuitive interface with customizable text styles
- 🔐 **Local configuration** - Settings stored securely in your home directory
- 💬 **Conversation context** - Maintains context across messages for better responses
- 📁 **Context-aware coding** - Powerful contextual operations for files, folders, codebase, git, and web
- 🛠️ **Extensible design** - Easily add support for additional LLM providers

## Project Structure

```
tama/
├── cmd/                 # Command-line applications
│   └── tama/            # Main Tama CLI application
│       └── main.go      # Entry point
├── internal/            # Application packages (private)
│   ├── config/          # Configuration handling
│   │   └── config.go
│   ├── ui/              # User interface
│   │   └── ui.go
│   ├── code/            # Code assistant
│   │   ├── handler.go   # Main code assistant handler
│   │   ├── context.go   # Context-aware operations
│   │   ├── commands.go  # Slash commands
│   │   ├── analyze.go   # Code analysis
│   │   └── types.go     # Types and interfaces
│   ├── chat/            # Chat interface
│   │   └── handler.go   # Chat handler
│   └── llm/             # LLM clients
│       ├── client.go    # Generic LLM client
│       ├── models.go    # Data structures
│       ├── openai.go    # OpenAI-specific code
│       └── ollama.go    # Ollama-specific code
└── go.mod               # Module definition
```

## Installation

### Prerequisites

- Go 1.21 or later
- OpenAI API key (optional)
- Ollama running locally (optional, but is the default)

### From Source

Clone the repository and build:

```bash
git clone https://github.com/warm3snow/tama.git
cd tama
go build -o tama ./cmd/tama
```

Then move the binary to your PATH:

```bash
sudo mv tama /usr/local/bin/
```

## Usage

Start the application:

```bash
tama
```

### Basic Commands

- Select text style on first run
- Type your queries or paste code at the prompt
- Type `exit` or `quit` to end the session

### Context-Aware Code Assistant

The code assistant provides powerful contextual operations that can be used to give the AI more context about your code:

#### File Context

View or analyze specific files:

```
@file main.go
```

This will show the contents of main.go and provide it as context to the AI.

#### Folder Context

Explore directory structures:

```
@folder ./internal depth=2
```

This will show the structure of the internal directory up to 2 levels deep.

#### Codebase Context

Get a high-level overview of the entire codebase:

```
@codebase depth=3
```

This analyzes the entire codebase structure and important files.

#### Git Context

Run and analyze git commands:

```
@git status
@git log --oneline -n 5
```

This executes the git command and provides the output as context to the AI.

#### Web Context

Search the web for relevant information:

```
@web "golang context switching"
```

This searches the web for the given query and provides the results as context.

### Slash Commands

The code assistant also supports slash commands:

- `/help` - Display available commands
- `/cd` - Display or change current directory
- `/file` - Help on file operations
- `/folder` - Help on folder operations
- `/codebase` - Help on codebase operations
- `/git` - Help on git operations
- `/web` - Help on web search
- `/! <command>` - Execute a shell command

## Configuration

On first run, Tama Code creates a default configuration file at `~/.config/tama/config.yaml`. Edit this file to change settings such as:

- API keys
- Default provider and model
- Temperature and token limits

Example configuration:

```json
{
  "providers": {
    "openai": {
      "api_key": "sk-xxx",
      "base_url": "https://api.openai.com/v1"
    },
    "ollama": {
      "base_url": "http://localhost:11434",
      "api_key": "ollama"
    }
  },
  "defaults": {
    "provider": "ollama",
    "model": "llama3.2:latest",
    "temperature": 0.7,
    "max_tokens": 2048
  }
}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Ollama](https://github.com/ollama/ollama) - For local LLM support
- [OpenAI](https://openai.com/) - For their powerful AI models
- [Fatih's Color Package](https://github.com/fatih/color) - For terminal coloring
