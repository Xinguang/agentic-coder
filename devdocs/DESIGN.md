# Agentic Coder - Design Document

## Overview

Agentic Coder is an AI-powered coding assistant CLI built in Go. It provides a unified interface to interact with multiple AI providers, both through APIs and local CLI tools.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI (main.go)                           │
├─────────────────────────────────────────────────────────────────┤
│                         Engine Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Engine    │  │   Prompt    │  │   Session Manager       │ │
│  │   (loop)    │  │   Builder   │  │   (persistence)         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                       Provider Layer                            │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │
│  │ Claude │ │ OpenAI │ │ Gemini │ │DeepSeek│ │   Ollama     │  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └──────────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                        │
│  │ClaudeCLI │ │ CodexCLI │ │GeminiCLI │  (Local CLI Providers) │
│  └──────────┘ └──────────┘ └──────────┘                        │
├─────────────────────────────────────────────────────────────────┤
│                        Tool Layer                               │
│  ┌──────┐ ┌───────┐ ┌──────┐ ┌──────┐ ┌────────┐ ┌──────────┐  │
│  │ Read │ │ Write │ │ Edit │ │ Bash │ │WebFetch│ │ WebSearch│  │
│  └──────┘ └───────┘ └──────┘ └──────┘ └────────┘ └──────────┘  │
│  ┌──────┐ ┌──────┐ ┌──────────┐ ┌────────┐ ┌─────────────────┐ │
│  │ Glob │ │ Grep │ │ Notebook │ │  Task  │ │    PlanMode     │ │
│  └──────┘ └──────┘ └──────────┘ └────────┘ └─────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                      Support Layer                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │    Auth     │  │   Config    │  │       Storage           │ │
│  │  Manager    │  │   Manager   │  │                         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Provider System (`pkg/provider/`)

The provider system abstracts AI model interactions through a common interface.

#### Interface Definition

```go
type AIProvider interface {
    Name() string
    SupportedModels() []string
    SupportsFeature(feature Feature) bool
    CreateMessage(ctx context.Context, req *Request) (*Response, error)
    CreateMessageStream(ctx context.Context, req *Request) (StreamReader, error)
}

type StreamReader interface {
    Recv() (StreamingEvent, error)
    Close() error
}
```

#### Provider Types

| Type | Description | Authentication |
|------|-------------|----------------|
| API Providers | Direct API calls | API Key |
| CLI Providers | Wrapper around local CLI tools | CLI login |

#### Adding a New Provider

1. Create directory: `pkg/provider/<name>/`
2. Implement `AIProvider` interface
3. Add provider type to `pkg/provider/factory.go`
4. Update `DetectProviderFromModel()` function
5. Add case in `main.go` `createProvider()` function

### 2. Tool System (`pkg/tool/`)

Tools extend the AI's capabilities to interact with the environment.

#### Interface Definition

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Execute(ctx context.Context, input map[string]interface{}) (*Output, error)
}
```

#### Built-in Tools

| Category | Tools |
|----------|-------|
| File Operations | Read, Write, Edit, Glob, Grep |
| Shell | Bash, KillShell |
| Web | WebFetch, WebSearch |
| Planning | EnterPlanMode, ExitPlanMode |
| Advanced | Task, NotebookEdit, LSP |

#### Adding a New Tool

1. Create file in `pkg/tool/builtin/`
2. Implement `Tool` interface
3. Register in `main.go` `registerBuiltinTools()` function

### 3. Engine (`pkg/engine/`)

The engine orchestrates the conversation loop between user input, AI responses, and tool execution.

#### Main Loop Flow

```
User Input
    │
    ▼
┌─────────────────┐
│  Build Request  │
│  (add tools,    │
│   system prompt)│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Call Provider  │◄────────────┐
│  (streaming)    │             │
└────────┬────────┘             │
         │                      │
         ▼                      │
┌─────────────────┐             │
│ Process Events  │             │
│ - Text: output  │             │
│ - Tool: execute │─────────────┘
│ - Stop: done    │   (tool results)
└────────┬────────┘
         │
         ▼
    Response Done
```

#### Callback System

```go
type CallbackOptions struct {
    OnText       func(text string)
    OnThinking   func(text string)
    OnToolUse    func(name string, input map[string]interface{})
    OnToolResult func(name string, result *tool.Output)
    OnError      func(err error)
}
```

### 4. Session Management (`pkg/session/`)

Sessions maintain conversation history and context.

#### Session Structure

```go
type Session struct {
    ID          string
    ProjectPath string
    CWD         string
    Model       string
    Messages    []Message
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

#### Persistence

Sessions are automatically saved to `~/.config/agentic-coder/sessions/`.

### 5. Authentication (`pkg/auth/`)

Manages credentials for different providers.

#### Supported Auth Types

- **API Key**: Simple key-based authentication
- **OAuth**: Token-based authentication (for future use)

#### Credential Storage

Credentials are stored in `~/.config/agentic-coder/credentials.json` with 0600 permissions.

## Data Flow

### Streaming Response Processing

```
Provider Stream
      │
      ▼
┌─────────────────────┐
│  MessageStartEvent  │ ─► Initialize response
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ContentBlockDelta    │ ─► Accumulate text/tool input
│  (multiple)         │    Callback: OnText/OnThinking
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ContentBlockStop     │ ─► Finalize block
│                     │    Execute tool if tool_use
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  MessageStopEvent   │ ─► Complete response
└─────────────────────┘
```

### CLI Provider Streaming

Local CLI providers (claudecli, codexcli, geminicli) parse stdout as JSONL:

```
CLI Process
    │
    ▼ (stdout)
┌─────────────────┐
│ Parse JSON line │
│ by line         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Calculate delta │  (fullText - lastText)
│ from cumulative │
│ text            │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Emit as         │
│ StreamingEvent  │
└─────────────────┘
```

## Configuration

### Directory Structure

```
~/.config/agentic-coder/
├── credentials.json    # API keys and auth tokens
├── config.yaml         # User configuration (future)
└── sessions/           # Session history
    ├── <session-id>.json
    └── ...
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Claude API authentication |
| `OPENAI_API_KEY` | OpenAI API authentication |
| `GOOGLE_API_KEY` | Gemini API authentication |
| `DEEPSEEK_API_KEY` | DeepSeek API authentication |
| `OLLAMA_HOST` | Ollama server URL |

## Error Handling

### Provider Errors

```go
type APIError struct {
    StatusCode int
    Message    string
    Type       string
}
```

### Tool Errors

Tool errors are returned in `Output.IsError` and displayed to user without stopping the conversation.

### Context Cancellation

- `Ctrl+C` cancels the current operation via context
- Double `Ctrl+C` exits the application

## Testing

### Unit Tests

```bash
go test ./...
```

### Integration Tests

Integration tests require actual API keys and are skipped by default:

```bash
INTEGRATION_TEST=1 go test ./... -v
```

## Performance Considerations

1. **Streaming**: All providers support streaming to reduce perceived latency
2. **Tool Parallelization**: Independent tool calls can run in parallel (future)
3. **Session Caching**: Sessions are loaded once and kept in memory
4. **Connection Reuse**: HTTP clients reuse connections via `http.Client`

## Security

1. **Credential Storage**: File permissions 0600 for credentials.json
2. **API Key Handling**: Keys never logged or displayed
3. **Tool Sandboxing**: Bash tool can be configured with restrictions (future)
4. **Input Validation**: All tool inputs are validated before execution

## Future Enhancements

- [ ] MCP (Model Context Protocol) server support
- [ ] Plugin system for custom tools
- [ ] Multi-agent orchestration
- [ ] Code review and refactoring agents
- [ ] IDE integrations (VS Code, JetBrains)
