# Agentic Coder

An AI-powered coding assistant CLI that supports multiple AI providers and local CLI tools.

## Why Agentic Coder?

### No Vendor Lock-in
Switch between AI providers with a single flag. Use Claude today, GPT-4 tomorrow, or run locally with Ollama - your workflow stays the same.

### Use Your Existing CLI Tools
Already have Claude Code, Codex, or Gemini CLI installed? Agentic Coder can wrap them directly - no API keys needed, leverage your existing subscriptions and login sessions.

### Unified Interface
One tool to rule them all. Same commands, same tools, same experience across 8+ AI providers. No need to learn different interfaces for different models.

### Privacy-First Options
Run completely offline with Ollama, or use local CLI tools that handle authentication independently. Your code never has to leave your machine.

### Built for Developers
- **Single Binary**: Written in Go, compiles to a single executable with zero dependencies
- **Cross-Platform**: Works on macOS, Linux, and Windows
- **Fast Startup**: No runtime overhead, starts instantly
- **Extensible**: Easy to add new providers or tools

### Rich Tool Ecosystem
19 built-in tools for real coding tasks: file operations, shell commands, web search, Jupyter notebooks, and more. The AI can actually help you code, not just chat.

### Cost Optimization
Mix and match providers based on task complexity. Use cheaper models for simple tasks, premium models for complex ones. Local CLI providers use your existing subscriptions.

## Features

- **Multi-Provider Support**: Connect to various AI providers through API or local CLI
- **Streaming Responses**: Real-time streaming output for all providers
- **Tool Integration**: Built-in tools for file operations, web search, shell commands
- **Session Management**: Persistent conversation history
- **Authentication Management**: Secure credential storage for API keys

## Supported Providers

### API Providers (requires API key)

| Provider | Models | Environment Variable |
|----------|--------|---------------------|
| Claude (Anthropic) | `sonnet`, `opus`, `haiku`, `claude-*` | `ANTHROPIC_API_KEY` |
| OpenAI | `gpt-4o`, `gpt-4`, `o1-*`, `o3-*`, `o4-*` | `OPENAI_API_KEY` |
| Gemini (Google) | `gemini-2.5-pro`, `gemini-2.5-flash` | `GOOGLE_API_KEY` |
| DeepSeek | `deepseek-*`, `coder`, `reasoner`, `r1` | `DEEPSEEK_API_KEY` |
| Ollama | `llama*`, `qwen*`, `mistral*`, `phi*` | Local (no key needed) |

### Local CLI Providers (uses installed CLI tools)

| Provider | Model Flag | Required CLI |
|----------|------------|--------------|
| Claude Code | `claudecli`, `claude-cli` | [Claude Code](https://docs.anthropic.com/en/docs/claude-code) |
| Codex | `codexcli`, `codex-cli`, `codex` | [Codex CLI](https://github.com/openai/codex) |
| Gemini CLI | `geminicli`, `gemini-cli` | [Gemini CLI](https://github.com/google-gemini/gemini-cli) |

## Installation

### From Source

```bash
git clone https://github.com/xinguang/agentic-coder.git
cd agentic-coder
make build
```

The binary will be available at `./bin/agentic-coder`.

### Prerequisites

- Go 1.22 or later
- For local CLI providers, install the respective CLI tools:
  - Claude Code: `npm install -g @anthropic-ai/claude-code`
  - Codex: `npm install -g @openai/codex`
  - Gemini CLI: `npm install -g @anthropic-ai/gemini-cli`

## Usage

### Quick Start

```bash
# Using Claude API (default)
export ANTHROPIC_API_KEY="your-api-key"
./bin/agentic-coder

# Using OpenAI
export OPENAI_API_KEY="your-api-key"
./bin/agentic-coder -m gpt-4o

# Using local Claude Code CLI
./bin/agentic-coder -m claudecli

# Using local Codex CLI
./bin/agentic-coder -m codex

# Using local Gemini CLI
./bin/agentic-coder -m geminicli

# Using Ollama (local)
./bin/agentic-coder -m llama3.2
```

### Authentication

Save API keys for persistent use:

```bash
# Login to a provider
./bin/agentic-coder auth login claude
./bin/agentic-coder auth login openai
./bin/agentic-coder auth login gemini

# Check authentication status
./bin/agentic-coder auth status

# Logout
./bin/agentic-coder auth logout claude
```

### Work Context Management

When switching between AI providers (e.g., when one runs out of tokens), you need to maintain task continuity. Work contexts help you track progress and generate handoff summaries.

```bash
# Create a new work context
./bin/agentic-coder work new "Implement user authentication" --goal "Add OAuth2 login"

# Update progress
./bin/agentic-coder work update abc123 --done "Created user model"
./bin/agentic-coder work update abc123 --pending "Add login endpoint"
./bin/agentic-coder work update abc123 --file "pkg/auth/oauth.go"
./bin/agentic-coder work update abc123 --note "Using JWT for tokens"

# List all work contexts
./bin/agentic-coder work list

# Show work context details
./bin/agentic-coder work show abc123

# Generate handoff summary (for switching providers)
./bin/agentic-coder work handoff abc123
./bin/agentic-coder work handoff abc123 --lang cn  # Chinese version
./bin/agentic-coder work handoff abc123 -o handoff.md  # Save to file

# Delete a work context
./bin/agentic-coder work delete abc123
```

The handoff summary includes:
- Goal and background
- Completed tasks
- Remaining tasks
- Key files involved
- Important notes
- Token usage per provider

### Command Line Options

```
Usage:
  agentic-coder [flags]
  agentic-coder [command]

Available Commands:
  auth        Manage authentication
  config      Manage configuration
  help        Help about any command
  version     Print version information
  work        Manage work context for task continuity

Flags:
  -h, --help           help for agentic-coder
  -k, --api-key string API key (overrides saved credentials)
  -m, --model string   Model to use (default "sonnet")
  -v, --verbose        Enable verbose output
```

### Interactive Commands

Once in the chat interface:

| Command | Description |
|---------|-------------|
| `/help`, `/h` | Show available commands |
| `/clear`, `/cls` | Clear the screen |
| `/model [name]` | Show or change the model |
| `/session` | Show current session info |
| `/sessions` | List recent sessions |
| `/resume [id]` | Resume a previous session |
| `/new` | Start a new session |
| `/save` | Save current session |
| `/work` | Manage work context |
| `/work new <title>` | Create new work context |
| `/work list` | List work contexts |
| `/work show <id>` | Show work context |
| `/work done <text>` | Mark item as done |
| `/work todo <text>` | Add pending item |
| `/work handoff` | Generate handoff summary |
| `/cost` | Show token usage |
| `/compact` | Compact conversation history |
| `/exit`, `/quit`, `/q` | Exit the program |

### Keyboard Shortcuts

- `Ctrl+C` - Interrupt current operation
- `Ctrl+C` (twice) - Exit the program
- `Ctrl+D` - Exit the program

## Project Structure

```
agentic-coder/
├── cmd/
│   └── agentic-coder/    # Main application
├── pkg/
│   ├── auth/             # Authentication management
│   ├── engine/           # Core AI engine
│   ├── provider/         # AI provider implementations
│   │   ├── claude/       # Claude API provider
│   │   ├── claudecli/    # Local Claude Code CLI provider
│   │   ├── codexcli/     # Local Codex CLI provider
│   │   ├── deepseek/     # DeepSeek API provider
│   │   ├── gemini/       # Gemini API provider
│   │   ├── geminicli/    # Local Gemini CLI provider
│   │   ├── ollama/       # Ollama provider
│   │   └── openai/       # OpenAI API provider
│   ├── session/          # Session management
│   ├── tool/             # Tool implementations
│   │   └── builtin/      # Built-in tools
│   └── ...
├── Makefile
└── README.md
```

## Built-in Tools

The assistant has access to the following tools:

- **File Operations**: Read, Write, Edit, Glob, Grep
- **Shell**: Bash command execution
- **Web**: WebSearch, WebFetch
- **Notebook**: Jupyter notebook editing
- **Planning**: EnterPlanMode, ExitPlanMode

## Configuration

Configuration files are stored in `~/.config/agentic-coder/`:

- `credentials.json` - Saved API keys and authentication data

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic/Claude API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GOOGLE_API_KEY` | Google/Gemini API key |
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `OLLAMA_HOST` | Ollama server URL (default: `http://localhost:11434`) |

## Development

```bash
# Build
make build

# Run tests
make test

# Run with verbose output
./bin/agentic-coder -v
```

## License

MIT License
