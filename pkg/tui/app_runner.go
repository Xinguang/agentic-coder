// Package tui provides a terminal UI using bubbletea
package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// AppRunner wraps the bubbletea app with engine integration
type AppRunner struct {
	engine  *engine.Engine
	config  Config
	model   *AppModel
	program *tea.Program

	// State
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex

	// Token tracking
	inputTokens  int
	outputTokens int

	// Tool tracking
	toolCount    int
	currentTool  string
}

// NewAppRunner creates a new app runner
func NewAppRunner(eng *engine.Engine, cfg Config) *AppRunner {
	model := NewAppModel()
	r := &AppRunner{
		engine: eng,
		config: cfg,
		model:  model,
	}

	// Set callbacks
	model.SetCallbacks(r.handleSubmit, r.handleCancel)

	return r
}

// Run starts the TUI
func (r *AppRunner) Run() error {
	// Add welcome message
	r.model.AppendContent(fmt.Sprintf("\n%sAgentic Coder v%s%s\n", ansiCyan, r.config.Version, ansiReset))
	r.model.AppendContent(fmt.Sprintf("%s%s • %s%s\n\n", ansiDim, r.config.Model, r.config.CWD, ansiReset))

	r.program = tea.NewProgram(r.model, tea.WithAltScreen())
	_, err := r.program.Run()
	return err
}

func (r *AppRunner) handleSubmit(input string) {
	// Handle commands
	if strings.HasPrefix(input, "/") {
		r.handleCommand(input)
		return
	}

	// Run engine in goroutine
	go r.runEngine(input)
}

func (r *AppRunner) handleCancel() {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
}

func (r *AppRunner) runEngine(input string) {
	r.mu.Lock()
	r.ctx, r.cancel = context.WithCancel(context.Background())
	ctx := r.ctx
	r.mu.Unlock()

	// Show user input
	r.program.Send(contentMsg{content: fmt.Sprintf("\n%s> %s%s\n\n", ansiCyan, input, ansiReset)})
	r.program.Send(statusMsg{text: "Thinking", isWorking: true})

	// Reset tool counter
	r.toolCount = 0
	r.currentTool = ""

	var responseBuffer strings.Builder

	// Set up callbacks
	r.engine.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			responseBuffer.WriteString(text)
			r.program.Send(contentMsg{content: text})
			r.program.Send(statusMsg{text: "Responding", isWorking: true})
		},
		OnThinking: func(text string) {
			r.program.Send(statusMsg{text: "Thinking", isWorking: true})
			r.program.Send(contentMsg{content: fmt.Sprintf("%s💭 %s%s", ansiDim, text, ansiReset)})
		},
		OnToolUse: func(name string, params map[string]interface{}) {
			r.toolCount++
			r.currentTool = name
			r.program.Send(statusMsg{text: fmt.Sprintf("Tool #%d: %s", r.toolCount, name), isWorking: true})
			r.program.Send(contentMsg{content: r.formatToolUse(name, params)})
		},
		OnToolResult: func(name string, result *tool.Output) {
			status := "✓"
			if result.IsError {
				status = "✗"
			}
			r.program.Send(statusMsg{text: fmt.Sprintf("Tool #%d: %s %s", r.toolCount, name, status), isWorking: true})
			r.program.Send(contentMsg{content: r.formatToolResult(name, result)})
		},
		OnUsage: func(inputTokens, outputTokens int) {
			r.inputTokens += inputTokens
			r.outputTokens += outputTokens
			r.model.SetTokenCount(r.inputTokens + r.outputTokens)
		},
		OnError: func(err error) {
			r.program.Send(contentMsg{content: err.Error(), isError: true})
		},
	})

	// Run
	if err := r.engine.Run(ctx, input); err != nil {
		if ctx.Err() == nil {
			r.program.Send(contentMsg{content: err.Error(), isError: true})
		}
	}

	r.program.Send(contentMsg{content: "\n"})
	r.program.Send(doneMsg{})

	// Save session
	if r.config.OnSaveSession != nil {
		r.config.OnSaveSession()
	}
}

func (r *AppRunner) formatToolUse(name string, params map[string]interface{}) string {
	var sb strings.Builder

	// Tool header with icon and action description
	icon := "⚡"
	action := name
	switch name {
	case "Read":
		icon = "📖"
		action = "Reading file"
	case "Write":
		icon = "📝"
		action = "Writing file"
	case "Edit":
		icon = "✏️"
		action = "Editing file"
	case "Bash":
		icon = "💻"
		action = "Running command"
	case "Grep":
		icon = "🔍"
		action = "Searching content"
	case "Glob":
		icon = "📂"
		action = "Finding files"
	case "Task":
		icon = "🤖"
		action = "Running task"
	case "WebFetch":
		icon = "🌐"
		action = "Fetching URL"
	case "WebSearch":
		icon = "🔎"
		action = "Searching web"
	}

	sb.WriteString(fmt.Sprintf("\n%s─────────────────────────────────────%s\n", ansiDim, ansiReset))
	sb.WriteString(fmt.Sprintf("%s %s%s%s %s⏳%s\n", icon, ansiYellow, action, ansiReset, ansiDim, ansiReset))

	// Tool-specific details
	switch name {
	case "Read":
		if fp, ok := params["file_path"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sFile:%s %s\n", ansiDim, ansiReset, fp))
		}
		if offset, ok := params["offset"].(float64); ok {
			sb.WriteString(fmt.Sprintf("   %sFrom line:%s %.0f\n", ansiDim, ansiReset, offset))
		}
		if limit, ok := params["limit"].(float64); ok {
			sb.WriteString(fmt.Sprintf("   %sLines:%s %.0f\n", ansiDim, ansiReset, limit))
		}

	case "Write":
		if fp, ok := params["file_path"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sFile:%s %s\n", ansiDim, ansiReset, fp))
		}
		if content, ok := params["content"].(string); ok {
			lines := strings.Count(content, "\n") + 1
			sb.WriteString(fmt.Sprintf("   %sContent:%s %d lines\n", ansiDim, ansiReset, lines))
		}

	case "Edit":
		if fp, ok := params["file_path"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sFile:%s %s\n", ansiDim, ansiReset, fp))
		}
		if old, ok := params["old_string"].(string); ok {
			preview := strings.Split(old, "\n")[0]
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %sReplace:%s %s\n", ansiDim, ansiReset, preview))
		}

	case "Bash":
		if cmd, ok := params["command"].(string); ok {
			// Show full command, split if too long
			if len(cmd) > 80 {
				sb.WriteString(fmt.Sprintf("   %s$ %s%s\n", ansiDim, cmd[:80], ansiReset))
				sb.WriteString(fmt.Sprintf("   %s  %s...%s\n", ansiDim, cmd[80:min(160, len(cmd))], ansiReset))
			} else {
				sb.WriteString(fmt.Sprintf("   %s$ %s%s\n", ansiDim, cmd, ansiReset))
			}
		}
		if desc, ok := params["description"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sDesc:%s %s\n", ansiDim, ansiReset, desc))
		}

	case "Grep":
		if pattern, ok := params["pattern"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sPattern:%s %s\n", ansiDim, ansiReset, pattern))
		}
		if path, ok := params["path"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sPath:%s %s\n", ansiDim, ansiReset, path))
		}

	case "Glob":
		if pattern, ok := params["pattern"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sPattern:%s %s\n", ansiDim, ansiReset, pattern))
		}
		if path, ok := params["path"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sPath:%s %s\n", ansiDim, ansiReset, path))
		}

	case "Task":
		if desc, ok := params["description"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sTask:%s %s\n", ansiDim, ansiReset, desc))
		}
		if agentType, ok := params["subagent_type"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sAgent:%s %s\n", ansiDim, ansiReset, agentType))
		}

	case "WebFetch":
		if url, ok := params["url"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sURL:%s %s\n", ansiDim, ansiReset, url))
		}

	case "WebSearch":
		if query, ok := params["query"].(string); ok {
			sb.WriteString(fmt.Sprintf("   %sQuery:%s %s\n", ansiDim, ansiReset, query))
		}
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *AppRunner) formatToolResult(name string, result *tool.Output) string {
	var sb strings.Builder

	if result.IsError {
		sb.WriteString(fmt.Sprintf("   %s✗ Error%s\n", ansiRed, ansiReset))
		// Show error details
		errLines := strings.Split(result.Content, "\n")
		for i, line := range errLines {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("   %s... (%d more lines)%s\n", ansiDim, len(errLines)-5, ansiReset))
				break
			}
			if line != "" {
				sb.WriteString(fmt.Sprintf("   %s%s%s\n", ansiRed, line, ansiReset))
			}
		}
		sb.WriteString(fmt.Sprintf("%s─────────────────────────────────────%s\n", ansiDim, ansiReset))
		return sb.String()
	}

	// Success - show result summary based on tool type
	content := result.Content
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	sb.WriteString(fmt.Sprintf("   %s✓ Done%s", ansiGreen, ansiReset))

	switch name {
	case "Read":
		sb.WriteString(fmt.Sprintf(" %s(%d lines)%s\n", ansiDim, lineCount, ansiReset))

	case "Write":
		sb.WriteString(fmt.Sprintf(" %s(file written)%s\n", ansiDim, ansiReset))

	case "Edit":
		sb.WriteString(fmt.Sprintf(" %s(file updated)%s\n", ansiDim, ansiReset))

	case "Bash":
		if lineCount > 1 {
			sb.WriteString(fmt.Sprintf(" %s(%d lines output)%s\n", ansiDim, lineCount, ansiReset))
			// Show first few lines of output
			for i, line := range lines {
				if i >= 3 {
					sb.WriteString(fmt.Sprintf("   %s... (%d more lines)%s\n", ansiDim, lineCount-3, ansiReset))
					break
				}
				if line != "" && len(line) < 100 {
					sb.WriteString(fmt.Sprintf("   %s%s%s\n", ansiDim, line, ansiReset))
				}
			}
		} else if content != "" {
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			sb.WriteString(fmt.Sprintf(" %s%s%s\n", ansiDim, content, ansiReset))
		} else {
			sb.WriteString("\n")
		}

	case "Grep":
		// Count matches
		if lineCount > 0 && content != "" {
			sb.WriteString(fmt.Sprintf(" %s(%d matches)%s\n", ansiDim, lineCount, ansiReset))
		} else {
			sb.WriteString(fmt.Sprintf(" %s(no matches)%s\n", ansiDim, ansiReset))
		}

	case "Glob":
		// Count files found
		if lineCount > 0 && content != "" {
			sb.WriteString(fmt.Sprintf(" %s(%d files)%s\n", ansiDim, lineCount, ansiReset))
		} else {
			sb.WriteString(fmt.Sprintf(" %s(no files)%s\n", ansiDim, ansiReset))
		}

	case "Task":
		sb.WriteString(fmt.Sprintf(" %s(completed)%s\n", ansiDim, ansiReset))

	default:
		if content != "" {
			if len(content) > 60 {
				sb.WriteString(fmt.Sprintf(" %s%s...%s\n", ansiDim, content[:60], ansiReset))
			} else {
				sb.WriteString(fmt.Sprintf(" %s%s%s\n", ansiDim, content, ansiReset))
			}
		} else {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(fmt.Sprintf("%s─────────────────────────────────────%s\n", ansiDim, ansiReset))
	return sb.String()
}

func (r *AppRunner) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/h", "/?":
		r.program.Send(contentMsg{content: r.helpText()})

	case "/clear":
		r.model.content.Reset()
		r.model.viewport.SetContent("")

	case "/exit", "/quit", "/q":
		r.program.Quit()

	case "/cost":
		cost := float64(r.inputTokens)*0.000003 + float64(r.outputTokens)*0.000015
		r.program.Send(contentMsg{content: fmt.Sprintf(
			"\nInput tokens:  %d\nOutput tokens: %d\nTotal cost:    $%.4f\n\n",
			r.inputTokens, r.outputTokens, cost,
		)})

	default:
		r.program.Send(contentMsg{content: fmt.Sprintf(
			"%sUnknown command: %s%s\nType /help for available commands\n\n",
			ansiRed, cmd, ansiReset,
		)})
	}
}

func (r *AppRunner) helpText() string {
	return fmt.Sprintf(`
%sCommands%s
  /help          Show this help
  /clear         Clear screen
  /exit          Exit
  /cost          Show token usage and cost

%sShortcuts%s
  Ctrl+C         Cancel current operation / Exit
  Esc            Cancel current operation

`, ansiCyan, ansiReset, ansiCyan, ansiReset)
}
