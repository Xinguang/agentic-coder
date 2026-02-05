# Agentic Coder - Technical Design Document

## Project Overview

**Agentic Coder** is an AI-powered coding assistant CLI tool built in Go, designed to **solve the vendor lock-in problem in AI tooling**. It provides a unified interface to interact with multiple AI providers, supporting 8+ providers through both API and local CLI approaches.

### Core Features

- **Zero Vendor Lock-in**: Switch AI providers with a single parameter while maintaining consistent workflow
- **Unified Tool Ecosystem**: 22 built-in tools with consistent interface across all providers
- **Multiple Deployment Options**: API, local CLI tools, completely offline (Ollama)
- **Single Binary**: Go compiled to ~10MB standalone executable with zero dependencies
- **Streaming Responses**: Real-time streaming output for all providers
- **Persistent Sessions**: Automatic conversation history saving with resume and compression support
- **Cost Tracking**: Real-time token usage and cost monitoring with pricing data for 20+ models
- **Code Review**: Integrated code quality checking and security analysis
- **Extended Thinking**: Support for Claude's extended thinking tokens
- **Permission Management**: Fine-grained tool permission control system
- **Skills System**: User-defined slash command support
- **MCP Integration**: Model Context Protocol support for external tool ecosystems
- **Lifecycle Hooks**: Custom logic injection at key moments

### Technology Stack

| Component | Version/Description |
|-----------|-------------------|
| **Language** | Go 1.24.2+ |
| **CLI Framework** | Cobra v1.10.2 |
| **TUI Framework** | Bubble Tea v1.3.10 + Bubbles v0.21.1 |
| **Markdown Rendering** | Glamour v0.10.0 |
| **Build Tool** | Make |

### Code Statistics

| Metric | Value |
|--------|-------|
| Total Lines of Code | ~16,000+ |
| Go Source Files | 86+ |
| Test Files | 13 |
| Main Packages | 23 |
| Built-in Tools | 22 |
| Supported AI Providers | 8+ |
| Binary Size | ~10MB |
| Largest Files | main.go (~1,330 lines), loop.go (~13KB) |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI (main.go)                           │
├─────────────────────────────────────────────────────────────────┤
│                       Workflow Layer                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Manager ─► Executors (concurrent) ─► Reviewers ─► Fixers   ││
│  │                         │                                   ││
│  │                     Evaluator                               ││
│  └─────────────────────────────────────────────────────────────┘│
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

### 5. Multi-Agent Workflow System (`pkg/workflow/`)

The workflow system orchestrates multiple AI agents for complex tasks that benefit from planning, concurrent execution, and quality review.

#### Agent Roles

| Role | Responsibility |
|------|----------------|
| **Manager** | Analyzes requirements, creates task plans with dependencies |
| **Executor** | Executes individual tasks using the engine |
| **Reviewer** | Reviews execution quality, identifies issues |
| **Fixer** | Auto-fixes minor issues found during review |
| **Evaluator** | Evaluates overall result quality |

#### Workflow Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Workflow                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Manager Agent                             ││
│  │  - Analyze requirement                                       ││
│  │  - Create task plan with DAG dependencies                   ││
│  └─────────────────────────────────────────────────────────────┘│
│                            │                                     │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Concurrent Executor Pool                        ││
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       ││
│  │  │Executor 1│ │Executor 2│ │Executor 3│ │Executor N│       ││
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘       ││
│  │  (semaphore-controlled, default max: 5)                     ││
│  └─────────────────────────────────────────────────────────────┘│
│                            │                                     │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Review & Fix Loop                               ││
│  │  ┌──────────┐     ┌──────────┐                              ││
│  │  │ Reviewer │ ──► │  Fixer   │  (if minor issues)           ││
│  │  └──────────┘     └──────────┘                              ││
│  └─────────────────────────────────────────────────────────────┘│
│                            │                                     │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Evaluator Agent                            ││
│  │  - Compare result against requirement                        ││
│  │  - Generate quality score and report                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

#### Key Components

**Task Plan & DAG Dependencies**
```go
type TaskPlan struct {
    ID          string
    Requirement string
    Analysis    string
    Tasks       []*Task
}

type Task struct {
    ID           string
    Title        string
    Description  string
    Dependencies []string  // DAG edges
    Priority     int
    Status       TaskStatus
    Execution    *Execution
    Reviews      []*Review
}
```

**Concurrency Control**
- Semaphore-based executor pool (configurable max workers)
- Resource locking prevents file conflicts between concurrent tasks
- DAG-based dependency resolution ensures correct execution order

**Review & Retry Loop**
```
Execute Task
     │
     ▼
  Review
     │
     ├─► Pass ──────────► Complete
     │
     ├─► Minor Issues ──► Auto-Fix ──► Re-review
     │
     └─► Major Issues ──► Retry (up to max retries) ──► Fail
```

#### Configuration

```go
type WorkflowConfig struct {
    MaxExecutors  int    // default: 5
    MaxReviewers  int    // default: 2
    MaxFixers     int    // default: 2
    MaxRetries    int    // default: 3
    EnableAutoFix bool   // default: true
    Models        RoleModels
}

type RoleModels struct {
    Default   string  // used if role-specific not set
    Manager   string
    Executor  string
    Reviewer  string
    Fixer     string
    Evaluator string
}
```

### 6. Authentication (`pkg/auth/`)

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
├── credentials.json         # API keys (0600 permissions)
├── config.yaml             # User configuration
├── permissions.yaml        # Tool permission rules
├── hooks.yaml              # Lifecycle hooks
├── mcp.json                # MCP server configuration
├── sessions/               # Conversation history
│   ├── <uuid-1>.json
│   └── <uuid-2>.json
├── work/                   # Work contexts
│   ├── <id-1>.json
│   └── <id-2>.json
└── skills/                 # Custom skills
    ├── commit.md
    └── review-pr.md
```

### Instruction Files

The project supports instruction files for customizing system prompts:

1. **AGENT.md** (Recommended)
   - Project-level: Place in project root
   - Global: `~/.claude/AGENT.md`

2. **CLAUDE.md** (Backward compatible)
   - Project-level: Place in project root
   - Global: `~/.claude/CLAUDE.md`

Priority: AGENT.md > CLAUDE.md

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

## Design Highlights

### 1. Architecture Advantages

- **Clear layered architecture**: CLI → Engine → Provider → Tool → Support Services
- **Interface-driven design**: Heavy use of interfaces for polymorphism and decoupling
- **Single responsibility principle**: Each package and module has clear responsibility
- **Highly extensible**: Provider and Tool systems support dynamic extension

### 2. Provider Abstraction Value

**Core Value**: Solving vendor lock-in in AI tooling

- **Unified interface**: All providers implement the same `AIProvider` interface
- **Zero-friction switching**: Switch providers via `-m` parameter
- **Diversity support**: API, CLI, local deployment three approaches
- **Easy to extend**: Adding new provider only requires implementing interface and registering

### 3. Tool Ecosystem

- **22 built-in tools**: Cover file operations, shell, web, development scenarios
- **Dynamic registration**: Tools managed through registry pattern
- **MCP integration**: Support external tool ecosystem extension
- **Permission control**: Fine-grained tool execution permission management

### 4. Streaming-First Design

- **All providers support streaming**: Consistent real-time response experience
- **Low latency**: First byte time < 1s
- **Cancelable**: Support Context cancellation mechanism
- **Callback-driven**: Engine decoupled from UI through callbacks

### 5. Engineering Quality

- **Robust error handling**: Layered error handling, clear error propagation
- **Concurrency safety**: Use mutex to protect shared state
- **Test coverage**: Target 70%+ coverage
- **Complete documentation**: Detailed code comments and documentation

## Performance Metrics

| Operation | Time |
|-----------|------|
| Startup time | < 100ms |
| First byte time (TTFB) | < 1s |
| Session load | < 50ms |
| Tool execution | < 500ms (average) |
| Binary size | ~10MB |

## Technical Debt & Improvements

### Current Technical Debt

1. **Provider implementation duplication**: Similar code across providers, can extract common logic
2. **Test coverage**: Currently only 13 test files, need more unit tests
3. **Configuration management**: Configuration file formats can be more unified
4. **Error handling**: Some error handling can be more detailed

### Short-term Improvements

1. **Increase test coverage**: Focus on Engine, Provider, Tool
2. **Extract Provider common logic**: Create `BaseProvider` to reduce code duplication
3. **Enhanced documentation**: Add more usage examples and best practices
4. **Performance optimization**: Implement tool parallelization

### Long-term Roadmap

1. ~~**Multi-agent system**: Agent orchestration and collaboration~~ ✅ Implemented in v0.2.0
2. **IDE integration**: VS Code, JetBrains plugins
3. **Web UI**: Web-based user interface
4. **Cloud service**: Hosted version and team collaboration
5. **Advanced workflow features**: Dynamic replanning, inter-task communication

## Maintainers

- **Project Creator**: @xinguang
- **Current Version**: v0.2.0
- **Last Updated**: 2025-02-05
- **Document Version**: v1.1.0

---

**This document is generated based on in-depth code analysis and updated continuously with the project. For questions or suggestions, please submit an Issue.**
