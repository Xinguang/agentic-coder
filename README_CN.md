# Agentic Coder

一个支持多 AI 提供商和本地 CLI 工具的 AI 编程助手命令行工具。

## 为什么选择 Agentic Coder？

### 无厂商锁定
通过一个参数即可切换 AI 提供商。今天用 Claude，明天用 GPT-4，或者用 Ollama 本地运行 - 你的工作流程保持不变。

### 复用已有的 CLI 工具
已经安装了 Claude Code、Codex 或 Gemini CLI？Agentic Coder 可以直接封装它们 - 无需 API 密钥，直接利用你现有的订阅和登录会话。

### 统一接口
一个工具统一所有。相同的命令、相同的工具、相同的体验，适用于 8+ 种 AI 提供商。无需为不同模型学习不同的接口。

### 隐私优先
使用 Ollama 完全离线运行，或使用独立处理认证的本地 CLI 工具。你的代码永远不必离开你的机器。

### 为开发者而生
- **单一二进制**：使用 Go 编写，编译为单个可执行文件，零依赖
- **跨平台**：支持 macOS、Linux 和 Windows
- **快速启动**：无运行时开销，即时启动
- **可扩展**：轻松添加新的 provider 或工具

### 丰富的工具生态
19 个内置工具满足真实编码需求：文件操作、Shell 命令、网络搜索、Jupyter notebook 等。AI 可以真正帮你写代码，而不只是聊天。

### 成本优化
根据任务复杂度混合使用不同 provider。简单任务用便宜的模型，复杂任务用高级模型。本地 CLI provider 使用你现有的订阅。

## 功能特性

- **多 Provider 支持**：通过 API 或本地 CLI 连接多种 AI 提供商
- **流式响应**：所有 provider 支持实时流式输出
- **工具集成**：内置文件操作、网络搜索、Shell 命令等工具
- **会话管理**：持久化对话历史
- **认证管理**：安全的 API 密钥存储

## 支持的 Provider

### API Provider（需要 API 密钥）

| Provider | 模型 | 环境变量 |
|----------|------|---------|
| Claude (Anthropic) | `sonnet`, `opus`, `haiku`, `claude-*` | `ANTHROPIC_API_KEY` |
| OpenAI | `gpt-4o`, `gpt-4`, `o1-*`, `o3-*`, `o4-*` | `OPENAI_API_KEY` |
| Gemini (Google) | `gemini-2.5-pro`, `gemini-2.5-flash` | `GOOGLE_API_KEY` |
| DeepSeek | `deepseek-*`, `coder`, `reasoner`, `r1` | `DEEPSEEK_API_KEY` |
| Ollama | `llama*`, `qwen*`, `mistral*`, `phi*` | 本地运行，无需密钥 |

### 本地 CLI Provider（使用已安装的 CLI 工具）

| Provider | 模型参数 | 所需 CLI |
|----------|----------|---------|
| Claude Code | `claudecli`, `claude-cli` | [Claude Code](https://docs.anthropic.com/en/docs/claude-code) |
| Codex | `codexcli`, `codex-cli`, `codex` | [Codex CLI](https://github.com/openai/codex) |
| Gemini CLI | `geminicli`, `gemini-cli` | [Gemini CLI](https://github.com/google-gemini/gemini-cli) |

## 安装

### 从源码安装

```bash
git clone https://github.com/xinguang/agentic-coder.git
cd agentic-coder
make build
```

编译后的二进制文件位于 `./bin/agentic-coder`。

### 前置要求

- Go 1.22 或更高版本
- 使用本地 CLI provider 时，需安装对应的 CLI 工具：
  - Claude Code: `npm install -g @anthropic-ai/claude-code`
  - Codex: `npm install -g @openai/codex`
  - Gemini CLI: `npm install -g @anthropic-ai/gemini-cli`

## 使用方法

### 快速开始

```bash
# 使用 Claude API（默认）
export ANTHROPIC_API_KEY="your-api-key"
./bin/agentic-coder

# 使用 OpenAI
export OPENAI_API_KEY="your-api-key"
./bin/agentic-coder -m gpt-4o

# 使用本地 Claude Code CLI
./bin/agentic-coder -m claudecli

# 使用本地 Codex CLI
./bin/agentic-coder -m codex

# 使用本地 Gemini CLI
./bin/agentic-coder -m geminicli

# 使用 Ollama（本地）
./bin/agentic-coder -m llama3.2
```

### 认证管理

保存 API 密钥以便持久使用：

```bash
# 登录到 provider
./bin/agentic-coder auth login claude
./bin/agentic-coder auth login openai
./bin/agentic-coder auth login gemini

# 查看认证状态
./bin/agentic-coder auth status

# 登出
./bin/agentic-coder auth logout claude
```

### 命令行选项

```
用法:
  agentic-coder [flags]
  agentic-coder [command]

可用命令:
  auth        管理认证
  config      管理配置
  help        查看帮助
  version     打印版本信息

选项:
  -h, --help           帮助信息
  -k, --api-key string API 密钥（覆盖已保存的凭证）
  -m, --model string   使用的模型（默认 "sonnet"）
  -v, --verbose        启用详细输出
```

### 交互式命令

进入聊天界面后：

| 命令 | 描述 |
|------|------|
| `/help` | 显示可用命令 |
| `/clear` | 清屏 |
| `/session` | 显示当前会话信息 |
| `/model [name]` | 显示或切换模型 |
| `/exit` | 退出程序 |

### 快捷键

- `Ctrl+C` - 中断当前操作
- `Ctrl+C`（两次）- 退出程序
- `Ctrl+D` - 退出程序

## 项目结构

```
agentic-coder/
├── cmd/
│   └── agentic-coder/    # 主程序
├── pkg/
│   ├── auth/             # 认证管理
│   ├── engine/           # 核心 AI 引擎
│   ├── provider/         # AI provider 实现
│   │   ├── claude/       # Claude API provider
│   │   ├── claudecli/    # 本地 Claude Code CLI provider
│   │   ├── codexcli/     # 本地 Codex CLI provider
│   │   ├── deepseek/     # DeepSeek API provider
│   │   ├── gemini/       # Gemini API provider
│   │   ├── geminicli/    # 本地 Gemini CLI provider
│   │   ├── ollama/       # Ollama provider
│   │   └── openai/       # OpenAI API provider
│   ├── session/          # 会话管理
│   ├── tool/             # 工具实现
│   │   └── builtin/      # 内置工具
│   └── ...
├── devdocs/              # 开发文档
│   ├── DESIGN.md         # 设计文档（英文）
│   └── DESIGN_CN.md      # 设计文档（中文）
├── Makefile
└── README.md
```

## 内置工具

助手可以使用以下工具：

- **文件操作**：Read、Write、Edit、Glob、Grep
- **Shell**：Bash 命令执行
- **网络**：WebSearch、WebFetch
- **Notebook**：Jupyter notebook 编辑
- **规划**：EnterPlanMode、ExitPlanMode

## 配置

配置文件存储在 `~/.config/agentic-coder/`：

- `credentials.json` - 保存的 API 密钥和认证数据

## 环境变量

| 变量 | 描述 |
|------|------|
| `ANTHROPIC_API_KEY` | Anthropic/Claude API 密钥 |
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `GOOGLE_API_KEY` | Google/Gemini API 密钥 |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `OLLAMA_HOST` | Ollama 服务器 URL（默认：`http://localhost:11434`）|

## 开发

```bash
# 构建
make build

# 运行测试
make test

# 详细模式运行
./bin/agentic-coder -v
```

详细的开发指南请参阅 [devdocs/DESIGN_CN.md](devdocs/DESIGN_CN.md)。

## 许可证

MIT License
