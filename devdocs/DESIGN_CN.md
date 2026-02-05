# Agentic Coder - 技术设计文档

## 项目概述

**Agentic Coder** 是一个用 Go 语言构建的 AI 编程助手 CLI 工具,旨在解决 AI 工具领域的厂商锁定问题。它提供了一个统一的接口来与多个 AI 提供商交互,支持 8+ 种 AI provider,包括 API 调用和本地 CLI 工具两种方式。

### 核心特性

- **零厂商锁定**: 通过单一参数即可切换 AI provider,工作流程保持一致
- **统一工具生态**: 22 个内置工具,跨所有 provider 保持一致的接口
- **多种部署方式**: 支持 API、本地 CLI 工具、完全离线(Ollama)
- **单一二进制**: Go 编写,编译为约 10MB 的独立可执行文件,零依赖
- **流式响应**: 所有 provider 支持实时流式输出
- **持久化会话**: 自动保存对话历史,支持恢复和切换
- **成本追踪**: 实时 token 使用量和费用监控,支持 20+ 模型的定价数据
- **代码审查**: 集成代码质量检查和安全分析,支持增量审查
- **扩展思考**: 支持 Claude 的 extended thinking tokens
- **权限管理**: 细粒度的工具权限控制系统
- **技能系统**: 用户自定义的 Slash 命令支持
- **MCP 集成**: Model Context Protocol 支持外部工具生态
- **生命周期钩子**: 在关键时刻注入自定义逻辑

### 技术栈

| 技术组件 | 版本/描述 |
|---------|----------|
| **语言** | Go 1.24.2+ |
| **CLI 框架** | Cobra v1.10.2 |
| **TUI 框架** | Bubble Tea v1.3.10 + Bubbles v0.21.1 |
| **Markdown 渲染** | Glamour v0.10.0 |
| **行编辑** | Liner v1.2.2 |
| **模式匹配** | doublestar v4.6.1 |
| **UUID 生成** | google/uuid v1.6.0 |
| **构建工具** | Make |

### 代码统计

| 指标 | 数值 |
|------|------|
| 总代码行数 | ~16,000+ |
| Go 源文件 | 86+ |
| 测试文件 | 13 |
| 主要包数 | 23 |
| 内置工具 | 22 |
| 支持的 AI Provider | 8+ |
| 二进制大小 | ~10MB |
| 主要文件最大行数 | main.go (~1,330行), loop.go (~13KB) |

## 架构设计

### 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLI Layer (cmd/agentic-coder)                │
│  Entry: main.go (~1,330 lines)                                  │
│  ├─ Commands: auth, work, config, version, workflow             │
│  ├─ Interactive chat loop with /commands                        │
│  └─ TUI/Classic mode selection                                  │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                Workflow Layer (pkg/workflow)                    │
│  Multi-Agent 工作流编排系统                                      │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Manager ─► Executors (并发) ─► Reviewers ─► Fixers         ││
│  │                    │                                        ││
│  │                Evaluator                                    ││
│  └─────────────────────────────────────────────────────────────┘│
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                  Engine Layer (pkg/engine)                      │
│  Core: loop.go (~13KB)                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Engine    │  │   Prompt    │  │   Callbacks             │ │
│  │   (agentic  │  │   Builder   │  │   (OnText, OnTool,      │ │
│  │    loop)    │  │             │  │    OnThinking, etc.)    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│               Provider Layer (pkg/provider)                     │
│  Core: types.go (351 lines) - 统一接口和数据结构                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ API Providers (require API keys) - 4,865 lines total       ││
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐ ││
│  │  │ Claude │ │ OpenAI │ │ Gemini │ │DeepSeek│ │  Ollama  │ ││
│  │  │ 542L   │ │ 607L   │ │ 559L   │ │ 559L   │ │  601L    │ ││
│  │  └────────┘ └────────┘ └────────┘ └────────┘ └──────────┘ ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ CLI Providers (use local CLI tools, no API keys)            ││
│  │  ┌──────────────┐ ┌──────────┐ ┌──────────────┐           ││
│  │  │  ClaudeCLI   │ │ CodexCLI │ │  GeminiCLI   │           ││
│  │  │ (Claude Code)│ │          │ │              │           ││
│  │  └──────────────┘ └──────────┘ └──────────────┘           ││
│  └─────────────────────────────────────────────────────────────┘│
│  Factory: factory.go (236 lines) - Provider 创建和模型检测     │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                Tool Layer (pkg/tool/builtin)                    │
│  Registry: 22 built-in tools (246 lines core)                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 文件操作: Read, Write, Edit, Glob, Grep                  │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Shell & 执行: Bash, KillShell                            │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Web 服务: WebSearch, WebFetch                            │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 开发工具: LSP, NotebookEdit, Task, TodoWrite,           │  │
│  │          AskUserQuestion, EnterPlanMode, ExitPlanMode,   │  │
│  │          Skill, TaskOutput                                │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ MCP 集成: MCPTools (动态工具注册)                        │  │
│  └──────────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                   Support Services Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐   │
│  │   Session    │  │     Auth     │  │       Cost          │   │
│  │  Management  │  │   Manager    │  │     Tracking        │   │
│  │   (~544L)    │  │   (~80L)     │  │   (20+ models)      │   │
│  └──────────────┘  └──────────────┘  └─────────────────────┘   │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐   │
│  │    Review    │  │   WorkCtx    │  │       TUI/UI        │   │
│  │   Pipeline   │  │  Management  │  │      Rendering      │   │
│  │   (~342L)    │  │   (~442L)    │  │     (~1,850L)       │   │
│  └──────────────┘  └──────────────┘  └─────────────────────┘   │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐   │
│  │    Config    │  │   Storage    │  │     Permission      │   │
│  │  Management  │  │   (~404L)    │  │       System        │   │
│  │   (~100L)    │  │              │  │                     │   │
│  └──────────────┘  └──────────────┘  └─────────────────────┘   │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐   │
│  │    Skill     │  │     Hook     │  │        Task         │   │
│  │   System     │  │   Manager    │  │     Subsystem       │   │
│  │   (~468L)    │  │              │  │     (~439L)         │   │
│  └──────────────┘  └──────────────┘  └─────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 架构特点

1. **分层设计**: CLI → Workflow → Engine → Provider → Tool → Support Services
2. **多智能体协作**: Manager、Executor、Reviewer、Fixer、Evaluator 角色分工
3. **Provider 抽象**: 统一接口隔离具体实现,支持 8+ AI 提供商
4. **工具注册表**: 动态工具注册,易于扩展,支持 MCP 外部工具
5. **回调驱动**: Engine 通过回调与 UI 解耦
6. **流式优先**: 所有 provider 支持流式响应
7. **会话持久化**: 自动保存,支持恢复和压缩
8. **权限系统**: 多级权限模式,细粒度控制
9. **插件化**: 技能系统、钩子系统、MCP 集成

## 核心组件详解

### 1. Provider 系统 (`pkg/provider/`)

Provider 系统是项目的核心抽象层,实现了对多个 AI 服务商的统一接口。总代码量约 4,865 行。

#### 核心接口

```go
// AIProvider 定义了所有 AI provider 必须实现的接口
type AIProvider interface {
    Name() string
    SupportedModels() []string
    SupportsFeature(feature Feature) bool
    CreateMessage(ctx context.Context, req *Request) (*Response, error)
    CreateMessageStream(ctx context.Context, req *Request) (StreamReader, error)
}

// StreamReader 用于读取流式响应
type StreamReader interface {
    Recv() (StreamingEvent, error)
    Close() error
}

// Feature 枚举支持的特性
type Feature int
const (
    FeatureToolUse Feature = iota
    FeatureThinking
    FeatureVision
    FeatureStreaming
)

// ContentBlock 接口 - 支持多种内容类型
type ContentBlock interface {
    Type() string // text, thinking, tool_use, tool_result, image
}
```

#### 支持的 Provider

| Provider | 类型 | 代码量 | 模型示例 | 特殊特性 |
|----------|------|--------|---------|---------|
| **Claude** | API | 542行 | sonnet, opus, haiku | Extended Thinking |
| **OpenAI** | API | 607行 | gpt-4o, o1, o3 | Function calling |
| **Gemini** | API | 559行 | gemini-2.5-pro/flash | Multimodal |
| **DeepSeek** | API | 559行 | deepseek-coder, r1 | Code-focused |
| **Ollama** | Local API | 601行 | llama3.2, qwen2.5 | 完全离线 |
| **ClaudeCLI** | CLI Wrapper | - | 继承 Claude Code | 复用已有订阅 |
| **CodexCLI** | CLI Wrapper | - | OpenAI Codex | CLI 方式 |
| **GeminiCLI** | CLI Wrapper | - | Gemini | CLI 方式 |

#### 模型别名映射

系统支持友好的模型别名:

```go
var modelAliases = map[string]string{
    "sonnet":     "claude-sonnet-4-5-20250929",
    "opus":       "claude-opus-4-5-20251101",
    "haiku":      "claude-haiku-4-20250514",
    "gpt4o":      "gpt-4o",
    "o1":         "o1-2025-12-17",
    "deepseek":   "deepseek-coder",
    "gemini-pro": "gemini-2.5-pro-002",
    // ... 更多别名
}
```

#### Provider Factory

`factory.go` (236 行) 实现了智能 provider 检测和创建:

```go
// DetectProviderFromModel 从模型名推断 provider 类型
func DetectProviderFromModel(model string) ProviderType {
    switch {
    case strings.Contains(model, "claude"), model == "sonnet", model == "opus":
        return ProviderClaude
    case strings.Contains(model, "gpt"), strings.Contains(model, "o1"):
        return ProviderOpenAI
    case strings.Contains(model, "gemini"):
        return ProviderGemini
    case strings.Contains(model, "deepseek"):
        return ProviderDeepSeek
    // ... 更多匹配规则
    }
}
```

#### 添加新 Provider 步骤

1. **创建包目录**: `pkg/provider/<name>/`
2. **实现接口**: 实现 `AIProvider` 和 `StreamReader`
3. **注册到 Factory**:
   - 在 `factory.go` 添加 `ProviderType` 常量
   - 更新 `DetectProviderFromModel()` 添加模型匹配规则
   - 在 `CreateProvider()` 添加创建逻辑
4. **添加测试**: 创建 `<name>_test.go`
5. **更新文档**: 在 README 和本文档中添加说明

### 2. 工具系统 (`pkg/tool/`)

工具系统扩展了 AI 与环境的交互能力。核心接口 246 行,22 个内置工具。

#### 核心接口

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, input *Input) (*Output, error)
    Validate(input *Input) error
}

type Output struct {
    Content string
    IsError bool
}
```

#### 工具分类

**1. 文件操作工具**

| 工具 | 功能 | 主要参数 |
|------|------|---------|
| `Read` | 读取文件,支持行号、图片、PDF | file_path, offset, limit |
| `Write` | 写入/创建文件 | file_path, content |
| `Edit` | 精确编辑文件(字符串替换) | file_path, old_string, new_string |
| `Glob` | 模式匹配查找文件 | pattern, path |
| `Grep` | 正则搜索文件内容 | pattern, path, output_mode |

**2. Shell 工具**

| 工具 | 功能 | 主要参数 |
|------|------|---------|
| `Bash` | 执行 shell 命令,支持后台运行 | command, timeout, run_in_background |
| `KillShell` | 终止后台 shell | shell_id |

**3. Web 工具**

| 工具 | 功能 | 主要参数 |
|------|------|---------|
| `WebSearch` | 搜索网络信息 | query, allowed_domains |
| `WebFetch` | 获取网页内容 | url, prompt |

**4. 开发工具**

| 工具 | 功能 | 主要参数 |
|------|------|---------|
| `LSP` | 代码智能(跳转/引用/悬浮) | operation, file_path, line, character |
| `NotebookEdit` | 编辑 Jupyter notebook | notebook_path, cell_id, new_source |
| `Task` | 启动子 agent | subagent_type, prompt, model |
| `TaskOutput` | 获取子任务输出 | task_id, block, timeout |
| `TodoWrite` | 管理待办事项 | todos (任务列表) |
| `AskUserQuestion` | 交互式提问 | questions (问题列表) |

**5. 规划工具**

| 工具 | 功能 | 使用场景 |
|------|------|---------|
| `EnterPlanMode` | 进入规划模式 | 复杂任务前的规划阶段 |
| `ExitPlanMode` | 退出规划模式 | 规划完成,准备执行 |
| `Skill` | 调用技能 | 执行特定领域技能 |

**6. MCP 工具**

| 工具 | 功能 | 说明 |
|------|------|------|
| `MCPTools` | 动态 MCP 工具 | 运行时注册外部工具 |

#### 工具注册表

工具通过注册表动态管理:

```go
type Registry struct {
    tools map[string]Tool
}

func (r *Registry) Register(tool Tool) {
    r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
    tool, ok := r.tools[name]
    return tool, ok
}
```

在 `main.go` 中注册所有内置工具:

```go
func registerBuiltinTools(registry *tool.Registry) {
    registry.Register(builtin.NewReadTool())
    registry.Register(builtin.NewWriteTool())
    registry.Register(builtin.NewEditTool())
    registry.Register(builtin.NewGlobTool())
    registry.Register(builtin.NewGrepTool())
    registry.Register(builtin.NewBashTool())
    registry.Register(builtin.NewKillShellTool())
    registry.Register(builtin.NewWebSearchTool())
    registry.Register(builtin.NewWebFetchTool())
    registry.Register(builtin.NewLSPTool())
    registry.Register(builtin.NewNotebookEditTool())
    registry.Register(builtin.NewTaskTool())
    registry.Register(builtin.NewTaskOutputTool())
    registry.Register(builtin.NewTodoWriteTool())
    registry.Register(builtin.NewAskUserQuestionTool())
    registry.Register(builtin.NewEnterPlanModeTool())
    registry.Register(builtin.NewExitPlanModeTool())
    registry.Register(builtin.NewSkillTool())
    // MCP 工具动态注册
}
```

#### 添加新工具步骤

1. **创建文件**: `pkg/tool/builtin/<name>_tool.go`
2. **实现接口**: 实现 `Tool` 接口的所有方法
3. **输入验证**: 在 `Validate` 或 `Execute` 中验证必需参数
4. **错误处理**: 使用 `Output.IsError = true` 返回错误
5. **注册工具**: 在 `main.go` 的 `registerBuiltinTools()` 中注册
6. **添加测试**: 创建对应的测试用例
7. **更新文档**: 在工具列表中添加说明

### 3. 多智能体工作流系统 (`pkg/workflow/`)

工作流系统编排多个 AI 智能体协作完成复杂任务,支持自动规划、并发执行和质量审查。

#### Agent 角色

| 角色 | 职责 |
|------|------|
| **Manager** | 分析需求,创建带依赖关系的任务计划 |
| **Executor** | 使用 Engine 执行单个任务 |
| **Reviewer** | 审查执行质量,识别问题 |
| **Fixer** | 自动修复审查中发现的小问题 |
| **Evaluator** | 评估整体结果质量 |

#### 工作流架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Workflow                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Manager Agent                             ││
│  │  - 分析需求                                                  ││
│  │  - 创建带 DAG 依赖的任务计划                                 ││
│  └─────────────────────────────────────────────────────────────┘│
│                            │                                     │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              并发执行器池 (Executor Pool)                    ││
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       ││
│  │  │Executor 1│ │Executor 2│ │Executor 3│ │Executor N│       ││
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘       ││
│  │  (信号量控制并发,默认最大: 5)                                ││
│  └─────────────────────────────────────────────────────────────┘│
│                            │                                     │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              审查与修复循环                                   ││
│  │  ┌──────────┐     ┌──────────┐                              ││
│  │  │ Reviewer │ ──► │  Fixer   │  (如有小问题)                 ││
│  │  └──────────┘     └──────────┘                              ││
│  └─────────────────────────────────────────────────────────────┘│
│                            │                                     │
│                            ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Evaluator Agent                            ││
│  │  - 对比需求与最终结果                                        ││
│  │  - 生成质量评分和报告                                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

#### 核心组件

**任务计划与 DAG 依赖**
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
    Dependencies []string  // DAG 边
    Priority     int
    Status       TaskStatus
    Execution    *Execution
    Reviews      []*Review
}
```

**并发控制**
- 基于信号量的执行器池(可配置最大工作数)
- 资源锁定防止并发任务间的文件冲突
- 基于 DAG 的依赖解析确保正确执行顺序

**审查与重试循环**
```
执行任务
     │
     ▼
   审查
     │
     ├─► 通过 ──────────► 完成
     │
     ├─► 小问题 ──► 自动修复 ──► 重新审查
     │
     └─► 严重问题 ──► 重试(最多 N 次) ──► 失败
```

#### 配置选项

```go
type WorkflowConfig struct {
    MaxExecutors  int    // 默认: 5
    MaxReviewers  int    // 默认: 2
    MaxFixers     int    // 默认: 2
    MaxRetries    int    // 默认: 3
    EnableAutoFix bool   // 默认: true
    Models        RoleModels
}

type RoleModels struct {
    Default   string  // 未指定角色时使用
    Manager   string
    Executor  string
    Reviewer  string
    Fixer     string
    Evaluator string
}
```

#### CLI 使用示例

```bash
# 基本用法
agentic-coder workflow "添加 JWT 用户认证"

# 自定义并发数
agentic-coder workflow --max-executors 10 "重构代码库"

# 为不同角色指定模型
agentic-coder workflow --model opus --executor-model sonnet "构建 REST API"
```

### 4. 引擎系统 (`pkg/engine/`)

引擎是 agentic loop 的核心实现,协调用户输入、AI 响应和工具执行。主文件 loop.go 约 13KB。

#### 主循环架构

```go
func (e *Engine) Run(ctx context.Context, userInput string) error {
    // 1. 构建请求
    req := e.buildRequest(userInput)

    // 2. 流式调用 provider
    stream, err := e.provider.CreateMessageStream(ctx, req)
    if err != nil {
        return err
    }
    defer stream.Close()

    // 3. 处理流式事件
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            break
        }

        switch event.Type {
        case EventTypeContentBlockDelta:
            e.handleContentDelta(event)
        case EventTypeContentBlockStop:
            e.handleContentStop(event)
        case EventTypeMessageStop:
            return nil
        }
    }

    return nil
}
```

#### 事件处理流程

```
Provider Stream
      │
      ▼
┌──────────────────────┐
│ MessageStartEvent    │ ─► 初始化响应
│ - message_id         │
│ - model              │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ ContentBlockStart    │ ─► 开始新内容块
│ - index              │    (text/thinking/tool_use)
│ - type               │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ ContentBlockDelta    │ ─► 累积内容
│ - text_delta         │    回调: OnText/OnThinking
│ - thinking_delta     │
│ - tool_input_delta   │
└──────────┬───────────┘
           │ (循环)
           ▼
┌──────────────────────┐
│ ContentBlockStop     │ ─► 完成内容块
│ - index              │    如果是 tool_use: 执行工具
└──────────┬───────────┘    回调: OnToolUse, OnToolResult
           │
           ▼
┌──────────────────────┐
│ MessageStopEvent     │ ─► 完成响应
│ - stop_reason        │    保存到会话历史
└──────────────────────┘
```

#### 回调系统

Engine 通过回调与 UI 层解耦:

```go
type CallbackOptions struct {
    // 文本输出回调
    OnText func(text string)

    // 思考过程回调 (Claude Extended Thinking)
    OnThinking func(text string)

    // 工具使用回调
    OnToolUse func(name string, input map[string]interface{})

    // 工具结果回调
    OnToolResult func(name string, result *tool.Output)

    // 错误回调
    OnError func(err error)

    // 消息完成回调
    OnMessageComplete func()
}
```

TUI 和 Classic 模式都通过实现这些回调来显示输出。

#### Prompt Builder

`PromptBuilder` 负责构建系统提示词:

```go
type PromptBuilder struct {
    systemPrompt    string
    tools           []ToolDefinition
    contextFiles    []string
    thinkingConfig  *ThinkingConfig
}

func (pb *PromptBuilder) Build() string {
    var parts []string

    // 基础系统提示词
    parts = append(parts, pb.systemPrompt)

    // 工具说明
    if len(pb.tools) > 0 {
        parts = append(parts, pb.buildToolsPrompt())
    }

    // 上下文文件
    if len(pb.contextFiles) > 0 {
        parts = append(parts, pb.buildContextPrompt())
    }

    // Extended Thinking 配置
    if pb.thinkingConfig != nil {
        parts = append(parts, pb.buildThinkingPrompt())
    }

    return strings.Join(parts, "\n\n")
}
```

### 5. 会话管理 (`pkg/session/`)

会话系统维护对话历史和上下文状态。核心代码约 544 行。

#### 会话结构

```go
type Session struct {
    // 基本信息
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    ProjectPath string    `json:"project_path"`
    CWD         string    `json:"cwd"`

    // 模型信息
    Model       string    `json:"model"`
    Provider    string    `json:"provider"`

    // 对话历史
    Messages    []Message `json:"messages"`

    // 上下文管理
    Todos       []Todo    `json:"todos"`
    GitBranch   string    `json:"git_branch"`

    // 权限设置
    PermissionMode string `json:"permission_mode"`

    // 时间戳
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`

    // Token 统计
    TokenUsage  TokenUsage `json:"token_usage"`
}

type Message struct {
    Role        string                   `json:"role"` // user, assistant
    Content     []ContentBlock           `json:"content"`
    StopReason  string                   `json:"stop_reason,omitempty"`
    Timestamp   time.Time                `json:"timestamp"`
}

type ContentBlock struct {
    Type     string                 `json:"type"` // text, thinking, tool_use, tool_result
    Text     string                 `json:"text,omitempty"`
    Thinking string                 `json:"thinking,omitempty"`
    ToolUse  *ToolUse              `json:"tool_use,omitempty"`
    ToolResult *ToolResult         `json:"tool_result,omitempty"`
}
```

#### 会话生命周期

```
创建会话
    │
    ├─► 自动检测项目路径
    ├─► 提取 Git 分支
    ├─► 生成唯一 ID (UUID)
    │
    ▼
对话循环
    │
    ├─► 添加用户消息
    ├─► 添加助手响应
    ├─► 更新 token 使用量
    ├─► 自动保存 (每次响应后)
    │
    ▼
会话管理
    │
    ├─► 列出会话 (按时间排序)
    ├─► 恢复会话 (加载历史)
    ├─► 压缩会话 (精简历史以节省 token)
    ├─► 标题提取 (从首条消息)
    │
    ▼
持久化
    │
    └─► 保存到 ~/.config/agentic-coder/sessions/<id>.json
```

#### 会话压缩

当对话历史过长时,可以压缩会话以节省 token:

```bash
# 在对话中执行
/compact

# 或通过命令行
./bin/agentic-coder --session <id> --compact
```

压缩算法:
1. 保留最近 N 条消息
2. 对早期消息进行摘要
3. 保留关键的工具调用和结果
4. 更新 token 估算

### 6. 认证系统 (`pkg/auth/`)

认证系统管理多个 provider 的凭证。核心代码约 80 行。

#### 认证类型

```go
type CredentialType string

const (
    CredentialTypeAPIKey CredentialType = "api_key"
    CredentialTypeOAuth  CredentialType = "oauth"
)

type Credential struct {
    Provider    string         `json:"provider"`
    Type        CredentialType `json:"type"`
    APIKey      string         `json:"api_key,omitempty"`
    OAuthToken  string         `json:"oauth_token,omitempty"`
    ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
    UpdatedAt   time.Time      `json:"updated_at"`
}
```

#### 认证管理器

```go
type Manager struct {
    storage *storage.Storage
}

// 保存凭证
func (m *Manager) SaveCredential(cred *Credential) error

// 获取凭证
func (m *Manager) GetCredential(provider string) (*Credential, error)

// 删除凭证
func (m *Manager) DeleteCredential(provider string) error

// 列出所有凭证
func (m *Manager) ListCredentials() ([]*Credential, error)
```

#### 凭证存储

凭证文件: `~/.config/agentic-coder/credentials.json`

```json
{
  "credentials": [
    {
      "provider": "claude",
      "type": "api_key",
      "api_key": "sk-ant-...",
      "updated_at": "2025-01-15T10:30:00Z"
    },
    {
      "provider": "openai",
      "type": "api_key",
      "api_key": "sk-proj-...",
      "updated_at": "2025-01-15T10:35:00Z"
    }
  ]
}
```

**安全措施**:
- 文件权限设置为 `0600` (仅所有者可读写)
- API key 不会在日志或输出中显示
- 支持通过环境变量覆盖 (`ANTHROPIC_API_KEY` 等)

### 7. 成本追踪 (`pkg/cost/`)

实时追踪 token 使用量和成本。

#### 成本追踪器

```go
type Tracker struct {
    pricing map[string]*ModelPricing
}

type ModelPricing struct {
    Provider       string
    Model          string
    InputPer1M     float64 // 美元/1M tokens
    OutputPer1M    float64
    CachePer1M     float64 // 缓存价格
}

type Usage struct {
    InputTokens  int64
    OutputTokens int64
    CacheTokens  int64
    TotalCost    float64
}
```

#### 预配置价格 (2025年2月)

| Provider | 模型 | 输入 ($/1M) | 输出 ($/1M) |
|----------|------|-------------|-------------|
| Claude | Sonnet 4.5 | $3 | $15 |
| Claude | Opus 4.5 | $15 | $75 |
| Claude | Haiku 4 | $0.8 | $4 |
| OpenAI | GPT-4o | $5 | $15 |
| OpenAI | GPT-4 | $30 | $60 |
| OpenAI | O1 | $15 | $60 |
| Gemini | 2.5 Pro | $1.25 | $5 |
| Gemini | 2.5 Flash | $0.1 | $0.4 |
| DeepSeek | V3 | $0.14 | $0.28 |
| Ollama | All models | $0 | $0 |

#### 使用方式

```bash
# 在对话中查看成本
/cost

# 输出示例:
# Token Usage:
#   Input:  12,450 tokens
#   Output: 3,280 tokens
#   Total:  15,730 tokens
# Estimated Cost: $0.086
```

### 8. 代码审查 (`pkg/review/`)

集成代码审查管道,自动检查代码质量。核心代码约 342 行。

#### 审查流程

```
代码更改
    │
    ▼
┌──────────────────┐
│  检测更改文件    │ (通过 git diff)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  安全检查        │ ─► SQL 注入、XSS、命令注入
│                  │    硬编码密钥、不安全函数
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  代码质量        │ ─► 复杂度分析、代码异味
│                  │    命名规范、注释覆盖
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  最佳实践        │ ─► 错误处理、资源泄漏
│                  │    并发安全、性能问题
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  生成报告        │ ─► 问题列表、严重程度
│                  │    建议修复方案
└──────────────────┘
```

#### 审查配置

```go
type Config struct {
    // 审查严格程度
    Strictness string // "strict", "normal", "loose"

    // 自动审查间隔
    AutoReviewEvery int // 每 N 次响应后自动审查

    // 增量审查
    IncrementalOnly bool // 仅审查更改的代码

    // 忽略规则
    IgnorePatterns []string
}
```

#### 启用审查

```bash
# 启动时启用
./bin/agentic-coder --review

# 配置审查选项
./bin/agentic-coder --review --review-strict=normal --review-every=5

# 手动触发审查
/review
```

### 9. 工作上下文管理 (`pkg/workctx/`)

当在 AI provider 之间切换时,工作上下文帮助维护任务连续性。核心代码约 442 行。

#### 使用场景

1. **Token 耗尽**: 当前 provider 的 token 用完,切换到另一个
2. **成本优化**: 简单任务用便宜的模型,复杂任务切换到高级模型
3. **能力需求**: 某些任务需要特定 provider 的特殊能力
4. **多人协作**: 不同团队成员使用不同 provider

#### 工作上下文结构

```go
type WorkContext struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Goal        string    `json:"goal"`
    Background  string    `json:"background"`

    // 任务跟踪
    Completed   []string  `json:"completed"`
    Pending     []string  `json:"pending"`

    // 关键文件
    KeyFiles    []string  `json:"key_files"`

    // 笔记和元数据
    Notes       []string  `json:"notes"`

    // Token 使用记录
    TokenUsageByProvider map[string]int64 `json:"token_usage_by_provider"`

    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

#### 交接摘要

生成的交接摘要包含:

```markdown
# 工作交接摘要

## 任务目标
实现用户认证功能

## 背景信息
项目使用 Go + PostgreSQL,需要添加 OAuth2 登录

## 已完成
- [x] 创建用户模型 (pkg/models/user.go)
- [x] 实现数据库迁移 (migrations/001_users.sql)
- [x] 添加 JWT token 生成 (pkg/auth/jwt.go)

## 待完成
- [ ] 实现 OAuth2 登录端点
- [ ] 添加刷新 token 逻辑
- [ ] 编写单元测试

## 关键文件
- pkg/auth/oauth.go
- pkg/models/user.go
- pkg/handlers/auth.go

## 重要说明
- 使用 JWT 作为 token 格式
- Token 有效期设置为 24 小时
- 需要支持 Google 和 GitHub OAuth

## Token 使用情况
- Claude Sonnet: 45,230 tokens ($0.23)
- GPT-4o: 12,890 tokens ($0.11)
- Total: 58,120 tokens ($0.34)
```

### 10. TUI 模式 (`pkg/tui/`)

基于 Bubble Tea 的终端用户界面。核心代码约 1,850 行。

#### TUI 特性

- **分屏显示**: 上半屏对话,下半屏输入
- **实时流式**: 流式显示 AI 响应
- **Markdown 渲染**: 使用 Glamour 美化输出
- **会话管理**: 内置会话切换 UI
- **成本显示**: 实时显示 token 使用和成本
- **审查状态**: 显示审查周期和结果
- **思考动画**: 显示 AI 思考过程的动画
- **待办项显示**: 实时展示任务进度

#### 启动 TUI 模式

```bash
./bin/agentic-coder -t
# 或
./bin/agentic-coder --tui
```

#### TUI 架构

```go
type Model struct {
    // 视图组件
    viewport     viewport.Model
    textarea     textarea.Model
    spinner      spinner.Model

    // 状态
    messages     []DisplayMessage
    streaming    bool
    thinking     bool

    // 配置
    width        int
    height       int

    // 回调到 engine
    onSubmit     func(string) error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
```

### 11. 权限系统 (`pkg/permission/`)

细粒度的工具权限控制系统。

#### 权限模式

```go
type PermissionMode string

const (
    PermissionModeDefault       PermissionMode = "default"          // 每次询问
    PermissionModePlan          PermissionMode = "plan"             // 计划模式,仅读取
    PermissionModeAcceptEdits   PermissionMode = "accept_edits"    // 自动接受编辑
    PermissionModeDontAsk       PermissionMode = "dont_ask"         // 直接执行
    PermissionModeBypass        PermissionMode = "bypass"           // 绕过所有权限
)
```

#### 权限规则

```go
type PermissionRule struct {
    Tool    string   // 工具名称
    Allow   bool     // 允许/拒绝
    Paths   []string // 路径模式 (glob)
    Commands []string // 命令模式
}
```

#### 使用示例

```bash
# 启动时设置权限模式
./bin/agentic-coder --permission accept_edits

# 配置权限规则
# ~/.config/agentic-coder/permissions.yaml
rules:
  - tool: Bash
    allow: false
    commands: ["rm -rf *", "dd *"]
  - tool: Write
    allow: true
    paths: ["src/**/*.go"]
```

### 12. 技能系统 (`pkg/skill/`)

用户自定义技能(Slash 命令)系统。核心代码约 468 行。

#### 技能定义

```yaml
---
name: commit
description: Create a git commit with staged changes
aliases: [ci, com]
arguments:
  - name: message
    type: string
    required: false
    description: Commit message
---
# Skill Prompt
Review the staged changes and create a meaningful commit message.
Follow conventional commit format.
```

#### 技能加载

```go
type Skill struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Aliases     []string `yaml:"aliases"`
    Arguments   []Arg    `yaml:"arguments"`
    Content     string   // Markdown content
}

// 从文件加载
func LoadSkill(path string) (*Skill, error)

// 从插件加载
func LoadPluginSkills(pluginDir string) ([]*Skill, error)
```

#### 使用技能

```bash
# 在对话中
/commit
/commit -m "feat: add new feature"

# 自定义技能
/review-pr 123
```

### 13. 钩子系统 (`pkg/hook/`)

生命周期钩子允许在关键时刻注入自定义逻辑。

#### 支持的钩子事件

```go
type HookEvent string

const (
    HookEventPreToolUse      HookEvent = "PreToolUse"
    HookEventPostToolUse     HookEvent = "PostToolUse"
    HookEventStop            HookEvent = "Stop"
    HookEventSubagentStop    HookEvent = "SubagentStop"
    HookEventSessionStart    HookEvent = "SessionStart"
    HookEventSessionEnd      HookEvent = "SessionEnd"
    HookEventUserPromptSubmit HookEvent = "UserPromptSubmit"
    HookEventPreCompact      HookEvent = "PreCompact"
    HookEventNotification    HookEvent = "Notification"
)
```

#### 钩子配置

```yaml
# ~/.config/agentic-coder/hooks.yaml
hooks:
  - event: PreToolUse
    condition: "tool == 'Bash' && input.command.contains('rm')"
    action: confirm
    message: "Dangerous command detected. Continue?"

  - event: PostToolUse
    condition: "tool == 'Write'"
    action: exec
    command: "git add {{ .file_path }}"
```

### 14. MCP 集成 (`pkg/mcp/`)

Model Context Protocol 支持外部工具生态系统。

#### 支持的服务器类型

```go
type ServerType string

const (
    ServerTypeStdio ServerType = "stdio" // 标准输入/输出
    ServerTypeSSE   ServerType = "sse"   // 服务器发送事件
    ServerTypeHTTP  ServerType = "http"  // HTTP 连接
)
```

#### MCP 配置

```json
// ~/.config/agentic-coder/mcp.json
{
  "servers": {
    "filesystem": {
      "type": "stdio",
      "command": "mcp-server-filesystem",
      "args": ["--root", "/path/to/project"]
    },
    "database": {
      "type": "http",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

#### 动态工具注册

```go
// MCP 工具自动注册到工具注册表
func (m *MCPManager) RegisterTools(registry *tool.Registry) error {
    for _, server := range m.servers {
        tools, err := server.ListTools()
        if err != nil {
            continue
        }
        for _, tool := range tools {
            registry.Register(NewMCPTool(tool, server))
        }
    }
    return nil
}
```

### 15. Task 子系统 (`pkg/task/`)

子 Agent 任务创建和管理系统。核心代码约 439 行。

#### Task 类型

```go
type Task struct {
    ID          string
    Type        string // agent, shell, remote
    Status      string // running, completed, failed
    Output      string
    Error       error
    CreatedAt   time.Time
    CompletedAt *time.Time
}
```

#### 子 Agent 类型

```go
const (
    SubagentTypeGeneral        = "general-purpose"
    SubagentTypeExplore        = "Explore"
    SubagentTypePlan           = "Plan"
    SubagentTypeCodeReview     = "code-review"
    SubagentTypePluginDev      = "plugin-dev"
    SubagentTypeCodeSimplifier = "code-simplifier"
)
```

#### 使用示例

```bash
# 启动子任务
Task tool with:
{
  "subagent_type": "Explore",
  "prompt": "Find all error handling patterns in the codebase",
  "model": "haiku"
}

# 后台运行
Task tool with run_in_background=true

# 获取输出
TaskOutput tool with task_id=<id>
```

### 16. 存储抽象 (`pkg/storage/`)

统一的存储抽象层,支持键值对存储。核心代码约 404 行。

#### 存储接口

```go
type Storage interface {
    Get(key string) ([]byte, error)
    Set(key string, value []byte) error
    Delete(key string) error
    List(prefix string) ([]string, error)
    Exists(key string) bool
}
```

#### 文件系统实现

```go
type FileStorage struct {
    basePath string
    mu       sync.RWMutex
}

// 原子写入
func (fs *FileStorage) Set(key string, value []byte) error {
    tmpFile := filepath.Join(fs.basePath, key+".tmp")
    finalFile := filepath.Join(fs.basePath, key)

    // 写入临时文件
    if err := os.WriteFile(tmpFile, value, 0600); err != nil {
        return err
    }

    // 原子重命名
    return os.Rename(tmpFile, finalFile)
}
```

## 数据流

### 完整请求-响应流程

```
用户输入
    │
    ▼
┌─────────────────────────────────────────┐
│ 1. CLI Layer                            │
│    - 解析命令/消息                       │
│    - 处理 /commands                      │
│    - 钩子: UserPromptSubmit             │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 2. Session Layer                        │
│    - 添加用户消息到历史                  │
│    - 更新会话时间戳                      │
│    - 钩子: SessionStart (首次)          │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 3. Engine Layer                         │
│    - 构建请求 (系统提示词 + 工具 + 历史) │
│    - 配置 thinking 参数                  │
│    - 加载技能和钩子                      │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 4. Provider Layer                       │
│    - 调用 API/CLI                        │
│    - 建立流式连接                        │
│    - 模型别名解析                        │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 5. Stream Processing                    │
│    ┌─────────────────────────────────┐  │
│    │ MessageStartEvent               │  │
│    │  ├─► 初始化响应对象              │  │
│    │  └─► 回调: OnMessageStart       │  │
│    └──────────────┬──────────────────┘  │
│                   │                     │
│    ┌──────────────▼──────────────────┐  │
│    │ ContentBlockDelta (循环)        │  │
│    │  ├─► 累积文本                    │  │
│    │  ├─► 累积 thinking               │  │
│    │  ├─► 累积工具输入                │  │
│    │  └─► 回调: OnText/OnThinking    │  │
│    └──────────────┬──────────────────┘  │
│                   │                     │
│    ┌──────────────▼──────────────────┐  │
│    │ ContentBlockStop                │  │
│    │  ├─► 完成内容块                  │  │
│    │  ├─► 钩子: PreToolUse           │  │
│    │  └─► 如果是 tool_use: 执行工具   │  │
│    └──────────────┬──────────────────┘  │
│                   │                     │
│    ┌──────────────▼──────────────────┐  │
│    │ MessageStopEvent                │  │
│    │  ├─► 完成响应                    │  │
│    │  ├─► 钩子: Stop                 │  │
│    │  └─► 回调: OnMessageComplete    │  │
│    └─────────────────────────────────┘  │
└─────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 6. Tool Execution (if any)              │
│    - 查找工具                            │
│    - 验证输入                            │
│    - 权限检查                            │
│    - 执行工具                            │
│    - 钩子: PostToolUse                  │
│    - 返回结果                            │
│    - 回调: OnToolUse, OnToolResult      │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 7. Continue Loop (if tool_use)          │
│    - 将工具结果添加到消息                 │
│    - 重新调用 provider                   │
│    - 继续流式处理                        │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 8. Save to Session                      │
│    - 保存助手响应                        │
│    - 更新 token 使用量                   │
│    - 更新成本统计                        │
│    - 持久化到磁盘                        │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ 9. UI Rendering                         │
│    - 显示最终响应                        │
│    - 更新 TUI/CLI 界面                   │
│    - 显示成本和统计                      │
└─────────────────────────────────────────┘
```

### CLI Provider 特殊处理

本地 CLI provider (ClaudeCLI, CodexCLI, GeminiCLI) 的流式处理:

```
CLI 进程启动
    │
    ├─► 传递参数和消息
    ├─► 捕获 stdout 输出
    │
    ▼
逐行读取 stdout
    │
    ├─► 每行是一个 JSON 对象
    ├─► 包含累积的完整响应
    │
    ▼
计算增量
    │
    ├─► delta = currentText - lastText
    ├─► 只发送新增的部分
    │
    ▼
转换为 StreamingEvent
    │
    ├─► 构造 ContentBlockDelta 事件
    ├─► 填充 text_delta 字段
    │
    ▼
发送给 Engine
```

示例 JSONL 输出:

```jsonl
{"type":"text","text":"Let"}
{"type":"text","text":"Let me"}
{"type":"text","text":"Let me help"}
{"type":"text","text":"Let me help you"}
```

转换为增量:

```
Event 1: delta = "Let"
Event 2: delta = " me"
Event 3: delta = " help"
Event 4: delta = " you"
```

## 配置管理

### 目录结构

```
~/.config/agentic-coder/
├── credentials.json         # 凭证存储 (0600)
├── config.yaml             # 用户配置
├── permissions.yaml        # 权限规则
├── hooks.yaml              # 生命周期钩子
├── mcp.json                # MCP 服务器配置
├── sessions/               # 会话历史
│   ├── <uuid-1>.json
│   ├── <uuid-2>.json
│   └── ...
├── work/                   # 工作上下文
│   ├── <id-1>.json
│   ├── <id-2>.json
│   └── ...
└── skills/                 # 用户技能
    ├── commit.md
    ├── review-pr.md
    └── ...
```

### 环境变量

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `ANTHROPIC_API_KEY` | Claude API 认证 | - |
| `OPENAI_API_KEY` | OpenAI API 认证 | - |
| `GOOGLE_API_KEY` | Gemini API 认证 | - |
| `DEEPSEEK_API_KEY` | DeepSeek API 认证 | - |
| `OLLAMA_HOST` | Ollama 服务器 URL | `http://localhost:11434` |
| `INTEGRATION_TEST` | 启用集成测试 | `0` |
| `AGENTIC_CODER_CONFIG_DIR` | 自定义配置目录 | `~/.config/agentic-coder` |

### 指令文件

项目支持两种指令文件:

1. **AGENT.md** (推荐)
   - 新的标准指令文件名
   - 项目级: 项目根目录
   - 全局级: `~/.claude/AGENT.md`

2. **CLAUDE.md** (向后兼容)
   - 旧版指令文件名
   - 项目级: 项目根目录
   - 全局级: `~/.claude/CLAUDE.md`

**优先级**: AGENT.md > CLAUDE.md

**自动迁移提示**: 如果只检测到 CLAUDE.md,会提示用户迁移。

## 错误处理

### 分层错误处理

```
Application Error
    │
    ├─► Provider Error
    │   ├─► API Error (HTTP 状态码)
    │   ├─► Network Error
    │   ├─► Timeout Error
    │   └─► Rate Limit Error
    │
    ├─► Tool Error
    │   ├─► Validation Error
    │   ├─► Execution Error
    │   └─► Permission Error
    │
    ├─► Session Error
    │   ├─► Load Error
    │   ├─► Save Error
    │   └─► Corruption Error
    │
    └─► System Error
        ├─► Context Canceled
        ├─► File System Error
        └─► Configuration Error
```

### Provider 错误

```go
type APIError struct {
    StatusCode int
    Message    string
    Type       string
    Provider   string
}

func (e *APIError) Error() string {
    return fmt.Sprintf("%s API error (%d): %s", e.Provider, e.StatusCode, e.Message)
}
```

常见错误码:

| 状态码 | 含义 | 处理方式 |
|--------|------|---------|
| 401 | 认证失败 | 提示检查 API key |
| 429 | 速率限制 | 等待后重试 |
| 500 | 服务器错误 | 重试或切换 provider |
| 503 | 服务不可用 | 稍后重试 |

### Tool 错误

工具错误不会中断对话,而是返回给 AI:

```go
if err != nil {
    return &tool.Output{
        Content: fmt.Sprintf("Error: %v", err),
        IsError: true,
    }, nil
}
```

### Context 取消

所有长时间运行的操作都支持 context 取消:

```go
func (e *Engine) Run(ctx context.Context, input string) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // 继续执行
    }
}
```

用户操作:
- `Ctrl+C` 一次: 取消当前操作
- `Ctrl+C` 两次: 退出程序

## 性能优化

### 1. 流式响应

所有 provider 都支持流式响应,减少感知延迟:

- **TTFB (Time To First Byte)**: 首字节时间通常 < 1s
- **增量更新**: 实时显示生成的文本
- **取消支持**: 随时中断不需要的响应

### 2. 会话管理

- **内存缓存**: 当前会话保存在内存中
- **延迟加载**: 仅在需要时加载历史会话
- **压缩算法**: 定期压缩长会话以节省 token

### 3. 连接复用

使用 `http.Client` 的连接池:

```go
var defaultClient = &http.Client{
    Timeout: 120 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

### 4. 工具并行化 (未来)

计划支持独立工具的并行执行:

```go
// 当 AI 请求多个独立工具时
var wg sync.WaitGroup
for _, toolCall := range toolCalls {
    wg.Add(1)
    go func(tc ToolCall) {
        defer wg.Done()
        result := executeTool(tc)
        results <- result
    }(toolCall)
}
wg.Wait()
```

## 安全性

### 1. 凭证安全

- **文件权限**: `credentials.json` 设置为 `0600`
- **不记录密钥**: API key 永远不会出现在日志中
- **内存保护**: 使用完毕后清零敏感字符串

```go
func (m *Manager) SaveCredential(cred *Credential) error {
    // 设置严格权限
    if err := os.Chmod(credFile, 0600); err != nil {
        return err
    }
    // ...
}
```

### 2. 输入验证

所有工具输入都经过验证:

```go
func (t *BashTool) Execute(ctx context.Context, input map[string]interface{}) (*Output, error) {
    // 验证必需参数
    command, ok := input["command"].(string)
    if !ok || command == "" {
        return &Output{
            Content: "Error: 'command' parameter is required",
            IsError: true,
        }, nil
    }

    // 路径验证
    if strings.Contains(command, "..") {
        return &Output{
            Content: "Error: path traversal not allowed",
            IsError: true,
        }, nil
    }

    // ...
}
```

### 3. Bash 沙箱 (计划中)

未来将实现 Bash 工具的沙箱限制:

- 禁止危险命令 (`rm -rf /`, `dd`, etc.)
- 限制文件系统访问范围
- 限制网络访问
- 资源限制 (CPU, 内存, 时间)

### 4. Permission 系统

支持工具权限请求:

```go
type PermissionMode string

const (
    PermissionModeDefault     PermissionMode = "default"       // 每次询问
    PermissionModeDontAsk     PermissionMode = "dont_ask"      // 自动允许
    PermissionModeAcceptEdits PermissionMode = "accept_edits"  // 仅文件编辑
    PermissionModePlan        PermissionMode = "plan"          // 仅读取
    PermissionModeBypass      PermissionMode = "bypass"        // 绕过所有
)
```

## 测试策略

### 测试结构

```
pkg/
├── engine/
│   ├── loop.go
│   └── loop_test.go
├── provider/
│   ├── types.go
│   ├── types_test.go
│   ├── claude/
│   │   ├── claude.go
│   │   └── claude_test.go
│   └── ...
└── tool/
    ├── tool.go
    ├── tool_test.go
    └── builtin/
        ├── read_tool.go
        └── read_tool_test.go
```

### 单元测试

```bash
# 运行所有测试
go test ./...

# 详细输出
go test ./... -v

# 测试单个包
go test ./pkg/engine -v

# 测试覆盖率
go test ./... -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 集成测试

集成测试需要真实的 API key,默认跳过:

```bash
# 启用集成测试
export INTEGRATION_TEST=1
export ANTHROPIC_API_KEY="your-key"
go test ./pkg/provider/claude -v
```

测试示例:

```go
func TestClaudeProvider_Integration(t *testing.T) {
    if os.Getenv("INTEGRATION_TEST") != "1" {
        t.Skip("Skipping integration test")
    }

    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        t.Fatal("ANTHROPIC_API_KEY not set")
    }

    provider := claude.New(apiKey)
    // ... 测试实际 API 调用
}
```

### Mock 测试

使用 mock provider 进行单元测试:

```go
type MockProvider struct {
    responses []string
    index     int
}

func (m *MockProvider) CreateMessageStream(ctx context.Context, req *Request) (StreamReader, error) {
    return &MockStreamReader{
        response: m.responses[m.index],
    }, nil
}
```

## 构建和部署

### 构建系统

使用 Makefile 管理构建:

```makefile
# 主要目标
build:          编译二进制文件
run:            编译并运行
test:           运行测试
test-coverage:  生成覆盖率报告
clean:          清理构建产物
install:        安装到 $GOPATH/bin
lint:           运行 linter
fmt:            格式化代码
tidy:           整理依赖
dev:            开发模式 (启用 race detector)
```

### 构建命令

```bash
# 标准构建
make build
# 输出: ./bin/agentic-coder

# 开发构建 (带 race detector)
make dev

# 安装到系统
make install

# 交叉编译 (示例)
GOOS=linux GOARCH=amd64 make build
GOOS=windows GOARCH=amd64 make build
GOOS=darwin GOARCH=arm64 make build
```

### 版本注入

构建时注入版本信息:

```makefile
VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X main.Version=$(VERSION) \
           -X main.Commit=$(COMMIT) \
           -X main.BuildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/agentic-coder ./cmd/agentic-coder
```

查看版本:

```bash
./bin/agentic-coder version
# 输出:
# Version:    0.1.0
# Commit:     e4df18c
# Built:      2025-02-05T10:30:00Z
# Go version: go1.24.2
```

### 发布流程

1. **更新版本号**
   ```bash
   # 更新 Makefile 中的 VERSION
   VERSION ?= 0.2.0
   ```

2. **运行测试**
   ```bash
   make test
   make lint
   ```

3. **构建所有平台**
   ```bash
   make build-all  # 如果有多平台构建目标
   ```

4. **创建 Git tag**
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```

5. **发布 GitHub Release**
   - 上传编译好的二进制文件
   - 附上 CHANGELOG

## 开发指南

### 代码规范

#### 导入分组

```go
import (
    // 标准库
    "context"
    "fmt"
    "os"

    // 第三方库
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

    // 本地包
    "github.com/xinguang/agentic-coder/pkg/engine"
    "github.com/xinguang/agentic-coder/pkg/provider"
)
```

#### 错误处理

```go
// 包装错误,添加上下文
if err != nil {
    return fmt.Errorf("failed to load session: %w", err)
}

// 自定义错误类型
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}
```

#### 命名约定

- **包名**: 小写,单数形式 (`engine`, `provider`, `tool`)
- **接口**: 名词或形容词 (`Provider`, `Tool`, `Reader`)
- **实现**: 具体名称 (`ClaudeProvider`, `ReadTool`)
- **方法**: 动词开头 (`CreateMessage`, `Execute`, `Build`)
- **常量**: 大驼峰 (`ProviderClaude`, `FeatureToolUse`)

#### 注释规范

```go
// Package engine implements the core agentic loop for AI interactions.
// It coordinates message streaming, tool execution, and callback management.
package engine

// Engine orchestrates the conversation loop between user input,
// AI responses, and tool execution.
type Engine struct {
    provider provider.AIProvider
    tools    *tool.Registry
}

// Run executes a single turn of the conversation loop.
// It streams the provider's response and executes any requested tools.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - userInput: The user's message
//
// Returns an error if the provider call fails or if context is canceled.
func (e *Engine) Run(ctx context.Context, userInput string) error {
    // ...
}
```

## 未来规划

### 短期 (1-3 个月)

- [x] **MCP (Model Context Protocol) 支持**
  - 实现 MCP 服务器接口
  - 支持远程工具调用
  - 多 MCP 服务器编排

- [ ] **插件系统增强**
  - 定义插件接口
  - 动态加载插件
  - 插件市场

- [ ] **高级 Bash 沙箱**
  - 命令白名单/黑名单
  - 文件系统权限限制
  - 资源使用限制

- [x] **改进的 TUI**
  - ✓ 思考动画
  - ✓ 待办项显示
  - [ ] 多 pane 布局
  - [ ] 快捷键自定义
  - [ ] 主题支持

### 中期 (3-6 个月)

- [ ] **多智能体系统**
  - Agent 编排框架
  - Agent 间通信协议
  - 专业化 Agent (代码审查、测试生成等)

- [x] **代码审查增强**
  - ✓ 自动审查
  - ✓ 增量审查
  - [ ] 更多语言支持
  - [ ] 自定义规则
  - [ ] CI/CD 集成

- [ ] **工具并行化**
  - 自动检测独立工具
  - 并行执行框架
  - 结果聚合

- [x] **配置系统**
  - ✓ YAML 配置文件
  - ✓ 项目级配置
  - [ ] Profile 管理

### 长期 (6+ 个月)

- [ ] **IDE 集成**
  - VS Code 扩展
  - JetBrains 插件
  - LSP 服务器模式

- [ ] **Web UI**
  - 基于 Web 的界面
  - 多用户支持
  - 远程访问

- [ ] **云服务**
  - 托管版本
  - Team 协作
  - 中央化会话存储

- [ ] **高级分析**
  - 使用模式分析
  - 成本优化建议
  - 性能监控

## 项目度量

### 代码统计 (截至 2025-02-05)

| 指标 | 数值 |
|------|------|
| 总代码行数 | ~16,000+ |
| Go 源文件 | 86+ |
| 测试文件 | 13 |
| 主要包数 | 23 |
| 内置工具 | 22 |
| 支持的 Provider | 8+ |
| 二进制大小 | ~10MB |
| Go 版本 | 1.24.2+ |
| 主要依赖 | 8 |
| 平均启动时间 | < 100ms |
| 首字节响应时间 | < 1s |

### 架构指标

| 组件 | 代码量 | 复杂度 | 测试覆盖率 (目标) |
|------|--------|--------|-------------------|
| Provider | ~4,865行 | 中 | 70%+ |
| Engine | ~13KB | 高 | 80%+ |
| Tool | ~246行核心 | 低-中 | 80%+ |
| Session | ~544行 | 中 | 75%+ |
| Auth | ~80行 | 低 | 80%+ |
| Review | ~342行 | 中 | 70%+ |
| WorkCtx | ~442行 | 中 | 70%+ |
| TUI | ~1,850行 | 高 | 60%+ |
| Task | ~439行 | 中 | 70%+ |
| Storage | ~404行 | 低 | 80%+ |
| Skill | ~468行 | 中 | 70%+ |

### 性能基准 (参考值)

| 操作 | 时间 |
|------|------|
| 启动时间 | < 100ms |
| 首字节时间 (TTFB) | < 1s |
| 会话加载 | < 50ms |
| 工具执行 | < 500ms (平均) |
| 二进制大小 | ~10MB |

## 常见问题

### Q1: 如何切换 Provider?

**方法 1: 启动时指定**
```bash
./bin/agentic-coder -m gpt-4o
./bin/agentic-coder -m gemini-2.5-flash
```

**方法 2: 对话中切换**
```
/model gpt-4o
```

### Q2: 如何使用本地 CLI Provider?

确保 CLI 工具已安装并在 PATH 中:

```bash
# 安装 Claude Code (示例)
npm install -g @anthropic-ai/claude-code

# 使用
./bin/agentic-coder -m claudecli
```

### Q3: 如何管理成本?

1. **查看当前成本**
   ```
   /cost
   ```

2. **使用便宜的模型**
   ```bash
   ./bin/agentic-coder -m haiku  # Claude Haiku
   ./bin/agentic-coder -m gemini-2.5-flash  # Gemini Flash
   ```

3. **使用免费的 Ollama**
   ```bash
   ./bin/agentic-coder -m llama3.2
   ```

4. **压缩长会话**
   ```
   /compact
   ```

### Q4: 如何在团队中使用?

1. **共享工作上下文**
   ```bash
   # 成员 A (使用 Claude)
   agentic-coder work new "Feature X"
   # ... 工作
   agentic-coder work handoff <id> -o handoff.md

   # 成员 B (使用 GPT-4)
   agentic-coder -m gpt-4o
   # 粘贴 handoff.md 内容,继续工作
   ```

2. **统一配置**
   - 将项目级 AGENT.md 提交到 Git
   - 团队成员共享相同的系统提示词

### Q5: 如何调试 Provider 问题?

```bash
# 启用详细输出
./bin/agentic-coder -v

# 检查凭证
./bin/agentic-coder auth status

# 测试 API key
export ANTHROPIC_API_KEY="your-key"
curl -H "X-API-Key: $ANTHROPIC_API_KEY" \
     -H "Content-Type: application/json" \
     https://api.anthropic.com/v1/messages \
     -d '{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"Hi"}],"max_tokens":10}'
```

### Q6: 如何贡献代码?

1. Fork 项目
2. 创建 feature 分支
3. 遵循代码规范 (见上文)
4. 添加测试
5. 提交 Pull Request

详见项目 CONTRIBUTING.md (如有)。

## 参考资源

### 官方文档

- [Anthropic Claude API](https://docs.anthropic.com/)
- [OpenAI API](https://platform.openai.com/docs)
- [Google Gemini API](https://ai.google.dev/)
- [DeepSeek API](https://www.deepseek.com/docs)
- [Ollama](https://ollama.ai/)

### Go 相关

- [Go 官方文档](https://go.dev/doc/)
- [Cobra CLI 框架](https://github.com/spf13/cobra)
- [Bubble Tea TUI](https://github.com/charmbracelet/bubbletea)

### 相关项目

- [Claude Code](https://github.com/anthropics/claude-code)
- [OpenAI Codex](https://github.com/openai/codex)
- [Continue.dev](https://github.com/continuedev/continue)

## 许可证

MIT License - 详见项目 LICENSE 文件

## 关键设计亮点

### 1. 架构优势

- **清晰的分层架构**: CLI → Engine → Provider → Tool → Support Services
- **接口驱动设计**: 大量使用接口实现多态和解耦
- **单一职责原则**: 每个包和模块职责明确
- **高度可扩展**: Provider 和 Tool 系统支持动态扩展

### 2. Provider 抽象的价值

**核心价值**: 解决 AI 工具领域的厂商锁定问题

- **统一接口**: 所有 Provider 实现相同的 `AIProvider` 接口
- **零侵入切换**: 通过 `-m` 参数即可切换 Provider
- **支持多样性**: API、CLI、本地部署三种方式
- **易于扩展**: 添加新 Provider 仅需实现接口并注册

### 3. 工具生态系统

- **22 个内置工具**: 覆盖文件操作、Shell、Web、开发等场景
- **动态注册**: 工具通过注册表模式动态管理
- **MCP 集成**: 支持外部工具生态系统扩展
- **权限控制**: 细粒度的工具执行权限管理

### 4. 流式优先设计

- **所有 Provider 支持流式**: 提供一致的实时响应体验
- **低延迟**: 首字节时间 < 1s
- **可取消**: 支持 Context 取消机制
- **回调驱动**: Engine 通过回调与 UI 解耦

### 5. 工程质量

- **错误处理完善**: 分层错误处理，错误传播清晰
- **并发安全**: 使用互斥锁保护共享状态
- **测试覆盖**: 目标 70%+ 覆盖率
- **文档完善**: 代码注释详细，文档齐全

## 技术债务与改进方向

### 当前技术债务

1. **Provider 实现重复**: 各 Provider 有相似代码，可提取公共逻辑
2. **测试覆盖率**: 当前仅 13 个测试文件，需增加单元测试
3. **配置管理**: 配置文件格式可以更加统一
4. **错误处理**: 部分错误处理可以更细致

### 短期改进建议

1. **提高测试覆盖率**: 重点覆盖 Engine、Provider、Tool
2. **提取 Provider 公共逻辑**: 创建 `BaseProvider` 减少重复代码
3. **增强文档**: 添加更多使用示例和最佳实践
4. **性能优化**: 实现工具并行化执行

### 长期规划

1. ~~**多智能体系统**: Agent 编排和协作~~ ✅ 已在 v0.2.0 实现
2. **IDE 集成**: VS Code、JetBrains 插件
3. **Web UI**: 基于 Web 的用户界面
4. **云服务**: 托管版本和团队协作
5. **高级工作流特性**: 动态重规划、任务间通信

## 维护者

- **项目创建者**: @xinguang
- **当前版本**: v0.2.0
- **最后更新**: 2025-02-05
- **文档版本**: v1.2.0

---

**本文档基于深度代码分析生成，随项目持续更新。如有疑问或建议，请提交 Issue。**
