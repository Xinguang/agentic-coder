// Package tui provides an interactive terminal UI for the coding assistant
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86")).
				Padding(0, 1)

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// Message types for tea.Cmd
type (
	// StreamTextMsg is sent when streaming text arrives
	StreamTextMsg struct {
		Text string
	}

	// StreamThinkingMsg is sent when thinking text arrives
	StreamThinkingMsg struct {
		Text string
	}

	// ToolUseMsg is sent when a tool is being used
	ToolUseMsg struct {
		Name   string
		Params map[string]interface{}
	}

	// ToolResultMsg is sent when a tool completes
	ToolResultMsg struct {
		Name    string
		Success bool
		Summary string
	}

	// StreamDoneMsg is sent when streaming completes
	StreamDoneMsg struct {
		Error error
	}

	// InterruptMsg is sent when user wants to interrupt
	InterruptMsg struct{}
)

// SubmitCallback is called when user submits input
type SubmitCallback func(input string) tea.Cmd

// Model represents the TUI state
type Model struct {
	// UI components
	viewport viewport.Model
	textarea textarea.Model

	// State
	content      strings.Builder
	ready        bool
	isStreaming  bool
	interrupted  bool
	model        string
	cwd          string
	messageCount int

	// Dimensions
	width  int
	height int

	// Callbacks
	onSubmit SubmitCallback

	// Error state
	err error
}

// Config holds TUI configuration
type Config struct {
	Model        string
	CWD          string
	Version      string
	OnSubmit     SubmitCallback
}

// New creates a new TUI model
func New(cfg Config) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Ctrl+C to interrupt)"
	ta.Focus()
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // Enter submits

	// Custom styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()

	m := Model{
		textarea:     ta,
		model:        cfg.Model,
		cwd:          cfg.CWD,
		onSubmit:     cfg.OnSubmit,
		messageCount: 0,
	}

	// Add welcome message
	m.appendOutput(fmt.Sprintf("%s Agentic Coder %s\n", titleStyle.Render(""), cfg.Version))
	m.appendOutput(fmt.Sprintf("%s Model: %s | CWD: %s\n", dimStyle.Render(""), cfg.Model, shortenPath(cfg.CWD, 50)))
	m.appendOutput(dimStyle.Render("Type your message below. Press Enter to send, Ctrl+C to interrupt.\n"))
	m.appendOutput(strings.Repeat("─", 60) + "\n")

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return textarea.Blink
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
				m.appendOutput(errorStyle.Render("\n⚠ Interrupted by user\n"))
				return m, func() tea.Msg { return InterruptMsg{} }
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if !m.isStreaming {
				input := strings.TrimSpace(m.textarea.Value())
				if input != "" {
					m.textarea.Reset()
					m.messageCount++
					m.appendOutput(fmt.Sprintf("\n%s %s\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render(">"), input))

					// Check for commands
					if strings.HasPrefix(input, "/") {
						return m, m.handleCommand(input)
					}

					m.isStreaming = true
					m.interrupted = false
					if m.onSubmit != nil {
						return m, m.onSubmit(input)
					}
				}
			}
			return m, nil

		case tea.KeyCtrlD:
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1
		inputHeight := 5 // textarea + border
		viewportHeight := m.height - headerHeight - inputHeight - 2

		if !m.ready {
			m.viewport = viewport.New(m.width, viewportHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.content.String())
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = viewportHeight
		}

		m.textarea.SetWidth(m.width - 4)

	case StreamTextMsg:
		m.appendOutput(msg.Text)
		m.viewport.GotoBottom()

	case StreamThinkingMsg:
		m.appendOutput(thinkingStyle.Render(msg.Text))
		m.viewport.GotoBottom()

	case ToolUseMsg:
		m.appendOutput(fmt.Sprintf("\n%s %s\n", toolStyle.Render("⚡"), msg.Name))
		for k, v := range msg.Params {
			valStr := fmt.Sprintf("%v", v)
			if len(valStr) > 80 {
				valStr = valStr[:80] + "..."
			}
			valStr = strings.ReplaceAll(valStr, "\n", "\\n")
			m.appendOutput(dimStyle.Render(fmt.Sprintf("   %s: %s\n", k, valStr)))
		}
		m.viewport.GotoBottom()

	case ToolResultMsg:
		if msg.Success {
			if msg.Summary != "" {
				m.appendOutput(successStyle.Render(fmt.Sprintf("✓ %s: %s\n", msg.Name, msg.Summary)))
			} else {
				m.appendOutput(successStyle.Render(fmt.Sprintf("✓ %s completed\n", msg.Name)))
			}
		} else {
			m.appendOutput(errorStyle.Render(fmt.Sprintf("✗ %s: %s\n", msg.Name, msg.Summary)))
		}
		m.viewport.GotoBottom()

	case StreamDoneMsg:
		m.isStreaming = false
		if msg.Error != nil && !m.interrupted {
			m.appendOutput(errorStyle.Render(fmt.Sprintf("\nError: %v\n", msg.Error)))
		}
		m.appendOutput("\n")
		m.viewport.GotoBottom()
	}

	// Update textarea
	if !m.isStreaming {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Status bar
	status := statusStyle.Render(fmt.Sprintf(" %s │ %d messages │ %s ",
		m.model, m.messageCount, shortenPath(m.cwd, 30)))

	streamingIndicator := ""
	if m.isStreaming {
		streamingIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Render(" ● Streaming...")
	}

	header := lipgloss.JoinHorizontal(lipgloss.Left, status, streamingIndicator)

	// Main content area
	content := m.viewport.View()

	// Input area
	inputBox := inputBorderStyle.Render(m.textarea.View())

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		inputBox,
	)
}

// appendOutput adds text to the output
func (m *Model) appendOutput(text string) {
	m.content.WriteString(text)
	if m.ready {
		m.viewport.SetContent(m.content.String())
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
		m.appendOutput(m.helpText())
	case "/clear", "/cls":
		m.content.Reset()
		m.viewport.SetContent("")
	case "/exit", "/quit", "/q":
		return tea.Quit
	case "/model":
		if len(parts) > 1 {
			m.model = parts[1]
			m.appendOutput(successStyle.Render(fmt.Sprintf("✓ Model changed to: %s\n", m.model)))
		} else {
			m.appendOutput(fmt.Sprintf("Current model: %s\n", m.model))
		}
	default:
		m.appendOutput(errorStyle.Render(fmt.Sprintf("Unknown command: %s\n", parts[0])))
	}

	m.viewport.GotoBottom()
	return nil
}

func (m *Model) helpText() string {
	return `
Commands:
  /help, /h      Show this help
  /clear, /cls   Clear the screen
  /model [name]  Show or change the model
  /exit, /quit   Exit the program

Keyboard:
  Enter          Send message
  Ctrl+C         Interrupt current operation / Exit
  Ctrl+D         Exit
`
}

// IsStreaming returns whether currently streaming
func (m *Model) IsStreaming() bool {
	return m.isStreaming
}

// SetStreaming sets streaming state
func (m *Model) SetStreaming(streaming bool) {
	m.isStreaming = streaming
}

// GetContent returns current output content
func (m *Model) GetContent() string {
	return m.content.String()
}

// shortenPath shortens a path for display
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// SendStreamText creates a command to send streaming text
func SendStreamText(text string) tea.Cmd {
	return func() tea.Msg {
		return StreamTextMsg{Text: text}
	}
}

// SendThinking creates a command to send thinking text
func SendThinking(text string) tea.Cmd {
	return func() tea.Msg {
		return StreamThinkingMsg{Text: text}
	}
}

// SendToolUse creates a command to show tool usage
func SendToolUse(name string, params map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		return ToolUseMsg{Name: name, Params: params}
	}
}

// SendToolResult creates a command to show tool result
func SendToolResult(name string, success bool, summary string) tea.Cmd {
	return func() tea.Msg {
		return ToolResultMsg{Name: name, Success: success, Summary: summary}
	}
}

// SendStreamDone creates a command to signal stream completion
func SendStreamDone(err error) tea.Cmd {
	return func() tea.Msg {
		return StreamDoneMsg{Error: err}
	}
}
