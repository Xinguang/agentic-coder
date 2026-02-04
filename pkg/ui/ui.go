// Package ui provides CLI user interface utilities
package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Colors for terminal output
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Dim        = "\033[2m"
	Italic     = "\033[3m"
	Underline  = "\033[4m"

	// Foreground colors
	Black      = "\033[30m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	White      = "\033[37m"
	Gray       = "\033[90m"

	// Bright colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"

	// Background colors
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// Icons for various UI elements
const (
	IconSuccess   = "✓"
	IconError     = "✗"
	IconWarning   = "⚠"
	IconInfo      = "ℹ"
	IconQuestion  = "?"
	IconArrow     = "→"
	IconBullet    = "•"
	IconStar      = "★"
	IconCheck     = "✓"
	IconCross     = "✗"
	IconThinking  = "💭"
	IconTool      = "⚡"
	IconFile      = "📄"
	IconFolder    = "📁"
	IconCode      = "💻"
	IconRocket    = "🚀"
	IconKey       = "🔐"
	IconGear      = "⚙"
	IconClock     = "⏱"
	IconSave      = "💾"
	IconLoad      = "📂"
	IconChat      = "💬"
	IconBot       = "🤖"
	IconUser      = "👤"
	IconWork      = "📋"
)

// Printer handles formatted output
type Printer struct {
	NoColor bool
}

// NewPrinter creates a new printer
func NewPrinter() *Printer {
	// Check if NO_COLOR env is set or output is not a terminal
	noColor := os.Getenv("NO_COLOR") != ""
	return &Printer{NoColor: noColor}
}

// color applies color if enabled
func (p *Printer) color(c, text string) string {
	if p.NoColor {
		return text
	}
	return c + text + Reset
}

// Success prints a success message
func (p *Printer) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(p.color(Green, IconSuccess+" "+msg))
}

// Error prints an error message
func (p *Printer) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(p.color(Red, IconError+" "+msg))
}

// Warning prints a warning message
func (p *Printer) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(p.color(Yellow, IconWarning+" "+msg))
}

// Info prints an info message
func (p *Printer) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(p.color(Blue, IconInfo+" "+msg))
}

// Dim prints dimmed text
func (p *Printer) Dim(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(p.color(Gray, msg))
}

// Bold prints bold text
func (p *Printer) Bold(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(p.color(Bold, msg))
}

// Title prints a title
func (p *Printer) Title(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println()
	fmt.Println(p.color(Bold+BrightCyan, msg))
	fmt.Println(p.color(Dim, strings.Repeat("─", len(msg))))
}

// Section prints a section header
func (p *Printer) Section(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println()
	fmt.Println(p.color(Bold+White, msg))
}

// Tool prints tool usage info
func (p *Printer) Tool(name string) {
	fmt.Print(p.color(Yellow, IconTool+" "+name))
}

// ToolParam prints a tool parameter
func (p *Printer) ToolParam(key string, value string) {
	if len(value) > 100 {
		value = value[:100] + "..."
	}
	value = strings.ReplaceAll(value, "\n", "\\n")
	fmt.Printf("   %s%s: %s%s\n", Gray, key, value, Reset)
}

// ToolSuccess prints tool success
func (p *Printer) ToolSuccess(name string, summary string) {
	if summary != "" {
		fmt.Printf("%s%s %s: %s%s\n", Green, IconSuccess, name, summary, Reset)
	} else {
		fmt.Printf("%s%s %s completed%s\n", Green, IconSuccess, name, Reset)
	}
}

// ToolError prints tool error
func (p *Printer) ToolError(name string, err string) {
	fmt.Printf("%s%s %s: %s%s\n", Red, IconError, name, err, Reset)
}

// Thinking prints thinking indicator
func (p *Printer) Thinking(text string) {
	fmt.Printf("%s%s %s%s", Gray, IconThinking, text, Reset)
}

// Prompt prints the input prompt
func (p *Printer) Prompt() {
	fmt.Print(p.color(BrightCyan+Bold, "> "))
}

// PromptContinue prints a continuation prompt
func (p *Printer) PromptContinue() {
	fmt.Print(p.color(Dim, "... "))
}

// StatusLine prints a status line
func (p *Printer) StatusLine(model, cwd string, msgCount int) {
	status := fmt.Sprintf("%s %s │ %s %s │ %s %d messages",
		IconBot, model,
		IconFolder, shortenPath(cwd, 30),
		IconChat, msgCount)
	fmt.Println(p.color(Dim, status))
}

// WelcomeBanner prints the welcome banner
func (p *Printer) WelcomeBanner(version, model, cwd string) {
	fmt.Println()
	fmt.Println(p.color(Bold+BrightCyan, "  ╭─────────────────────────────────────────╮"))
	fmt.Println(p.color(Bold+BrightCyan, "  │")+p.color(Bold+White, "     Agentic Coder ")+p.color(Dim, "v"+version)+p.color(Bold+BrightCyan, strings.Repeat(" ", 22-len(version))+"│"))
	fmt.Println(p.color(Bold+BrightCyan, "  │")+p.color(Dim, "     AI-Powered Coding Assistant        ")+p.color(Bold+BrightCyan, "│"))
	fmt.Println(p.color(Bold+BrightCyan, "  ╰─────────────────────────────────────────╯"))
	fmt.Println()
	fmt.Printf("  %s Model: %s%s%s\n", IconBot, BrightGreen, model, Reset)
	fmt.Printf("  %s CWD:   %s%s%s\n", IconFolder, Dim, shortenPath(cwd, 40), Reset)
	fmt.Println()
	fmt.Println(p.color(Dim, "  Type /help for commands, Ctrl+C to interrupt"))
	fmt.Println()
}

// HelpMenu prints the help menu
func (p *Printer) HelpMenu() {
	fmt.Println()
	fmt.Println(p.color(Bold+BrightCyan, "  Commands"))
	fmt.Println(p.color(Dim, "  "+strings.Repeat("─", 50)))

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help, /h", "Show this help"},
		{"/clear, /cls", "Clear the screen"},
		{"/model [name]", "Show or change the model"},
		{"/session", "Show current session info"},
		{"/sessions", "List recent sessions"},
		{"/resume [id]", "Resume a previous session"},
		{"/new", "Start a new session"},
		{"/save", "Save current session"},
		{"/work", "Manage work context"},
		{"/work new <title>", "Create new work context"},
		{"/work list", "List work contexts"},
		{"/work show <id>", "Show work context"},
		{"/work done <text>", "Mark item as done"},
		{"/work todo <text>", "Add pending item"},
		{"/work handoff", "Generate handoff summary"},
		{"/compact", "Compact conversation history"},
		{"/cost", "Show token usage and cost"},
		{"/exit, /quit, /q", "Exit the program"},
	}

	for _, c := range commands {
		fmt.Printf("  %s%-20s%s %s%s%s\n", BrightYellow, c.cmd, Reset, Dim, c.desc, Reset)
	}

	fmt.Println()
	fmt.Println(p.color(Bold+BrightCyan, "  Keyboard Shortcuts"))
	fmt.Println(p.color(Dim, "  "+strings.Repeat("─", 50)))
	fmt.Printf("  %sCtrl+C%s           %sInterrupt current operation%s\n", BrightYellow, Reset, Dim, Reset)
	fmt.Printf("  %sCtrl+C (twice)%s   %sExit the program%s\n", BrightYellow, Reset, Dim, Reset)
	fmt.Printf("  %sCtrl+D%s           %sExit the program%s\n", BrightYellow, Reset, Dim, Reset)
	fmt.Println()
}

// SessionInfo prints session information
func (p *Printer) SessionInfo(id, model string, msgCount int, created, updated time.Time) {
	fmt.Println()
	fmt.Println(p.color(Bold, "Session Info"))
	fmt.Println(p.color(Dim, strings.Repeat("─", 40)))
	fmt.Printf("  ID:       %s%s%s\n", BrightCyan, id, Reset)
	fmt.Printf("  Model:    %s%s%s\n", Green, model, Reset)
	fmt.Printf("  Messages: %s%d%s\n", Yellow, msgCount, Reset)
	fmt.Printf("  Created:  %s%s%s\n", Dim, created.Format("2006-01-02 15:04:05"), Reset)
	fmt.Printf("  Updated:  %s%s%s\n", Dim, updated.Format("2006-01-02 15:04:05"), Reset)
	fmt.Println()
}

// SessionList prints a list of sessions
func (p *Printer) SessionList(sessions []SessionListItem) {
	if len(sessions) == 0 {
		fmt.Println(p.color(Dim, "No sessions found."))
		return
	}

	fmt.Println()
	fmt.Println(p.color(Bold, "Recent Sessions"))
	fmt.Println(p.color(Dim, strings.Repeat("─", 60)))

	for i, s := range sessions {
		marker := " "
		if s.IsCurrent {
			marker = p.color(Green, IconArrow)
		}

		age := formatAge(s.UpdatedAt)
		preview := s.Preview
		if len(preview) > 40 {
			preview = preview[:40] + "..."
		}

		fmt.Printf(" %s %s%s%s  %s%-8s%s  %s%s%s\n",
			marker,
			BrightCyan, s.ID[:8], Reset,
			Dim, age, Reset,
			Gray, preview, Reset)

		if i >= 9 {
			remaining := len(sessions) - 10
			if remaining > 0 {
				fmt.Printf("   %s... and %d more%s\n", Dim, remaining, Reset)
			}
			break
		}
	}
	fmt.Println()
}

// SessionListItem represents a session in the list
type SessionListItem struct {
	ID        string
	Preview   string
	UpdatedAt time.Time
	IsCurrent bool
}

// WorkContextList prints work context list
func (p *Printer) WorkContextList(contexts []WorkContextItem) {
	if len(contexts) == 0 {
		fmt.Println(p.color(Dim, "No work contexts found. Use '/work new <title>' to create one."))
		return
	}

	fmt.Println()
	fmt.Println(p.color(Bold, IconWork+" Work Contexts"))
	fmt.Println(p.color(Dim, strings.Repeat("─", 60)))

	for _, ctx := range contexts {
		pct := 0
		if ctx.Done+ctx.Pending > 0 {
			pct = ctx.Done * 100 / (ctx.Done + ctx.Pending)
		}

		progressBar := p.progressBar(pct, 10)

		fmt.Printf("  %s%s%s  %s %s%3d%%%s  %s (%d/%d)\n",
			BrightCyan, ctx.ID, Reset,
			progressBar,
			Dim, pct, Reset,
			ctx.Title, ctx.Done, ctx.Done+ctx.Pending)
	}
	fmt.Println()
}

// WorkContextItem represents a work context in the list
type WorkContextItem struct {
	ID      string
	Title   string
	Done    int
	Pending int
}

// progressBar creates a progress bar
func (p *Printer) progressBar(pct, width int) string {
	filled := pct * width / 100
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	color := Red
	if pct >= 70 {
		color = Green
	} else if pct >= 30 {
		color = Yellow
	}

	return p.color(color, bar)
}

// CostSummary prints token usage and cost
func (p *Printer) CostSummary(inputTokens, outputTokens int64, cost float64) {
	fmt.Println()
	fmt.Println(p.color(Bold, "Token Usage"))
	fmt.Println(p.color(Dim, strings.Repeat("─", 30)))
	fmt.Printf("  Input:   %s%d%s tokens\n", BrightCyan, inputTokens, Reset)
	fmt.Printf("  Output:  %s%d%s tokens\n", BrightCyan, outputTokens, Reset)
	fmt.Printf("  Total:   %s%d%s tokens\n", Yellow, inputTokens+outputTokens, Reset)
	if cost > 0 {
		fmt.Printf("  Cost:    %s$%.4f%s\n", Green, cost, Reset)
	}
	fmt.Println()
}

// Divider prints a divider line
func (p *Printer) Divider() {
	fmt.Println(p.color(Dim, strings.Repeat("─", 50)))
}

// NewLine prints a new line
func (p *Printer) NewLine() {
	fmt.Println()
}

// shortenPath shortens a path to fit within maxLen
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Try to keep the last parts
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) <= 2 {
		return "..." + path[len(path)-maxLen+3:]
	}

	// Keep first and last parts
	result := parts[0] + "/.../" + parts[len(parts)-1]
	if len(result) > maxLen {
		return "..." + path[len(path)-maxLen+3:]
	}
	return result
}

// formatAge formats a time as a human-readable age
func formatAge(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
