// Package tui provides an interactive terminal UI for the coding assistant
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	StreamDoneMsg struct{ Error error }
	InterruptMsg  struct{}
)

// SubmitCallback is called when user submits input
type SubmitCallback func(input string) tea.Cmd

// Model represents the TUI state
type Model struct {
	textinput    textinput.Model
	spinner      spinner.Model
	output       *strings.Builder
	isStreaming  bool
	isThinking   bool
	thinkingText string
	interrupted  bool
	model        string
	cwd          string
	version      string
	width        int
	height       int
	onSubmit     SubmitCallback
	lastActivity time.Time
	showWelcome  bool
}

// Config holds TUI configuration
type Config struct {
	Model    string
	CWD      string
	Version  string
	OnSubmit SubmitCallback
}

// New creates a new TUI model
func New(cfg Config) Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.Prompt = ""  // We'll render prompt ourselves
	ti.CharLimit = 0

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	return Model{
		textinput:    ti,
		spinner:      s,
		output:       &strings.Builder{},
		model:        cfg.Model,
		cwd:          cfg.CWD,
		version:      cfg.Version,
		onSubmit:     cfg.OnSubmit,
		lastActivity: time.Now(),
		showWelcome:  true,
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
				m.lastActivity = time.Now()

				if m.onSubmit != nil {
					return m, tea.Batch(m.onSubmit(input), m.spinner.Tick)
				}
			}
			return m, nil

		case tea.KeyCtrlD:
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textinput.Width = msg.Width - 4

	case spinner.TickMsg:
		if m.isStreaming {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case StreamTextMsg:
		m.isThinking = false
		m.lastActivity = time.Now()
		m.print(msg.Text)

	case StreamThinkingMsg:
		m.lastActivity = time.Now()

	case ToolUseMsg:
		m.isThinking = false
		m.thinkingText = msg.Name
		m.lastActivity = time.Now()
		m.print(fmt.Sprintf("\n%s %s\n", toolStyle.Render("⚡"), msg.Name))
		for k, v := range msg.Params {
			valStr := fmt.Sprintf("%v", v)
			if len(valStr) > 80 {
				valStr = valStr[:80] + "..."
			}
			valStr = strings.ReplaceAll(valStr, "\n", "\\n")
			m.print(dimStyle.Render(fmt.Sprintf("  %s: %s\n", k, valStr)))
		}

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

	case StreamDoneMsg:
		m.isStreaming = false
		m.isThinking = false
		m.thinkingText = ""
		if msg.Error != nil && !m.interrupted {
			m.print(errorStyle.Render(fmt.Sprintf("\nError: %v\n", msg.Error)))
		}
		m.print("\n")
	}

	// Always update text input (allow typing while streaming)
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	var b strings.Builder

	// Welcome message (only shown once at start)
	if m.showWelcome {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Agentic Coder v%s", m.version)))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Model: %s | %s", m.model, shortenPath(m.cwd, 50))))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Type /help for commands, Ctrl+C to interrupt"))
		b.WriteString("\n\n")
	}

	// Output content
	output := m.output.String()
	b.WriteString(output)

	// Ensure output ends with newline before prompt
	if len(output) > 0 && !strings.HasSuffix(output, "\n") {
		b.WriteString("\n")
	}

	// Status line when streaming (show above prompt)
	if m.isStreaming {
		status := m.thinkingText
		if status == "" {
			status = "Thinking"
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("%s %s...", m.spinner.View(), status)))
		b.WriteString("\n")
	}

	// Prompt line (always visible): > input_here
	b.WriteString(promptStyle.Render("> "))
	b.WriteString(m.textinput.View())

	return b.String()
}

// print adds text to output
func (m *Model) print(text string) {
	m.output.WriteString(text)
	m.showWelcome = false // Hide welcome after first output
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
	case "/exit", "/quit", "/q":
		return tea.Quit
	case "/model":
		if len(parts) > 1 {
			m.model = parts[1]
			m.print(successStyle.Render(fmt.Sprintf("✓ Model: %s\n", m.model)))
		} else {
			m.print(fmt.Sprintf("Model: %s\n", m.model))
		}
	default:
		m.print(errorStyle.Render(fmt.Sprintf("Unknown command: %s\n", parts[0])))
	}
	return nil
}

func (m *Model) helpText() string {
	return dimStyle.Render(`
  Commands:
    /help      Show this help
    /clear     Clear screen
    /model     Show/change model
    /exit      Exit

  Shortcuts:
    Enter      Send message
    Ctrl+C     Interrupt / Exit
    Ctrl+D     Exit
`) + "\n"
}

// Helper functions
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
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

func SendStreamDone(err error) tea.Cmd {
	return func() tea.Msg { return StreamDoneMsg{Error: err} }
}
