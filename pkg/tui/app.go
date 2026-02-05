// Package tui provides a terminal UI using bubbletea
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// Messages
type (
	// tickMsg for spinner animation
	tickMsg time.Time

	// contentMsg for adding content to viewport
	contentMsg struct {
		content string
		isError bool
	}

	// statusMsg for updating status bar
	statusMsg struct {
		text      string
		isWorking bool
	}

	// doneMsg signals completion
	doneMsg struct{}
)

// AppModel represents the TUI state
type AppModel struct {
	viewport    viewport.Model
	textarea    textarea.Model
	content     strings.Builder
	statusText  string
	isWorking   bool
	spinnerIdx  int
	startTime   time.Time
	tokenCount  int
	width       int
	height      int
	ready       bool
	mdRenderer  *glamour.TermRenderer

	// Pending input queue
	pendingInput string

	// Callbacks
	onSubmit func(input string)
	onCancel func()
}

// NewAppModel creates a new TUI model
func NewAppModel() *AppModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)

	return &AppModel{
		textarea:   ta,
		statusText: "Ready",
		mdRenderer: renderer,
	}
}

// SetCallbacks sets the submit and cancel callbacks
func (m *AppModel) SetCallbacks(onSubmit func(string), onCancel func()) {
	m.onSubmit = onSubmit
	m.onCancel = onCancel
}

// Init implements tea.Model
func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.tickCmd())
}

func (m *AppModel) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.isWorking && m.onCancel != nil {
				m.onCancel()
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyEsc:
			if m.isWorking && m.onCancel != nil {
				m.onCancel()
			}
			return m, nil

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textarea.Value())
			if input != "" {
				m.textarea.Reset()
				if m.isWorking {
					// Queue the input for later
					m.pendingInput = input
					m.AppendContent(fmt.Sprintf("\n%s[Queued: %s]%s\n", ansiDim, input, ansiReset))
				} else {
					if m.onSubmit != nil {
						m.onSubmit(input)
					}
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate heights: viewport gets most space, status 1 line, input 3 lines
		statusHeight := 1
		inputHeight := 3
		viewportHeight := m.height - statusHeight - inputHeight

		if !m.ready {
			m.viewport = viewport.New(m.width, viewportHeight)
			m.viewport.SetContent(m.content.String())
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = viewportHeight
		}

		m.textarea.SetWidth(m.width - 2)
		return m, nil

	case tickMsg:
		if m.isWorking {
			m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerChars)
		}
		return m, m.tickCmd()

	case contentMsg:
		// Debug: log message receipt
		if os.Getenv("DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] contentMsg received: %d bytes\n", len(msg.content))
		}
		if msg.isError {
			m.content.WriteString(fmt.Sprintf("\n\033[31m%s\033[0m\n", msg.content))
		} else {
			m.content.WriteString(msg.content)
		}
		m.viewport.SetContent(m.content.String())
		m.viewport.GotoBottom()
		return m, nil

	case statusMsg:
		m.statusText = msg.text
		m.isWorking = msg.isWorking
		if msg.isWorking {
			m.startTime = time.Now()
		}
		return m, nil

	case doneMsg:
		m.isWorking = false
		m.statusText = "Ready"
		// Process pending input if any
		if m.pendingInput != "" && m.onSubmit != nil {
			input := m.pendingInput
			m.pendingInput = ""
			m.onSubmit(input)
		}
		return m, nil
	}

	// Always update textarea (allow typing while working)
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport (for scrolling)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m *AppModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Build the three regions
	var b strings.Builder

	// Region 1: Content viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Region 2: Status bar
	statusContent := m.buildStatusBar()
	b.WriteString(statusContent)
	b.WriteString("\n")

	// Region 3: Input area
	b.WriteString(m.textarea.View())

	return b.String()
}

func (m *AppModel) buildStatusBar() string {
	var parts []string

	if m.isWorking {
		// Spinner + status text
		spinner := spinnerChars[m.spinnerIdx]
		parts = append(parts, fmt.Sprintf("%s %s", spinner, m.statusText))

		// Elapsed time
		elapsed := time.Since(m.startTime)
		if elapsed.Seconds() >= 1 {
			parts = append(parts, fmt.Sprintf("%.0fs", elapsed.Seconds()))
		}

		// Token count if available
		if m.tokenCount > 0 {
			parts = append(parts, fmt.Sprintf("↓ %dk tokens", m.tokenCount/1000))
		}

		// Hint
		parts = append(parts, "esc to interrupt")
	} else {
		parts = append(parts, m.statusText)
	}

	text := strings.Join(parts, " · ")
	return statusStyle.Width(m.width).Render(text)
}

// AppendContent adds content to the viewport
func (m *AppModel) AppendContent(content string) {
	m.content.WriteString(content)
	m.viewport.SetContent(m.content.String())
	m.viewport.GotoBottom()
}

// AppendMarkdown renders and adds markdown content
func (m *AppModel) AppendMarkdown(content string) {
	if m.mdRenderer != nil {
		rendered, err := m.mdRenderer.Render(content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}
	m.AppendContent(content + "\n")
}

// SetStatus updates the status bar
func (m *AppModel) SetStatus(text string, isWorking bool) {
	m.statusText = text
	m.isWorking = isWorking
	if isWorking {
		m.startTime = time.Now()
	}
}

// SetTokenCount updates the token count display
func (m *AppModel) SetTokenCount(count int) {
	m.tokenCount = count
}

// Program message commands for external use
func ContentCmd(content string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return contentMsg{content: content, isError: isError}
	}
}

func StatusCmd(text string, isWorking bool) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{text: text, isWorking: isWorking}
	}
}

func DoneCmd() tea.Cmd {
	return func() tea.Msg {
		return doneMsg{}
	}
}
