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

	var responseBuffer strings.Builder

	// Set up callbacks
	r.engine.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			responseBuffer.WriteString(text)
			r.program.Send(contentMsg{content: text})
		},
		OnThinking: func(text string) {
			r.program.Send(statusMsg{text: "Thinking", isWorking: true})
			r.program.Send(contentMsg{content: fmt.Sprintf("%s💭 %s%s", ansiDim, text, ansiReset)})
		},
		OnToolUse: func(name string, params map[string]interface{}) {
			r.program.Send(statusMsg{text: fmt.Sprintf("Running %s", name), isWorking: true})
			r.program.Send(contentMsg{content: r.formatToolUse(name, params)})
		},
		OnToolResult: func(name string, result *tool.Output) {
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
	icon := "⚡"
	switch name {
	case "Read":
		icon = "📖"
	case "Write":
		icon = "📝"
	case "Edit":
		icon = "✏️"
	case "Bash":
		icon = "💻"
	case "Grep", "Glob":
		icon = "🔍"
	case "Task":
		icon = "🤖"
	}

	var detail string
	switch name {
	case "Edit", "Write", "Read":
		if fp, ok := params["file_path"].(string); ok {
			detail = fmt.Sprintf("   %s", fp)
		}
	case "Bash":
		if cmd, ok := params["command"].(string); ok {
			if len(cmd) > 60 {
				cmd = cmd[:60] + "..."
			}
			detail = fmt.Sprintf("   $ %s", cmd)
		}
	case "Grep", "Glob":
		if pattern, ok := params["pattern"].(string); ok {
			detail = fmt.Sprintf("   pattern: %s", pattern)
		}
	}

	result := fmt.Sprintf("\n%s %s%s%s\n", icon, ansiYellow, name, ansiReset)
	if detail != "" {
		result += fmt.Sprintf("%s%s%s\n", ansiDim, detail, ansiReset)
	}
	return result
}

func (r *AppRunner) formatToolResult(name string, result *tool.Output) string {
	if result.IsError {
		return fmt.Sprintf("%s   ✗ %s%s\n", ansiRed, truncate(result.Content, 60), ansiReset)
	}

	summary := ""
	content := result.Content
	lines := strings.Split(content, "\n")
	if len(lines) > 5 {
		summary = fmt.Sprintf("%d lines", len(lines))
	} else if len(content) > 100 {
		summary = content[:100] + "..."
	} else if content != "" {
		summary = strings.ReplaceAll(content, "\n", " ")
		if len(summary) > 60 {
			summary = summary[:60] + "..."
		}
	}

	if summary != "" {
		return fmt.Sprintf("%s   ✓ %s%s\n", ansiGreen, summary, ansiReset)
	}
	return fmt.Sprintf("%s   ✓%s\n", ansiGreen, ansiReset)
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
