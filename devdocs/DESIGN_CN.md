# Agentic Coder - 设计文档

## 概述

Agentic Coder 是一个用 Go 语言构建的 AI 编程助手 CLI 工具。它提供了一个统一的接口来与多个 AI 提供商交互，支持 API 调用和本地 CLI 工具两种方式。

## 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI (main.go)                           │
├─────────────────────────────────────────────────────────────────┤
│                          引擎层                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Engine    │  │   Prompt    │  │   Session Manager       │ │
│  │   (循环)    │  │   Builder   │  │   (持久化)              │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                        Provider 层                              │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │
│  │ Claude │ │ OpenAI │ │ Gemini │ │DeepSeek│ │   Ollama     │  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └──────────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                        │
│  │ClaudeCLI │ │ CodexCLI │ │GeminiCLI │  (本地 CLI Provider)   │
│  └──────────┘ └──────────┘ └──────────┘                        │
├─────────────────────────────────────────────────────────────────┤
│                         工具层                                   │
│  ┌──────┐ ┌───────┐ ┌──────┐ ┌──────┐ ┌────────┐ ┌──────────┐  │
│  │ Read │ │ Write │ │ Edit │ │ Bash │ │WebFetch│ │ WebSearch│  │
│  └──────┘ └───────┘ └──────┘ └──────┘ └────────┘ └──────────┘  │
│  ┌──────┐ ┌──────┐ ┌──────────┐ ┌────────┐ ┌─────────────────┐ │
│  │ Glob │ │ Grep │ │ Notebook │ │  Task  │ │    PlanMode     │ │
│  └──────┘ └──────┘ └──────────┘ └────────┘ └─────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                         支撑层                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │    Auth     │  │   Config    │  │       Storage           │ │
│  │  Manager    │  │   Manager   │  │                         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. Provider 系统 (`pkg/provider/`)

Provider 系统通过统一接口抽象 AI 模型交互。

#### 接口定义

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

#### Provider 类型

| 类型 | 描述 | 认证方式 |
|------|------|---------|
| API Provider | 直接 API 调用 | API Key |
| CLI Provider | 封装本地 CLI 工具 | CLI 登录 |

#### 添加新 Provider 步骤

1. 创建目录：`pkg/provider/<name>/`
2. 实现 `AIProvider` 接口
3. 在 `pkg/provider/factory.go` 添加 provider 类型
4. 更新 `DetectProviderFromModel()` 函数
5. 在 `main.go` 的 `createProvider()` 函数添加 case

### 2. 工具系统 (`pkg/tool/`)

工具扩展了 AI 与环境交互的能力。

#### 接口定义

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Execute(ctx context.Context, input map[string]interface{}) (*Output, error)
}
```

#### 内置工具

| 分类 | 工具 |
|------|------|
| 文件操作 | Read, Write, Edit, Glob, Grep |
| Shell | Bash, KillShell |
| 网络 | WebFetch, WebSearch |
| 规划 | EnterPlanMode, ExitPlanMode |
| 高级 | Task, NotebookEdit, LSP |

#### 添加新工具步骤

1. 在 `pkg/tool/builtin/` 创建文件
2. 实现 `Tool` 接口
3. 在 `main.go` 的 `registerBuiltinTools()` 函数中注册

### 3. 引擎 (`pkg/engine/`)

引擎协调用户输入、AI 响应和工具执行之间的对话循环。

#### 主循环流程

```
用户输入
    │
    ▼
┌─────────────────┐
│  构建请求       │
│  (添加工具,     │
│   系统提示词)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  调用 Provider  │◄────────────┐
│  (流式)         │             │
└────────┬────────┘             │
         │                      │
         ▼                      │
┌─────────────────┐             │
│  处理事件       │             │
│ - 文本: 输出    │             │
│ - 工具: 执行    │─────────────┘
│ - 停止: 完成    │   (工具结果)
└────────┬────────┘
         │
         ▼
    响应完成
```

#### 回调系统

```go
type CallbackOptions struct {
    OnText       func(text string)        // 文本输出回调
    OnThinking   func(text string)        // 思考过程回调
    OnToolUse    func(name string, input map[string]interface{})  // 工具使用回调
    OnToolResult func(name string, result *tool.Output)           // 工具结果回调
    OnError      func(err error)          // 错误回调
}
```

### 4. 会话管理 (`pkg/session/`)

会话维护对话历史和上下文。

#### 会话结构

```go
type Session struct {
    ID          string      // 会话 ID
    ProjectPath string      // 项目路径
    CWD         string      // 当前工作目录
    Model       string      // 使用的模型
    Messages    []Message   // 消息历史
    CreatedAt   time.Time   // 创建时间
    UpdatedAt   time.Time   // 更新时间
}
```

#### 持久化

会话自动保存到 `~/.config/agentic-coder/sessions/`。

### 5. 认证管理 (`pkg/auth/`)

管理不同 provider 的凭证。

#### 支持的认证类型

- **API Key**：简单的密钥认证
- **OAuth**：基于令牌的认证（预留）

#### 凭证存储

凭证存储在 `~/.config/agentic-coder/credentials.json`，文件权限为 0600。

## 数据流

### 流式响应处理

```
Provider 流
      │
      ▼
┌─────────────────────┐
│  MessageStartEvent  │ ─► 初始化响应
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ContentBlockDelta    │ ─► 累积文本/工具输入
│  (多个)             │    回调: OnText/OnThinking
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ContentBlockStop     │ ─► 完成块
│                     │    如果是工具则执行
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  MessageStopEvent   │ ─► 完成响应
└─────────────────────┘
```

### CLI Provider 流式处理

本地 CLI provider (claudecli, codexcli, geminicli) 将 stdout 解析为 JSONL：

```
CLI 进程
    │
    ▼ (stdout)
┌─────────────────┐
│ 逐行解析 JSON   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 计算增量        │  (fullText - lastText)
│ 从累积文本      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 发送为          │
│ StreamingEvent  │
└─────────────────┘
```

## 配置

### 目录结构

```
~/.config/agentic-coder/
├── credentials.json    # API 密钥和认证令牌
├── config.yaml         # 用户配置（未来）
└── sessions/           # 会话历史
    ├── <session-id>.json
    └── ...
```

### 环境变量

| 变量 | 用途 |
|------|------|
| `ANTHROPIC_API_KEY` | Claude API 认证 |
| `OPENAI_API_KEY` | OpenAI API 认证 |
| `GOOGLE_API_KEY` | Gemini API 认证 |
| `DEEPSEEK_API_KEY` | DeepSeek API 认证 |
| `OLLAMA_HOST` | Ollama 服务器 URL |

## 错误处理

### Provider 错误

```go
type APIError struct {
    StatusCode int     // HTTP 状态码
    Message    string  // 错误消息
    Type       string  // 错误类型
}
```

### 工具错误

工具错误通过 `Output.IsError` 返回，显示给用户但不会中断对话。

### 上下文取消

- `Ctrl+C` 通过 context 取消当前操作
- 双击 `Ctrl+C` 退出应用

## 测试

### 单元测试

```bash
go test ./...
```

### 集成测试

集成测试需要实际的 API 密钥，默认跳过：

```bash
INTEGRATION_TEST=1 go test ./... -v
```

## 性能考虑

1. **流式传输**：所有 provider 支持流式传输以减少感知延迟
2. **工具并行化**：独立的工具调用可以并行运行（未来）
3. **会话缓存**：会话加载一次后保存在内存中
4. **连接复用**：HTTP 客户端通过 `http.Client` 复用连接

## 安全性

1. **凭证存储**：credentials.json 文件权限 0600
2. **API Key 处理**：密钥不会被记录或显示
3. **工具沙箱**：Bash 工具可配置限制（未来）
4. **输入验证**：所有工具输入在执行前进行验证

## 开发注意事项

### 添加新 Provider

1. **接口一致性**：必须实现完整的 `AIProvider` 接口
2. **流式支持**：必须实现 `StreamReader` 以支持流式响应
3. **错误处理**：正确处理 API 错误和网络错误
4. **Context 支持**：尊重 context 取消信号

### 添加新工具

1. **输入验证**：验证所有必需参数
2. **错误返回**：使用 `Output.IsError` 返回错误，而不是 panic
3. **超时处理**：长时间运行的工具应支持 context 超时
4. **幂等性**：尽可能保持工具操作的幂等性

### 代码规范

1. **导入分组**：标准库、第三方库、本地包
2. **错误处理**：使用 `fmt.Errorf` 包装错误
3. **日志**：使用结构化日志（未来）
4. **测试**：每个包都应有对应的 `_test.go` 文件

### 调试技巧

1. 使用 `-v` 标志启用详细输出
2. 检查 `~/.config/agentic-coder/` 下的日志和配置
3. 使用 `go test -v` 运行详细测试

## 未来增强

- [ ] MCP (Model Context Protocol) 服务器支持
- [ ] 自定义工具插件系统
- [ ] 多智能体编排
- [ ] 代码审查和重构智能体
- [ ] IDE 集成 (VS Code, JetBrains)
