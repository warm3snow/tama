# Tama - Golang Copilot Agent

Tama is an autonomous AI coding assistant that acts as a peer programmer. It can perform multi-step coding tasks by analyzing your codebase, reading files, proposing edits, and running commands.

## Features

- **Codebase Analysis**: Analyzes your project structure and code to understand context
- **Autonomous Actions**: Reads files, proposes edits, and runs terminal commands
- **Error Handling**: Responds to compile and lint errors automatically
- **Continuous Improvement**: Monitors output and auto-corrects in a loop until tasks are completed
- **Natural Language Interface**: Communicate your coding tasks in plain English
- **Local LLM Support**: Uses Ollama by default with llama3.2:latest model

## Installation

```bash
go get github.com/warm3snow/tama
```

## Prerequisites

By default, Tama uses [Ollama](https://ollama.ai/) with the llama3.2:latest model. Make sure you have Ollama installed and the model pulled:

```bash
# Install Ollama from https://ollama.ai/
# Then pull the llama3.2 model
ollama pull llama3.2:latest
```

## Usage

```bash
# Start the Tama agent
tama start

# Execute a specific task
tama exec "create a REST API endpoint for user authentication"

# Get help
tama help
```

## Architecture

Tama follows a modular architecture with the following components:

![Tama Architecture](https://raw.githubusercontent.com/warm3snow/tama/main/docs/architecture.png)

1. **User Interface**: Accepts natural language prompts from the user and streams results back
2. **Copilot Agent**: Central coordinator that manages the workflow between components
3. **LLM Interface**: Communicates with language models for code generation and decision making
4. **Workspace Manager**: Handles file operations and codebase analysis
5. **Tools Registry**: Provides a set of tools for the agent to use (file operations, terminal commands, etc.)

### Workflow

1. User provides a prompt to the Copilot Agent
2. Copilot analyzes the workspace to gather context
3. Copilot sends the prompt, context, and available tools to the LLM
4. LLM decides on the next action to take (which tool to use with what arguments)
5. Copilot executes the tool and captures the result
6. Result is sent back to the LLM along with the original context for the next decision
7. This loop continues until the task is complete
8. Results are streamed back to the user

## Configuration

Tama automatically creates a configuration file at `~/.tama/tama.yaml` if it doesn't exist. You can also place a `tama.yaml` file in your project root to override the default configuration:

```yaml
llm:
  provider: ollama
  model: llama3.2:latest
  api_key: "" # Not needed for local Ollama
  base_url: "http://localhost:11434/v1"
  temperature: 0.7
  max_tokens: 4096
  options:
    timeout: "60s"

tools:
  enabled:
    - file_read
    - file_edit
    - terminal_run
    - test_run
    - file_search
    - dir_list

workspace:
  ignore_dirs:
    - .git
    - node_modules
    - vendor
  ignore_files:
    - .DS_Store
    - "*.log"
  max_file_size: 1048576 # 1MB

ui:
  color_enabled: true
  log_level: info
  verbose: false
```

### LLM Providers

Tama supports any LLM provider with an OpenAI-compatible API:

- **ollama**: Local LLM using Ollama (default)
- **openai**: OpenAI API (requires API key)
- **mock**: Mock implementation for testing

To use OpenAI instead of Ollama, update your configuration:

```yaml
llm:
  provider: openai
  model: gpt-4
  api_key: "your-api-key" # Or set TAMA_API_KEY environment variable
  base_url: "https://api.openai.com/v1"
```

## Available Tools

- **file_read**: Reads the contents of a file
- **file_edit**: Edits the contents of a file
- **terminal_run**: Runs a command in the terminal
- **test_run**: Runs tests in the project
- **file_search**: Searches for patterns in files
- **dir_list**: Lists files in a directory

## License

MIT 