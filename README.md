# Tama AI Assistant

<img src="https://via.placeholder.com/200x100?text=Tama+AI" alt="Tama AI Logo" width="200"/>

[![Go Report Card](https://goreportcard.com/badge/github.com/warm3snow/tama)](https://goreportcard.com/report/github.com/warm3snow/tama)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Overview

Tama AI Assistant provides a clean, distraction-free terminal interface for interacting with large language models (LLMs) like OpenAI's GPT and local Ollama models. Designed for developers who prefer working in the terminal, Tama offers a seamless experience to leverage AI assistance while staying in your workflow.

## Features

- 🤖 **Multiple AI Models** - Support for OpenAI GPT models and local Ollama models
- 💬 **Interactive Chat** - Natural conversation with AI in your terminal
- 🔧 **Tool Integration** - AI can use tools to help with your tasks
- 📁 **Context-aware** - Powerful contextual operations for files, folders, codebase, git, and web
- ⚡ **Fast & Efficient** - Written in Go for optimal performance
- 🎨 **Beautiful UI** - Clean, modern terminal interface
- 🔒 **Secure** - Local configuration and secure API handling

## Installation

```bash
go install github.com/warm3snow/tama@latest
```

Or build from source:

```bash
git clone https://github.com/warm3snow/tama.git
cd tama
go build
```

## Configuration

Create a configuration file at `~/.config/tama/config.json`:

```json
{
  "openai": {
    "api_key": "your-api-key",
    "model": "gpt-4-turbo-preview"
  },
  "ollama": {
    "host": "http://localhost:11434",
    "model": "llama2"
  }
}
```

## Usage

Start an interactive chat session:
```bash
tama chat
```

Get AI assistance with specific tasks:
```bash
tama chat "Explain how this code works"
```

## Context Commands

Tama supports various context commands to help AI understand your environment:

- `@file <path>` - File as context
- `@folder <path>` - Folder as context
- `@codebase` - Codebase as context
- `@git <command>` - Git information as context
- `@web <query>` - Web search as context

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
