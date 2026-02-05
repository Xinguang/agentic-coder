package tui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// Runner manages the TUI and engine interaction
type Runner struct {
	engine  *engine.Engine
	program *tea.Program
	model   *Model

	// Cancellation
	ctx        context.Context
	cancel     context.CancelFunc
	cancelLock sync.Mutex

	// Message channel for async updates
	msgChan       chan tea.Msg
	droppedMsgCnt atomic.Uint64
}

// NewRunner creates a new TUI runner
func NewRunner(eng *engine.Engine, cfg Config) *Runner {
	r := &Runner{
		engine:  eng,
		msgChan: make(chan tea.Msg, 500), // Increased from 100 to 500
	}

	// Set up the submit callback
	cfg.OnSubmit = r.handleSubmit

	m := New(cfg)
	r.model = &m

	return r
}

// Run starts the TUI
func (r *Runner) Run() error {
	// Print welcome BEFORE starting bubbletea (avoids render conflicts)
	r.model.PrintWelcome()

	r.program = tea.NewProgram(
		*r.model,
		// Note: Not using WithAltScreen() to allow text selection and native scrolling
	)

	// Start message forwarder
	go r.forwardMessages()

	_, err := r.program.Run()
	return err
}

// forwardMessages forwards messages from channel to program
func (r *Runner) forwardMessages() {
	for msg := range r.msgChan {
		if r.program != nil {
			r.program.Send(msg)
		}
	}
}

// handleSubmit is called when user submits input
func (r *Runner) handleSubmit(input string, opID uint64) tea.Cmd {
	return func() tea.Msg {
		// Cancel any existing operation
		r.cancelLock.Lock()
		if r.cancel != nil {
			r.cancel()
		}
		// Create new context
		r.ctx, r.cancel = context.WithCancel(context.Background())
		ctx := r.ctx
		r.cancelLock.Unlock()

		// Set up callbacks to forward to TUI
		r.engine.SetCallbacks(&engine.CallbackOptions{
			OnText: func(text string) {
				r.sendMsg(StreamTextMsg{Text: text})
			},
			OnThinking: func(text string) {
				r.sendMsg(StreamThinkingMsg{Text: text})
			},
			OnToolUse: func(name string, params map[string]interface{}) {
				r.sendMsg(ToolUseMsg{Name: name, Params: params})
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
				r.sendMsg(ToolResultMsg{Name: name, Success: success, Summary: summary})
			},
			OnUsage: func(inputTokens, outputTokens int) {
				// Calculate approximate cost for Claude
				// Claude: ~$3/$15 per MTok for input/output (Sonnet 3.5)
				cost := float64(inputTokens)*0.000003 + float64(outputTokens)*0.000015
				r.sendMsg(TokenUpdateMsg{
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
					CostUSD:      cost,
				})
			},
			OnError: func(err error) {
				r.sendMsg(StreamDoneMsg{Error: err, OpID: opID})
			},
		})

		// Run the engine
		err := r.engine.Run(ctx, input)

		// Signal completion
		return StreamDoneMsg{Error: err, OpID: opID}
	}
}

// sendMsg sends a message to the TUI with timeout
func (r *Runner) sendMsg(msg tea.Msg) {
	select {
	case r.msgChan <- msg:
		return
	default:
		// Channel full, try with timeout before dropping
	}

	// Wait up to 100ms for channel to have space
	select {
	case r.msgChan <- msg:
	case <-time.After(100 * time.Millisecond):
		// Still full after timeout, drop message
		cnt := r.droppedMsgCnt.Add(1)
		if cnt%10 == 1 {
			log.Printf("WARNING: Message channel full, dropped %d messages total", cnt)
		}
	}
}

// Interrupt cancels the current operation
func (r *Runner) Interrupt() {
	r.cancelLock.Lock()
	defer r.cancelLock.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
}
