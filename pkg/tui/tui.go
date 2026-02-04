// Package tui provides an interactive terminal UI for the coding assistant
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles - Clean minimal design like Claude Code
var (
	// Prompt styles
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	// Status/dim text
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	// Tool styles
	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// Success style
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	// Thinking style
	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			Italic(true)

	// User input echo
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Bold(true)
)

// Message types for tea.Cmd
type (
	StreamTextMsg     struct{ Text string }
	StreamThinkingMsg struct{ Text string }
	ToolUseMsg        struct {
		Name   string
		Params map[string]interface{}
	}
	ToolResultMsg struct {
		Name    string
		Success bool
		Summary string
	}
	StreamDoneMsg struct {
		Error error
		OpID  uint64 // Operation ID to track which operation completed
	}
	InterruptMsg struct{}
)

// SubmitCallback is called when user submits input
// opID is passed to track the operation and return in StreamDoneMsg
type SubmitCallback func(input string, opID uint64) tea.Cmd

// Model represents the TUI state
type Model struct {
	textinput    textinput.Model
	spinner      spinner.Model
	viewport     viewport.Model
	output       *strings.Builder
	isStreaming  bool
	isThinking   bool
	thinkingText string
	interrupted  bool
	currentOpID  uint64 // Current operation ID
	autoScroll   bool   // Auto-scroll to bottom on new content
	model        string
	cwd          string
	version      string
	sessionID    string // Short session ID
	messageCount int    // Messages in session (for resumed sessions)
	width           int
	height          int
	onSubmit        SubmitCallback
	onListSessions  ListSessionsCallback
	onResumeSession ResumeSessionCallback
	onNewSession    NewSessionCallback
	lastActivity    time.Time
	showWelcome     bool
	ready           bool // Viewport ready after first WindowSizeMsg
}

// SessionInfo holds session metadata for display
type SessionInfo struct {
	ID           string
	Summary      string
	MessageCount int
	UpdatedAt    string
	IsCurrent    bool
}

// SessionCallback types
type ListSessionsCallback func() []SessionInfo
type ResumeSessionCallback func(id string) (messageCount int, err error)
type NewSessionCallback func() (sessionID string, err error)

// Config holds TUI configuration
type Config struct {
	Model        string
	CWD          string
	Version      string
	SessionID    string // Short session ID for display
	MessageCount int    // Number of messages in resumed session
	OnSubmit     SubmitCallback
	OnListSessions  ListSessionsCallback
	OnResumeSession ResumeSessionCallback
	OnNewSession    NewSessionCallback
}

// New creates a new TUI model
func New(cfg Config) Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.Prompt = "" // We'll render prompt ourselves
	ti.CharLimit = 0

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	vp := viewport.New(80, 20) // Will be resized on WindowSizeMsg
	vp.SetContent("")

	return Model{
		textinput:       ti,
		spinner:         s,
		viewport:        vp,
		output:          &strings.Builder{},
		autoScroll:      true,
		model:           cfg.Model,
		cwd:             cfg.CWD,
		version:         cfg.Version,
		sessionID:       cfg.SessionID,
		messageCount:    cfg.MessageCount,
		onSubmit:        cfg.OnSubmit,
		onListSessions:  cfg.OnListSessions,
		onResumeSession: cfg.OnResumeSession,
		onNewSession:    cfg.OnNewSession,
		lastActivity:    time.Now(),
		showWelcome:     true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			// Clear input or stop streaming on Escape
			if m.textinput.Value() != "" {
				m.textinput.Reset()
				return m, nil
			} else if m.isStreaming {
				m.interrupted = true
				m.print(errorStyle.Render("\n⚠ Interrupted\n"))
				return m, func() tea.Msg { return InterruptMsg{} }
			}
			return m, nil

		case tea.KeyCtrlC:
			if m.isStreaming {
				m.interrupted = true
				m.print(errorStyle.Render("\n⚠ Interrupted\n"))
				return m, func() tea.Msg { return InterruptMsg{} }
			}
			return m, tea.Quit

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textinput.Value())
			if input != "" {
				// If streaming, interrupt first
				if m.isStreaming {
					m.interrupted = true
					m.print(errorStyle.Render("\n⚠ Interrupted\n\n"))
				}

				m.textinput.Reset()

				// Echo user input
				m.print(userStyle.Render(input) + "\n\n")

				// Handle commands
				if strings.HasPrefix(input, "/") {
					m.isStreaming = false
					m.isThinking = false
					return m, m.handleCommand(input)
				}

				m.isStreaming = true
				m.isThinking = true
				m.thinkingText = "Thinking"
				m.interrupted = false
				m.currentOpID++ // Increment operation ID
				m.lastActivity = time.Now()

				if m.onSubmit != nil {
					return m, tea.Batch(m.onSubmit(input, m.currentOpID), m.spinner.Tick)
				}
			}
			return m, nil

		case tea.KeyCtrlD:
			return m, tea.Quit

		case tea.KeyTab:
			// Tab: ignore (could be used for autocomplete later)
			return m, nil

		case tea.KeyPgUp:
			m.autoScroll = false
			m.viewport.ViewUp()
			return m, nil

		case tea.KeyPgDown:
			m.viewport.ViewDown()
			// Re-enable auto-scroll if at bottom
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
			return m, nil

		case tea.KeyUp:
			// Allow scrolling with arrow keys (Ctrl+Up for scrolling when typing)
			m.autoScroll = false
			m.viewport.LineUp(1)
			return m, nil

		case tea.KeyDown:
			// Allow scrolling with arrow keys (Ctrl+Down for scrolling when typing)
			m.viewport.LineDown(1)
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textinput.Width = msg.Width - 4

		// Calculate viewport height (total - status line - input line - padding)
		headerHeight := 0
		footerHeight := 2 // status line + input line
		if m.isStreaming {
			footerHeight = 3 // extra line for thinking status
		}
		vpHeight := msg.Height - headerHeight - footerHeight
		if vpHeight < 1 {
			vpHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.viewport.SetContent(m.getViewportContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}

		// Auto-scroll to bottom
		if m.autoScroll {
			m.viewport.GotoBottom()
		}

	case spinner.TickMsg:
		if m.isStreaming {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case StreamTextMsg:
		m.isThinking = false
		m.lastActivity = time.Now()
		m.print(msg.Text)
		m.updateViewport()

	case StreamThinkingMsg:
		m.lastActivity = time.Now()

	case ToolUseMsg:
		m.isThinking = false
		m.thinkingText = msg.Name
		m.lastActivity = time.Now()
		m.print(fmt.Sprintf("\n%s %s\n", toolStyle.Render("⚡"), msg.Name))

		// For Edit tool, show full diff
		if msg.Name == "Edit" {
			if filePath, ok := msg.Params["file_path"].(string); ok {
				m.print(dimStyle.Render(fmt.Sprintf("  file: %s\n", filePath)))
			}
			if oldStr, ok := msg.Params["old_string"].(string); ok && oldStr != "" {
				m.print(errorStyle.Render("  - ") + dimStyle.Render(formatCodeBlock(oldStr)) + "\n")
			}
			if newStr, ok := msg.Params["new_string"].(string); ok && newStr != "" {
				m.print(successStyle.Render("  + ") + dimStyle.Render(formatCodeBlock(newStr)) + "\n")
			}
		} else if msg.Name == "Write" {
			if filePath, ok := msg.Params["file_path"].(string); ok {
				m.print(dimStyle.Render(fmt.Sprintf("  file: %s\n", filePath)))
			}
			if content, ok := msg.Params["content"].(string); ok {
				lines := strings.Split(content, "\n")
				m.print(dimStyle.Render(fmt.Sprintf("  content: %d lines\n", len(lines))))
			}
		} else {
			// Default: show params with truncation
			for k, v := range msg.Params {
				valStr := fmt.Sprintf("%v", v)
				if len(valStr) > 100 {
					valStr = valStr[:100] + "..."
				}
				valStr = strings.ReplaceAll(valStr, "\n", "\\n")
				m.print(dimStyle.Render(fmt.Sprintf("  %s: %s\n", k, valStr)))
			}
		}
		m.updateViewport()

	case ToolResultMsg:
		m.thinkingText = "Thinking"
		m.lastActivity = time.Now()
		if msg.Success {
			if msg.Summary != "" {
				m.print(successStyle.Render(fmt.Sprintf("✓ %s: %s\n", msg.Name, msg.Summary)))
			} else {
				m.print(successStyle.Render(fmt.Sprintf("✓ %s\n", msg.Name)))
			}
		} else {
			m.print(errorStyle.Render(fmt.Sprintf("✗ %s: %s\n", msg.Name, msg.Summary)))
		}
		m.updateViewport()

	case StreamDoneMsg:
		// Only process if this is the current operation (ignore stale messages)
		if msg.OpID == m.currentOpID {
			m.isStreaming = false
			m.isThinking = false
			m.thinkingText = ""
			if msg.Error != nil && !m.interrupted {
				m.print(errorStyle.Render(fmt.Sprintf("\nError: %v\n", msg.Error)))
			}
			m.print("\n")
			m.updateViewport()
		}
	}

	// Always update text input (allow typing while streaming)
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Viewport (scrollable output area)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status line when streaming (show above prompt)
	if m.isStreaming {
		status := m.thinkingText
		if status == "" {
			status = "Thinking"
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("%s %s...", m.spinner.View(), status)))
		b.WriteString("\n")
	}

	// Scroll indicator
	scrollInfo := ""
	if !m.viewport.AtBottom() {
		scrollInfo = dimStyle.Render(" [↑↓ scroll]")
	}

	// Prompt line (always visible): > input_here
	b.WriteString(promptStyle.Render("> "))
	b.WriteString(m.textinput.View())
	b.WriteString(scrollInfo)

	return b.String()
}

// print adds text to output
func (m *Model) print(text string) {
	m.output.WriteString(text)
	m.showWelcome = false // Hide welcome after first output
}

// getViewportContent returns the content for the viewport
func (m *Model) getViewportContent() string {
	var b strings.Builder

	// Welcome message (only shown once at start)
	if m.showWelcome {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Agentic Coder v%s", m.version)))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Model: %s | %s", m.model, shortenPath(m.cwd, 50))))
		b.WriteString("\n")
		if m.sessionID != "" {
			if m.messageCount > 0 {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  Session: %s (resumed, %d messages)", m.sessionID, m.messageCount)))
			} else {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  Session: %s", m.sessionID)))
			}
			b.WriteString("\n")
		}
		b.WriteString(dimStyle.Render("  Type /help for commands, ↑↓/PgUp/PgDn to scroll"))
		b.WriteString("\n\n")
	}

	// Output content
	b.WriteString(m.output.String())

	return b.String()
}

// updateViewport updates the viewport content and scrolls if needed
func (m *Model) updateViewport() {
	m.viewport.SetContent(m.getViewportContent())
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// handleCommand processes slash commands
func (m *Model) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "/help", "/h":
		m.print(m.helpText())
	case "/clear", "/cls":
		m.output.Reset()
		m.showWelcome = true
		m.updateViewport()
	case "/exit", "/quit", "/q":
		return tea.Quit
	case "/model":
		if len(parts) > 1 {
			m.model = parts[1]
			m.print(successStyle.Render(fmt.Sprintf("✓ Model: %s\n", m.model)))
		} else {
			m.print(fmt.Sprintf("Model: %s\n", m.model))
		}

	case "/history", "/sessions":
		if m.onListSessions == nil {
			m.print(errorStyle.Render("Session management not available\n"))
			return nil
		}
		sessions := m.onListSessions()
		if len(sessions) == 0 {
			m.print(dimStyle.Render("No sessions found\n"))
			return nil
		}
		m.print(dimStyle.Render("\n📋 Session History\n"))
		m.print(dimStyle.Render("─────────────────────────────────────────\n"))
		for i, s := range sessions {
			marker := "  "
			if s.IsCurrent {
				marker = successStyle.Render("▶ ")
			}
			// Show: number. title (msgs, date)
			m.print(fmt.Sprintf("%s%d. %s", marker, i+1, s.Summary))
			m.print(dimStyle.Render(fmt.Sprintf(" (%d msgs, %s)\n", s.MessageCount, s.UpdatedAt)))
		}
		m.print(dimStyle.Render("─────────────────────────────────────────\n"))
		m.print(dimStyle.Render("Use /resume <number> to switch session\n\n"))

	case "/resume":
		if m.onResumeSession == nil {
			m.print(errorStyle.Render("Session management not available\n"))
			return nil
		}
		if len(parts) < 2 {
			m.print(errorStyle.Render("Usage: /resume <session-id or number>\n"))
			return nil
		}
		// Check if it's a number (from /history list)
		sessionID := parts[1]
		if m.onListSessions != nil {
			if idx, err := fmt.Sscanf(parts[1], "%d", new(int)); err == nil && idx == 1 {
				num := 0
				fmt.Sscanf(parts[1], "%d", &num)
				sessions := m.onListSessions()
				if num > 0 && num <= len(sessions) {
					sessionID = sessions[num-1].ID
				}
			}
		}
		msgCount, err := m.onResumeSession(sessionID)
		if err != nil {
			m.print(errorStyle.Render(fmt.Sprintf("Failed to resume: %v\n", err)))
			return nil
		}
		m.sessionID = sessionID
		if len(sessionID) > 8 {
			m.sessionID = sessionID[:8]
		}
		m.messageCount = msgCount
		m.print(successStyle.Render(fmt.Sprintf("✓ Resumed session: %s (%d messages)\n", m.sessionID, msgCount)))

	case "/new":
		if m.onNewSession == nil {
			m.print(errorStyle.Render("Session management not available\n"))
			return nil
		}
		sessionID, err := m.onNewSession()
		if err != nil {
			m.print(errorStyle.Render(fmt.Sprintf("Failed to create session: %v\n", err)))
			return nil
		}
		m.sessionID = sessionID
		if len(sessionID) > 8 {
			m.sessionID = sessionID[:8]
		}
		m.messageCount = 0
		m.output.Reset()
		m.showWelcome = true
		m.print(successStyle.Render(fmt.Sprintf("✓ New session: %s\n", m.sessionID)))

	default:
		m.print(errorStyle.Render(fmt.Sprintf("Unknown command: %s\n", parts[0])))
	}
	m.updateViewport()
	return nil
}

func (m *Model) helpText() string {
	return dimStyle.Render(`
  Commands:
    /help      Show this help
    /history   List session history
    /resume N  Resume session (N = number or ID)
    /new       Start new session
    /clear     Clear screen
    /model     Show/change model
    /exit      Exit

  Shortcuts:
    Enter      Send message
    Ctrl+C     Interrupt / Exit
    Ctrl+D     Exit
    Esc        Clear input / Cancel
    ↑/↓        Scroll
    PgUp/PgDn  Scroll page
`) + "\n"
}

// Helper functions
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// formatCodeBlock formats a code block for display, limiting lines
func formatCodeBlock(code string) string {
	lines := strings.Split(code, "\n")
	maxLines := 10
	if len(lines) > maxLines {
		result := strings.Join(lines[:maxLines], "\n    ")
		return fmt.Sprintf("\n    %s\n    ... (%d more lines)", result, len(lines)-maxLines)
	}
	if len(lines) > 1 {
		return "\n    " + strings.Join(lines, "\n    ")
	}
	return code
}

// Message constructors for external use
func SendStreamText(text string) tea.Cmd {
	return func() tea.Msg { return StreamTextMsg{Text: text} }
}

func SendThinking(text string) tea.Cmd {
	return func() tea.Msg { return StreamThinkingMsg{Text: text} }
}

func SendToolUse(name string, params map[string]interface{}) tea.Cmd {
	return func() tea.Msg { return ToolUseMsg{Name: name, Params: params} }
}

func SendToolResult(name string, success bool, summary string) tea.Cmd {
	return func() tea.Msg { return ToolResultMsg{Name: name, Success: success, Summary: summary} }
}

func SendStreamDone(err error, opID uint64) tea.Cmd {
	return func() tea.Msg { return StreamDoneMsg{Error: err, OpID: opID} }
}
