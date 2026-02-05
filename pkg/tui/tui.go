// Package tui provides an interactive terminal UI matching Claude Code style
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ANSI color codes
const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[90m"
	ansiCyan   = "\033[36;1m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33;1m"
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
		OpID  uint64
	}
	InterruptMsg   struct{}
	TokenUpdateMsg struct {
		InputTokens  int
		OutputTokens int
		CostUSD      float64
	}
)

// SubmitCallback is called when user submits input
type SubmitCallback func(input string, opID uint64) tea.Cmd

// BashCallback is called for ! prefix commands
type BashCallback func(command string) tea.Cmd

// Model represents the TUI state
type Model struct {
	textinput    textinput.Model
	spinner      spinner.Model
	isStreaming  bool
	thinkingText string
	interrupted  bool
	currentOpID  uint64
	verbose      bool

	// Session info
	model        string
	cwd          string
	version      string
	sessionID    string
	sessionName  string
	messageCount int

	// Token tracking
	inputTokens  int
	outputTokens int
	totalCostUSD float64

	// Todo/Task list
	todos     []Todo
	showTodos bool

	// Dimensions
	width  int
	height int

	// Callbacks
	onSubmit        SubmitCallback
	onBash          BashCallback
	onListSessions  ListSessionsCallback
	onResumeSession ResumeSessionCallback
	onNewSession    NewSessionCallback

	ready bool
}

// Todo represents a task item
type Todo struct {
	Content string
	Status  string // pending, in_progress, completed
}

// SessionInfo holds session metadata for display
type SessionInfo struct {
	ID           string
	Summary      string
	MessageCount int
	UpdatedAt    string
	IsCurrent    bool
}

// Callback types
type ListSessionsCallback func() []SessionInfo
type ResumeSessionCallback func(id string) (messageCount int, err error)
type NewSessionCallback func() (sessionID string, err error)

// Config holds TUI configuration
type Config struct {
	Model           string
	CWD             string
	Version         string
	SessionID       string
	SessionName     string
	MessageCount    int
	OnSubmit        SubmitCallback
	OnBash          BashCallback
	OnListSessions  ListSessionsCallback
	OnResumeSession ResumeSessionCallback
	OnNewSession    NewSessionCallback
}

// New creates a new TUI model
func New(cfg Config) Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 0

	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		textinput:       ti,
		spinner:         s,
		model:           cfg.Model,
		cwd:             cfg.CWD,
		version:         cfg.Version,
		sessionID:       cfg.SessionID,
		sessionName:     cfg.SessionName,
		messageCount:    cfg.MessageCount,
		onSubmit:        cfg.OnSubmit,
		onBash:          cfg.OnBash,
		onListSessions:  cfg.OnListSessions,
		onResumeSession: cfg.OnResumeSession,
		onNewSession:    cfg.OnNewSession,
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
		case tea.KeyCtrlC:
			if m.isStreaming {
				m.interrupted = true
				fmt.Fprintf(os.Stdout, "\n%s⏹ Interrupted%s\n", ansiRed, ansiReset)
				return m, func() tea.Msg { return InterruptMsg{} }
			}
			return m, tea.Quit

		case tea.KeyCtrlD:
			return m, tea.Quit

		case tea.KeyCtrlL:
			fmt.Print("\033[H\033[2J")
			return m, nil

		case tea.KeyCtrlO:
			m.verbose = !m.verbose
			if m.verbose {
				fmt.Fprintf(os.Stdout, "%sVerbose mode enabled%s\n", ansiDim, ansiReset)
			} else {
				fmt.Fprintf(os.Stdout, "%sVerbose mode disabled%s\n", ansiDim, ansiReset)
			}
			return m, nil

		case tea.KeyCtrlT:
			m.showTodos = !m.showTodos
			if m.showTodos && len(m.todos) > 0 {
				m.printTodos()
			}
			return m, nil

		case tea.KeyEsc:
			if m.textinput.Value() != "" {
				m.textinput.Reset()
				return m, nil
			} else if m.isStreaming {
				m.interrupted = true
				fmt.Fprintf(os.Stdout, "\n%s⏹ Interrupted%s\n", ansiRed, ansiReset)
				return m, func() tea.Msg { return InterruptMsg{} }
			}
			return m, nil

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textinput.Value())
			if input == "" {
				return m, nil
			}

			if m.isStreaming {
				m.interrupted = true
				fmt.Fprintf(os.Stdout, "\n%s⏹ Interrupted%s\n\n", ansiRed, ansiReset)
			}

			m.textinput.Reset()

			if strings.HasPrefix(input, "/") {
				fmt.Fprintf(os.Stdout, "%s> %s%s\n\n", ansiDim, input, ansiReset)
				m.isStreaming = false
				return m, m.handleCommand(input)
			} else if strings.HasPrefix(input, "!") {
				bashCmd := strings.TrimPrefix(input, "!")
				bashCmd = strings.TrimSpace(bashCmd)
				fmt.Fprintf(os.Stdout, "%s$ %s%s\n", ansiDim, bashCmd, ansiReset)
				if m.onBash != nil {
					return m, m.onBash(bashCmd)
				}
				fmt.Fprintf(os.Stdout, "%sBash mode not available%s\n", ansiRed, ansiReset)
				return m, nil
			}

			fmt.Fprintf(os.Stdout, "> %s\n\n", input)
			m.isStreaming = true
			m.thinkingText = "Thinking"
			m.interrupted = false
			m.currentOpID++

			if m.onSubmit != nil {
				return m, tea.Batch(m.onSubmit(input, m.currentOpID), m.spinner.Tick)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textinput.Width = msg.Width - 4
		m.ready = true

	case spinner.TickMsg:
		if m.isStreaming {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case StreamTextMsg:
		fmt.Fprint(os.Stdout, msg.Text)

	case ToolUseMsg:
		m.thinkingText = msg.Name
		m.printToolUse(msg.Name, msg.Params)

	case ToolResultMsg:
		m.thinkingText = "Thinking"
		m.printToolResult(msg.Name, msg.Success, msg.Summary)

	case StreamDoneMsg:
		if msg.OpID == m.currentOpID {
			m.isStreaming = false
			m.thinkingText = ""
			if msg.Error != nil && !m.interrupted {
				fmt.Fprintf(os.Stdout, "\n%sError: %v%s\n", ansiRed, msg.Error, ansiReset)
			}
			fmt.Fprint(os.Stdout, "\n")
		}

	case TokenUpdateMsg:
		m.inputTokens += msg.InputTokens
		m.outputTokens += msg.OutputTokens
		m.totalCostUSD += msg.CostUSD
	}

	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders only the input prompt
func (m Model) View() string {
	if !m.ready {
		return ""
	}

	if m.isStreaming {
		return fmt.Sprintf("%s %s\n> %s", m.spinner.View(), m.thinkingText, m.textinput.View())
	}

	return fmt.Sprintf("> %s", m.textinput.View())
}

// PrintWelcome prints the welcome message
func (m *Model) PrintWelcome() {
	info := m.model
	if m.cwd != "" {
		info += " • " + shortenPath(m.cwd, 40)
	}
	if m.sessionID != "" {
		info += " • " + m.sessionID
		if m.messageCount > 0 {
			info += fmt.Sprintf(" (%d msgs)", m.messageCount)
		}
	}

	fmt.Fprintf(os.Stdout, "\n%sAgentic Coder v%s%s\n", ansiCyan, m.version, ansiReset)
	fmt.Fprintf(os.Stdout, "%s%s%s\n\n", ansiDim, info, ansiReset)
	fmt.Fprintf(os.Stdout, "%sType your message or /help for commands%s\n\n", ansiDim, ansiReset)
}

func (m *Model) printTodos() {
	fmt.Fprintf(os.Stdout, "%s━━━ Tasks ━━━%s\n", ansiDim, ansiReset)
	for i, todo := range m.todos {
		if i >= 10 {
			fmt.Fprintf(os.Stdout, "%s  ... and %d more%s\n", ansiDim, len(m.todos)-10, ansiReset)
			break
		}
		icon := "○"
		color := ansiDim
		switch todo.Status {
		case "in_progress":
			icon = "◐"
			color = ansiCyan
		case "completed":
			icon = "●"
			color = ansiGreen
		}
		fmt.Fprintf(os.Stdout, "%s  %s %s%s\n", color, icon, todo.Content, ansiReset)
	}
	fmt.Fprintf(os.Stdout, "%s━━━━━━━━━━━━━%s\n\n", ansiDim, ansiReset)
}

func (m *Model) printToolUse(name string, params map[string]interface{}) {
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
		if m.verbose {
			if old, ok := params["old_string"].(string); ok && old != "" {
				fmt.Fprintf(os.Stdout, "%s  - %s%s\n", ansiRed, m.formatCode(old), ansiReset)
			}
			if newStr, ok := params["new_string"].(string); ok && newStr != "" {
				fmt.Fprintf(os.Stdout, "%s  + %s%s\n", ansiGreen, m.formatCode(newStr), ansiReset)
			}
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
	default:
		count := 0
		for k, v := range params {
			if count >= 2 {
				fmt.Fprintf(os.Stdout, "%s   ...%s\n", ansiDim, ansiReset)
				break
			}
			valStr := truncate(fmt.Sprintf("%v", v), 60)
			valStr = strings.ReplaceAll(valStr, "\n", "↵")
			fmt.Fprintf(os.Stdout, "%s   %s: %s%s\n", ansiDim, k, valStr, ansiReset)
			count++
		}
	}
}

func (m *Model) printToolResult(name string, success bool, summary string) {
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

func (m *Model) formatCode(code string) string {
	lines := strings.Split(code, "\n")
	if len(lines) > 5 {
		return fmt.Sprintf("(%d lines)", len(lines))
	}
	if len(lines) == 1 {
		return truncate(code, 60)
	}
	return fmt.Sprintf("(%d lines)", len(lines))
}

func (m *Model) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/h", "/?":
		fmt.Fprint(os.Stdout, m.helpText())

	case "/clear":
		fmt.Print("\033[H\033[2J")

	case "/exit", "/quit", "/q":
		return tea.Quit

	case "/model":
		if len(parts) > 1 {
			m.model = parts[1]
			fmt.Fprintf(os.Stdout, "%sModel: %s%s\n", ansiGreen, m.model, ansiReset)
		} else {
			fmt.Fprintf(os.Stdout, "Current model: %s\n", m.model)
		}

	case "/cost":
		fmt.Fprintf(os.Stdout, "Input tokens:  %d\n", m.inputTokens)
		fmt.Fprintf(os.Stdout, "Output tokens: %d\n", m.outputTokens)
		fmt.Fprintf(os.Stdout, "Total cost:    $%.4f\n", m.totalCostUSD)

	case "/verbose":
		m.verbose = !m.verbose
		if m.verbose {
			fmt.Fprintf(os.Stdout, "%sVerbose mode enabled%s\n", ansiGreen, ansiReset)
		} else {
			fmt.Fprintf(os.Stdout, "%sVerbose mode disabled%s\n", ansiDim, ansiReset)
		}

	case "/todos", "/tasks":
		if len(m.todos) == 0 {
			fmt.Fprintf(os.Stdout, "%sNo tasks%s\n", ansiDim, ansiReset)
		} else {
			for _, todo := range m.todos {
				icon := "○"
				switch todo.Status {
				case "in_progress":
					icon = "◐"
				case "completed":
					icon = "●"
				}
				fmt.Fprintf(os.Stdout, "  %s %s\n", icon, todo.Content)
			}
		}

	case "/history", "/sessions", "/resume":
		if cmd == "/resume" && len(parts) > 1 {
			m.resumeSession(parts[1])
		} else {
			m.listSessions()
		}

	case "/new":
		m.newSession()

	case "/rename":
		if len(parts) > 1 {
			m.sessionName = strings.Join(parts[1:], " ")
			fmt.Fprintf(os.Stdout, "%sSession renamed to: %s%s\n", ansiGreen, m.sessionName, ansiReset)
		} else {
			fmt.Fprintf(os.Stdout, "%sUsage: /rename <name>%s\n", ansiRed, ansiReset)
		}

	case "/config", "/settings":
		fmt.Fprintf(os.Stdout, "%sSettings:%s\n", ansiDim, ansiReset)
		fmt.Fprintf(os.Stdout, "  Model: %s\n", m.model)
		fmt.Fprintf(os.Stdout, "  Verbose: %v\n", m.verbose)
		fmt.Fprintf(os.Stdout, "  CWD: %s\n", m.cwd)

	default:
		fmt.Fprintf(os.Stdout, "%sUnknown command: %s%s\n", ansiRed, cmd, ansiReset)
		fmt.Fprintf(os.Stdout, "%sType /help for available commands%s\n", ansiDim, ansiReset)
	}

	return nil
}

func (m *Model) listSessions() {
	if m.onListSessions == nil {
		fmt.Fprintf(os.Stdout, "%sSession management not available%s\n", ansiRed, ansiReset)
		return
	}
	sessions := m.onListSessions()
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
		fmt.Fprintf(os.Stdout, "%s%d. %s", marker, i+1, title)
		fmt.Fprintf(os.Stdout, "%s (%d msgs, %s)%s\n", ansiDim, s.MessageCount, s.UpdatedAt, ansiReset)
	}
	fmt.Fprintf(os.Stdout, "%s───────────────────────────────────────%s\n", ansiDim, ansiReset)
	fmt.Fprintf(os.Stdout, "%sUse /resume <number> to switch%s\n\n", ansiDim, ansiReset)
}

func (m *Model) resumeSession(idOrNum string) {
	if m.onResumeSession == nil {
		fmt.Fprintf(os.Stdout, "%sSession management not available%s\n", ansiRed, ansiReset)
		return
	}

	sessionID := idOrNum
	if m.onListSessions != nil {
		var num int
		if _, err := fmt.Sscanf(idOrNum, "%d", &num); err == nil {
			sessions := m.onListSessions()
			if num > 0 && num <= len(sessions) {
				sessionID = sessions[num-1].ID
			}
		}
	}

	msgCount, err := m.onResumeSession(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%sFailed to resume: %v%s\n", ansiRed, err, ansiReset)
		return
	}

	m.sessionID = sessionID
	if len(sessionID) > 8 {
		m.sessionID = sessionID[:8]
	}
	m.messageCount = msgCount
	fmt.Fprintf(os.Stdout, "%sResumed session: %s (%d messages)%s\n\n", ansiGreen, m.sessionID, msgCount, ansiReset)
}

func (m *Model) newSession() {
	if m.onNewSession == nil {
		fmt.Fprintf(os.Stdout, "%sSession management not available%s\n", ansiRed, ansiReset)
		return
	}

	sessionID, err := m.onNewSession()
	if err != nil {
		fmt.Fprintf(os.Stdout, "%sFailed to create session: %v%s\n", ansiRed, err, ansiReset)
		return
	}

	m.sessionID = sessionID
	if len(sessionID) > 8 {
		m.sessionID = sessionID[:8]
	}
	m.sessionName = ""
	m.messageCount = 0
	m.inputTokens = 0
	m.outputTokens = 0
	m.totalCostUSD = 0
	fmt.Fprintf(os.Stdout, "%sNew session: %s%s\n\n", ansiGreen, m.sessionID, ansiReset)
}

func (m *Model) helpText() string {
	return fmt.Sprintf(`
%sCommands%s
  /help          Show this help
  /clear         Clear screen
  /exit          Exit
  /model [name]  Show or change model
  /cost          Show token usage and cost
  /verbose       Toggle verbose mode
  /todos         Show task list

%sSessions%s
  /history       List sessions
  /resume <n>    Resume session by number or ID
  /new           Start new session
  /rename <name> Rename current session

%sShortcuts%s
  Ctrl+C         Interrupt / Exit
  Ctrl+D         Exit
  Ctrl+L         Clear screen
  Ctrl+O         Toggle verbose
  Ctrl+T         Toggle task list

%sInput Modes%s
  /command       Run a command
  !shell cmd     Run shell command directly

`, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset, ansiCyan, ansiReset)
}

// UpdateTodos updates the todo list
func (m *Model) UpdateTodos(todos []Todo) {
	m.todos = todos
}

func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Message constructors
func SendStreamText(text string) tea.Cmd {
	return func() tea.Msg { return StreamTextMsg{Text: text} }
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

func SendTokenUpdate(input, output int, cost float64) tea.Cmd {
	return func() tea.Msg { return TokenUpdateMsg{InputTokens: input, OutputTokens: output, CostUSD: cost} }
}
