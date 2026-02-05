// Package tui provides a simple terminal UI without bubbletea
package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// SimpleRunner runs without bubbletea for cleaner output
type SimpleRunner struct {
	engine   *engine.Engine
	config   Config
	verbose  bool

	// Token tracking
	inputTokens  int
	outputTokens int
	totalCost    float64

	// Cancellation
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// NewSimpleRunner creates a simple TUI runner
func NewSimpleRunner(eng *engine.Engine, cfg Config) *SimpleRunner {
	return &SimpleRunner{
		engine: eng,
		config: cfg,
	}
}

// Run starts the simple TUI
func (r *SimpleRunner) Run() error {
	r.printWelcome()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		os.Stdout.Sync() // Flush to ensure prompt is visible

		input, err := reader.ReadString('\n')
		if err != nil {
			// EOF means stdin closed (Ctrl+D)
			return nil
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if r.handleCommand(input) {
				continue
			}
		}

		// Handle bash
		if strings.HasPrefix(input, "!") {
			cmd := strings.TrimPrefix(input, "!")
			cmd = strings.TrimSpace(cmd)
			fmt.Fprintf(os.Stdout, "%s$ %s%s\n", ansiDim, cmd, ansiReset)
			// TODO: execute bash
			continue
		}

		// Regular message
		fmt.Println()
		r.runEngine(input)
		fmt.Print("\n\n") // Ensure newline after response

		// Save session after each message
		if r.config.OnSaveSession != nil {
			r.config.OnSaveSession()
		}
	}
}

func (r *SimpleRunner) printWelcome() {
	info := r.config.Model
	if r.config.CWD != "" {
		info += " • " + shortenPath(r.config.CWD, 40)
	}
	if r.config.SessionID != "" {
		info += " • " + r.config.SessionID
		if r.config.MessageCount > 0 {
			info += fmt.Sprintf(" (%d msgs)", r.config.MessageCount)
		}
	}

	fmt.Fprintf(os.Stdout, "\n%sAgentic Coder v%s%s\n", ansiCyan, r.config.Version, ansiReset)
	fmt.Fprintf(os.Stdout, "%s%s%s\n\n", ansiDim, info, ansiReset)
	fmt.Fprintf(os.Stdout, "%sType your message or /help for commands%s\n\n", ansiDim, ansiReset)
}

func (r *SimpleRunner) runEngine(input string) {
	// Recover from panics
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stdout, "\n%sRecovered from panic: %v%s\n", ansiRed, err, ansiReset)
		}
	}()

	r.mu.Lock()
	r.ctx, r.cancel = context.WithCancel(context.Background())
	ctx := r.ctx
	r.mu.Unlock()

	// Set up callbacks
	r.engine.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			fmt.Print(text)
		},
		OnThinking: func(text string) {
			// Skip thinking in non-verbose mode
		},
		OnToolUse: func(name string, params map[string]interface{}) {
			r.printToolUse(name, params)
		},
		OnToolResult: func(name string, result *tool.Output) {
			success := !result.IsError
			summary := ""
			if result.IsError {
				summary = result.Content
			} else {
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
			}
			r.printToolResult(name, success, summary)
		},
		OnUsage: func(inputTokens, outputTokens int) {
			r.inputTokens += inputTokens
			r.outputTokens += outputTokens
			r.totalCost += float64(inputTokens)*0.000003 + float64(outputTokens)*0.000015
		},
		OnError: func(err error) {
			fmt.Fprintf(os.Stdout, "\n%sError: %v%s\n", ansiRed, err, ansiReset)
		},
	})

	// Run
	if err := r.engine.Run(ctx, input); err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stdout, "%sError: %v%s\n", ansiRed, err, ansiReset)
		}
	}
}

func (r *SimpleRunner) printToolUse(name string, params map[string]interface{}) {
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

	fmt.Fprintf(os.Stdout, "\n%s %s%s%s\n", icon, ansiYellow, name, ansiReset)

	switch name {
	case "Edit":
		if fp, ok := params["file_path"].(string); ok {
			fmt.Fprintf(os.Stdout, "%s   %s%s\n", ansiDim, fp, ansiReset)
		}
	case "Write", "Read":
		if fp, ok := params["file_path"].(string); ok {
			fmt.Fprintf(os.Stdout, "%s   %s%s\n", ansiDim, fp, ansiReset)
		}
	case "Bash":
		if cmd, ok := params["command"].(string); ok {
			fmt.Fprintf(os.Stdout, "%s   $ %s%s\n", ansiDim, truncate(cmd, 80), ansiReset)
		}
	case "Grep", "Glob":
		if pattern, ok := params["pattern"].(string); ok {
			fmt.Fprintf(os.Stdout, "%s   pattern: %s%s\n", ansiDim, pattern, ansiReset)
		}
	}
}

func (r *SimpleRunner) printToolResult(name string, success bool, summary string) {
	if success {
		if summary != "" {
			fmt.Fprintf(os.Stdout, "%s   ✓ %s%s\n", ansiGreen, truncate(summary, 60), ansiReset)
		} else {
			fmt.Fprintf(os.Stdout, "%s   ✓%s\n", ansiGreen, ansiReset)
		}
	} else {
		fmt.Fprintf(os.Stdout, "%s   ✗ %s%s\n", ansiRed, truncate(summary, 60), ansiReset)
	}
}

func (r *SimpleRunner) handleCommand(input string) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return true
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/h", "/?":
		fmt.Print(r.helpText())

	case "/clear":
		fmt.Print("\033[H\033[2J")

	case "/exit", "/quit", "/q":
		fmt.Fprintf(os.Stdout, "%sGoodbye!%s\n", ansiDim, ansiReset)
		os.Exit(0)

	case "/model":
		if len(parts) > 1 {
			r.config.Model = parts[1]
			fmt.Fprintf(os.Stdout, "%sModel: %s%s\n", ansiGreen, r.config.Model, ansiReset)
		} else {
			fmt.Fprintf(os.Stdout, "Current model: %s\n", r.config.Model)
		}

	case "/cost":
		fmt.Fprintf(os.Stdout, "Input tokens:  %d\n", r.inputTokens)
		fmt.Fprintf(os.Stdout, "Output tokens: %d\n", r.outputTokens)
		fmt.Fprintf(os.Stdout, "Total cost:    $%.4f\n", r.totalCost)

	case "/verbose":
		r.verbose = !r.verbose
		if r.verbose {
			fmt.Fprintf(os.Stdout, "%sVerbose mode enabled%s\n", ansiGreen, ansiReset)
		} else {
			fmt.Fprintf(os.Stdout, "%sVerbose mode disabled%s\n", ansiDim, ansiReset)
		}

	case "/history", "/sessions":
		r.listSessions()

	case "/resume":
		if len(parts) > 1 {
			r.resumeSession(parts[1])
		} else {
			r.listSessions()
		}

	case "/new":
		r.newSession()

	case "/config", "/settings":
		fmt.Fprintf(os.Stdout, "%sSettings:%s\n", ansiDim, ansiReset)
		fmt.Fprintf(os.Stdout, "  Model: %s\n", r.config.Model)
		fmt.Fprintf(os.Stdout, "  Verbose: %v\n", r.verbose)
		fmt.Fprintf(os.Stdout, "  CWD: %s\n", r.config.CWD)

	default:
		fmt.Fprintf(os.Stdout, "%sUnknown command: %s%s\n", ansiRed, cmd, ansiReset)
		fmt.Fprintf(os.Stdout, "%sType /help for available commands%s\n", ansiDim, ansiReset)
	}

	return true
}

func (r *SimpleRunner) listSessions() {
	if r.config.OnListSessions == nil {
		fmt.Fprintf(os.Stdout, "%sSession management not available%s\n", ansiRed, ansiReset)
		return
	}
	sessions := r.config.OnListSessions()
	if len(sessions) == 0 {
		fmt.Fprintf(os.Stdout, "%sNo sessions found%s\n", ansiDim, ansiReset)
		return
	}

	fmt.Fprintf(os.Stdout, "\n%s📋 Sessions%s\n", ansiDim, ansiReset)
	fmt.Fprintf(os.Stdout, "%s───────────────────────────────────────%s\n", ansiDim, ansiReset)
	for i, s := range sessions {
		marker := "  "
		if s.IsCurrent {
			marker = fmt.Sprintf("%s▸ %s", ansiCyan, ansiReset)
		}
		title := s.Summary
		if title == "" {
			title = "(untitled)"
		}
		displayID := s.ShortID
		if displayID == "" {
			displayID = s.ID
		}
		fmt.Fprintf(os.Stdout, "%s%d. %s%s %s", marker, i+1, ansiDim, displayID, ansiReset)
		fmt.Fprintf(os.Stdout, " %s", title)
		fmt.Fprintf(os.Stdout, "%s (%d msgs, %s)%s\n", ansiDim, s.MessageCount, s.UpdatedAt, ansiReset)
	}
	fmt.Fprintf(os.Stdout, "%s───────────────────────────────────────%s\n", ansiDim, ansiReset)
	fmt.Fprintf(os.Stdout, "%sUse /resume <number> to switch%s\n\n", ansiDim, ansiReset)
}

func (r *SimpleRunner) resumeSession(idOrNum string) {
	if r.config.OnResumeSession == nil {
		fmt.Fprintf(os.Stdout, "%sSession management not available%s\n", ansiRed, ansiReset)
		return
	}

	sessionID := idOrNum
	if r.config.OnListSessions != nil {
		var num int
		if _, err := fmt.Sscanf(idOrNum, "%d", &num); err == nil {
			sessions := r.config.OnListSessions()
			if num > 0 && num <= len(sessions) {
				sessionID = sessions[num-1].ID
			}
		}
	}

	msgCount, err := r.config.OnResumeSession(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%sFailed to resume: %v%s\n", ansiRed, err, ansiReset)
		return
	}

	r.config.SessionID = sessionID
	if len(sessionID) > 8 {
		r.config.SessionID = sessionID[:8]
	}
	r.config.MessageCount = msgCount
	fmt.Fprintf(os.Stdout, "%sResumed session: %s (%d messages)%s\n", ansiGreen, r.config.SessionID, msgCount, ansiReset)
}

func (r *SimpleRunner) newSession() {
	if r.config.OnNewSession == nil {
		fmt.Fprintf(os.Stdout, "%sSession management not available%s\n", ansiRed, ansiReset)
		return
	}

	sessionID, err := r.config.OnNewSession()
	if err != nil {
		fmt.Fprintf(os.Stdout, "%sFailed to create session: %v%s\n", ansiRed, err, ansiReset)
		return
	}

	r.config.SessionID = sessionID
	if len(sessionID) > 8 {
		r.config.SessionID = sessionID[:8]
	}
	r.config.MessageCount = 0
	r.inputTokens = 0
	r.outputTokens = 0
	r.totalCost = 0
	fmt.Fprintf(os.Stdout, "%sNew session: %s%s\n", ansiGreen, r.config.SessionID, ansiReset)
}

func (r *SimpleRunner) helpText() string {
	return fmt.Sprintf(`
%sCommands%s
  /help          Show this help
  /clear         Clear screen
  /exit          Exit
  /model [name]  Show or change model
  /cost          Show token usage and cost
  /verbose       Toggle verbose mode

%sSessions%s
  /history       List sessions
  /resume <n>    Resume session by number or ID
  /new           Start new session

%sShortcuts%s
  Ctrl+C         Exit
  Ctrl+D         Exit

%sInput Modes%s
  /command       Run a command
  !shell cmd     Run shell command directly

`, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset)
}
