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
- 🔄 **Multiple LLM providers** - Support for OpenAI API and local Ollama with OpenAI-compatible endpoints
- 🌈 **Colorful, clean UI** - Intuitive interface with customizable text styles
- 🔐 **Local configuration** - Settings stored securely in your home directory
- 💬 **Conversation context** - Maintains context across messages for better responses
- 📁 **Context-aware coding** - Powerful contextual operations for files, folders, codebase, git, and web
- 🛠️ **Extensible architecture** - Modular design with clear separation of concerns

## Architecture

The project follows a clean, modular architecture:

```
tama/
├── cmd/                    # Command-line interface
│   └── root.go            # Root command and initialization
├── internal/              # Internal packages
│   ├── config/            # Configuration management
│   │   └── config.go      # Config types and loading
│   ├── copilot/          # Core orchestration
│   │   └── core.go       # Main copilot logic
│   ├── llm/              # LLM integration
│   │   ├── api_types.go  # API data structures
│   │   ├── client.go     # Generic LLM client
│   │   ├── context.go    # LLM context management
│   │   └── providers.go  # Provider implementations
│   ├── machine/          # Machine context
│   │   └── context.go    # System information
│   ├── tools/            # Tool registry
│   │   └── registry.go   # Tool management
│   └── workspace/        # Workspace management
│       └── manager.go    # File operations
└── go.mod                # Module definition
```

### Core Components

- **Copilot**: Central orchestrator that manages interactions between components
- **LLM Client**: Handles communication with language models using OpenAI-compatible API
- **Machine Context**: Provides system information and environment details
- **Tool Registry**: Manages available tools and their execution
- **Workspace Manager**: Handles file system operations and workspace context

## Installation

### Prerequisites

- Go 1.21 or later
- OpenAI API key (optional)
- Ollama running locally (optional, but recommended)

### From Source

```bash
git clone https://github.com/warm3snow/tama.git
cd tama
go build
```

## Usage

Start the application:

```bash
tama code
```

### Available Commands

- `/help` - Display available commands
- `!` - Execute a shell command, e.g. `/!ls -la`
- `@` - Add a context to the LLM, e.g. `@main.go`
- `/reset` - Reset conversation history

### Context Shortcuts

Context shortcuts can be used anywhere in your message:

- `@file_name` - File as context
- `@folder_name` - Folder as context
- `@codebase` - Codebase as context
- `@web` - Enable web browsing

## Configuration

Configuration is stored in `~/.config/tama/config.json`:

```json
{
  "providers": {
    "openai": {
      "type": "openai",
      "api_key": "sk-xxx",
      "base_url": "https://api.openai.com/v1"
    },
    "ollama": {
      "type": "ollama",
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
- [Cobra](https://github.com/spf13/cobra) - For CLI framework
- [Color](https://github.com/fatih/color) - For terminal coloring
