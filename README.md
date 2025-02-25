# Tama Code

Tama Code is a command-line interface for interacting with large language models (LLMs) like OpenAI's GPT and Ollama models.

## Features

- Simple terminal UI
- Support for multiple LLM providers (OpenAI, Ollama)
- Configuration stored in user's home directory
- Interactive chat interface

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
│   └── llm/             # LLM clients
│       ├── client.go    # Generic LLM client
│       ├── models.go    # Data structures
│       ├── openai.go    # OpenAI-specific code
│       └── ollama.go    # Ollama-specific code
└── go.mod               # Module definition
```

## Building and Running

### Prerequisites

- Go 1.21 or later
- OpenAI API key (optional)
- Ollama running locally (optional, default)

### Building

```bash
go build -o tama ./cmd/tama
```

### Running

```bash
./tama
```

On first run, the application will create a default configuration file at `~/.config/tama/config.yaml`.

## Configuration

The configuration file is located at `~/.config/tama/config.yaml` and contains:

- Provider configurations (API keys, base URLs)
- Default provider and model
- Model parameters (temperature, max tokens)

Example:

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

## License

MIT
