// Package tui provides a simple terminal UI without bubbletea
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/peterh/liner"
	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/review"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// Markdown renderer
var mdRenderer *glamour.TermRenderer

func init() {
	var err error
	mdRenderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		mdRenderer = nil
	}
}

// SimpleRunner runs without bubbletea for cleaner output
type SimpleRunner struct {
	engine        *engine.Engine
	config        Config
	verbose       bool
	reviewer      *review.Reviewer
	reviewHistory *review.ReviewHistory

	// Incremental and pipeline review
	incrementalReviewer *review.IncrementalReviewer
	pipeline            *review.Pipeline
	lastResponse        string // For incremental review comparison

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
	// Set default max review cycles
	if cfg.MaxReviewCycles == 0 {
		cfg.MaxReviewCycles = 5
	}
	return &SimpleRunner{
		engine: eng,
		config: cfg,
	}
}

// SetReviewer sets the reviewer for automatic review
func (r *SimpleRunner) SetReviewer(prov provider.AIProvider) {
	r.reviewer = review.NewReviewer(prov)
}

// SetReviewerWithConfig sets the reviewer with custom config
func (r *SimpleRunner) SetReviewerWithConfig(prov provider.AIProvider, cfg *review.ReviewConfig) {
	r.reviewer = review.NewReviewerWithConfig(prov, cfg)
}

// SetReviewHistory sets the review history recorder
func (r *SimpleRunner) SetReviewHistory(history *review.ReviewHistory) {
	r.reviewHistory = history
}

// SetIncrementalReview enables incremental review mode
func (r *SimpleRunner) SetIncrementalReview(enabled bool) {
	if enabled && r.reviewer != nil {
		r.incrementalReviewer = review.NewIncrementalReviewer(r.reviewer)
	} else {
		r.incrementalReviewer = nil
	}
}

// SetReviewPipeline sets a custom review pipeline
func (r *SimpleRunner) SetReviewPipeline(pipeline *review.Pipeline) {
	r.pipeline = pipeline
}

// Run starts the simple TUI
func (r *SimpleRunner) Run() error {
	r.printWelcome()

	// Create liner instance with full editing support
	line := liner.NewLiner()
	defer line.Close()

	// Enable multiline mode and ctrl+c handling
	line.SetCtrlCAborts(true)
	line.SetMultiLineMode(true)

	// Load history
	historyFile := filepath.Join(os.TempDir(), "agentic-coder-history")
	if f, err := os.Open(historyFile); err == nil {
		line.ReadHistory(f)
		f.Close()
	}

	// Save history on exit
	defer func() {
		if f, err := os.Create(historyFile); err == nil {
			line.WriteHistory(f)
			f.Close()
		}
	}()

	for {
		input, err := line.Prompt("> ")
		if err != nil {
			if err == liner.ErrPromptAborted {
				continue // Ctrl+C, just show new prompt
			}
			if err == io.EOF {
				fmt.Fprintf(os.Stdout, "\n%sGoodbye!%s\n", ansiDim, ansiReset)
				return nil // Ctrl+D, exit gracefully
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Add to history
		line.AppendHistory(input)

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
		response := r.runEngine(input)

		// Auto-review if enabled
		if r.config.EnableReview && r.reviewer != nil && response != "" {
			r.runReviewCycle(input, response)
		}

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

func (r *SimpleRunner) runEngine(input string) string {
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

	// Full response for review and markdown rendering
	var fullResponse strings.Builder
	var textBuffer strings.Builder // Buffer for markdown text only

	// Set up callbacks
	r.engine.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			// Accumulate text for markdown rendering
			textBuffer.WriteString(text)
			fullResponse.WriteString(text)
		},
		OnThinking: func(text string) {
			// Flush any pending text before showing thinking
			r.flushMarkdown(&textBuffer)
			// Show thinking in dim color
			fmt.Fprintf(os.Stdout, "%s%s%s", ansiDim, text, ansiReset)
		},
		OnToolUse: func(name string, params map[string]interface{}) {
			// Flush any pending text before showing tool use
			r.flushMarkdown(&textBuffer)
			r.printToolUse(name, params)
			// Record tool use in full response
			fullResponse.WriteString(fmt.Sprintf("\n[Tool: %s]\n", name))
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
			// Record tool result in full response
			if result.IsError {
				fullResponse.WriteString(fmt.Sprintf("[Tool Error: %s]\n", summary))
			} else {
				fullResponse.WriteString(fmt.Sprintf("[Tool Success: %s]\n", summary))
			}
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

	// Flush any remaining text
	r.flushMarkdown(&textBuffer)

	// Add newline after response
	fmt.Println()

	return fullResponse.String()
}

// flushMarkdown renders and prints accumulated markdown text
func (r *SimpleRunner) flushMarkdown(buf *strings.Builder) {
	text := buf.String()
	if text == "" {
		return
	}
	buf.Reset()

	// Render with glamour if available
	if mdRenderer != nil {
		rendered, err := mdRenderer.Render(text)
		if err == nil {
			// Remove extra newlines from glamour output
			rendered = strings.TrimSpace(rendered)
			fmt.Println(rendered)
			return
		}
	}

	// Fallback to raw text
	fmt.Print(text)
}


// runReviewCycle runs the automatic review and correction cycle
func (r *SimpleRunner) runReviewCycle(originalRequest, response string) {
	maxCycles := r.config.MaxReviewCycles
	if maxCycles <= 0 {
		maxCycles = 5
	}

	currentResponse := response
	totalReviewTokens := 0

	// Check if incremental review is applicable
	if r.incrementalReviewer != nil && r.lastResponse != "" {
		incResult, err := r.incrementalReviewer.ReviewChanges(r.lastResponse, currentResponse)
		if err == nil && !incResult.NeedsReview() {
			fmt.Fprintf(os.Stdout, "\n%s✓ No significant changes detected, skipping review%s\n\n", ansiGreen, ansiReset)
			r.lastResponse = currentResponse
			return
		}
		if err == nil && incResult.ChangedBlocks < incResult.TotalBlocks {
			fmt.Fprintf(os.Stdout, "\n%s📊 Incremental review: %d/%d blocks changed%s\n",
				ansiDim, incResult.ChangedBlocks, incResult.TotalBlocks, ansiReset)
		}
	}

	for cycle := 1; cycle <= maxCycles; cycle++ {
		fmt.Fprintf(os.Stdout, "\n%s🔍 Reviewing response (cycle %d/%d)...%s\n", ansiDim, cycle, maxCycles, ansiReset)

		// Run pipeline if available, otherwise use standard reviewer
		var result *review.ReviewResult
		var err error
		startTime := timeNow()
		ctx := context.Background()

		if r.pipeline != nil {
			// Use pipeline-based review
			pipelineResult, pErr := r.pipeline.Run(ctx, currentResponse)
			if pErr != nil {
				err = pErr
			} else {
				// Convert pipeline result to review result
				result = &review.ReviewResult{
					Passed:   pipelineResult.Passed,
					Issues:   pipelineResult.Summary,
					Feedback: r.extractSuggestions(pipelineResult),
				}
			}
		} else {
			// Standard review
			result, err = r.reviewer.Review(ctx, originalRequest, currentResponse)
		}
		durationMs := timeNow().Sub(startTime).Milliseconds()

		if err != nil {
			fmt.Fprintf(os.Stdout, "%sReview error: %v%s\n", ansiRed, err, ansiReset)
			return
		}

		// Record to history if available
		if r.reviewHistory != nil {
			r.reviewHistory.Record(r.config.SessionID, cycle, result, durationMs)
		}

		// Track review token usage
		reviewTokens := result.InputTokens + result.OutputTokens
		totalReviewTokens += reviewTokens
		r.inputTokens += result.InputTokens
		r.outputTokens += result.OutputTokens

		if result.Passed {
			fmt.Fprintf(os.Stdout, "%s✓ Review passed%s", ansiGreen, ansiReset)
			fmt.Fprintf(os.Stdout, " %s(review tokens: %d)%s\n\n", ansiDim, totalReviewTokens, ansiReset)

			// Cache for incremental review
			if r.incrementalReviewer != nil {
				for _, block := range review.ExtractCodeBlocks(currentResponse) {
					r.incrementalReviewer.CacheResult(block, true, "", "")
				}
			}
			r.lastResponse = currentResponse
			return
		}

		// Show issues found
		fmt.Fprintf(os.Stdout, "%s⚠ Issues found:%s\n", ansiYellow, ansiReset)
		if result.Issues != "" {
			fmt.Fprintf(os.Stdout, "%s%s%s\n", ansiDim, result.Issues, ansiReset)
		}

		// Check if this is the last cycle
		if cycle == maxCycles {
			fmt.Fprintf(os.Stdout, "\n%s❌ Max review cycles reached. Suggestions:%s\n", ansiRed, ansiReset)
			fmt.Fprintf(os.Stdout, "%s%s%s\n", ansiDim, result.Feedback, ansiReset)
			fmt.Fprintf(os.Stdout, "%sPlease refine your request or manually address the issues above.%s\n", ansiDim, ansiReset)
			fmt.Fprintf(os.Stdout, "%s(total review tokens: %d)%s\n\n", ansiDim, totalReviewTokens, ansiReset)
			r.lastResponse = currentResponse
			return
		}

		// Generate correction prompt and run engine again
		correctionPrompt := r.reviewer.GenerateCorrectionPrompt(result.Issues, result.Feedback)
		fmt.Fprintf(os.Stdout, "\n%s🔄 Auto-correcting...%s\n\n", ansiCyan, ansiReset)

		currentResponse = r.runEngine(correctionPrompt)
	}
}

// extractSuggestions extracts suggestions from pipeline result
func (r *SimpleRunner) extractSuggestions(result *review.PipelineResult) string {
	var suggestions []string
	for _, check := range result.Checks {
		suggestions = append(suggestions, check.Suggestions...)
	}
	return strings.Join(suggestions, "\n")
}

// timeNow is a variable for testing
var timeNow = time.Now

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

	case "/review-stats":
		if r.reviewHistory == nil {
			fmt.Fprintf(os.Stdout, "%sReview history not available%s\n", ansiDim, ansiReset)
		} else {
			stats := r.reviewHistory.GetStats()
			fmt.Fprintf(os.Stdout, "\n%s📊 Review Statistics%s\n", ansiCyan, ansiReset)
			fmt.Fprintf(os.Stdout, "%s───────────────────────────────────────%s\n", ansiDim, ansiReset)
			fmt.Fprintf(os.Stdout, "Total reviews:    %d\n", stats.TotalReviews)
			fmt.Fprintf(os.Stdout, "Passed:           %d\n", stats.PassedCount)
			fmt.Fprintf(os.Stdout, "Failed:           %d\n", stats.FailedCount)
			fmt.Fprintf(os.Stdout, "Pass rate:        %.1f%%\n", stats.PassRate)
			fmt.Fprintf(os.Stdout, "Avg tokens/review: %d\n", stats.AvgTokensPerReview)
			fmt.Fprintf(os.Stdout, "Avg duration:     %dms\n", stats.AvgDurationMs)
			fmt.Fprintf(os.Stdout, "%s───────────────────────────────────────%s\n\n", ansiDim, ansiReset)
		}

	case "/review-clear":
		if r.reviewHistory == nil {
			fmt.Fprintf(os.Stdout, "%sReview history not available%s\n", ansiDim, ansiReset)
		} else {
			if err := r.reviewHistory.ClearHistory(); err != nil {
				fmt.Fprintf(os.Stdout, "%sFailed to clear history: %v%s\n", ansiRed, err, ansiReset)
			} else {
				fmt.Fprintf(os.Stdout, "%s✓ Review history cleared%s\n", ansiGreen, ansiReset)
			}
		}
		// Also clear incremental review cache
		if r.incrementalReviewer != nil {
			r.incrementalReviewer.ClearCache()
			r.lastResponse = ""
		}

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

%sReview%s
  /review-stats  Show review statistics
  /review-clear  Clear review history

%sShortcuts%s
  Ctrl+C         Exit
  Ctrl+D         Exit

%sInput Modes%s
  /command       Run a command
  !shell cmd     Run shell command directly

`, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset)
}
