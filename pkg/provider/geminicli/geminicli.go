// Package geminicli implements a provider that calls the local Gemini CLI
package geminicli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

// Provider implements a provider using local Gemini CLI
type Provider struct {
	model      string
	cliPath    string
	yoloMode   bool   // Auto approve all actions
	sandbox    bool   // Run in sandbox mode
	systemPrompt string
}

// Option configures the Provider
type Option func(*Provider)

// WithModel sets the model
func WithModel(model string) Option {
	return func(p *Provider) {
		p.model = model
	}
}

// WithCLIPath sets custom CLI path
func WithCLIPath(path string) Option {
	return func(p *Provider) {
		p.cliPath = path
	}
}

// WithYoloMode enables auto-approve all actions
func WithYoloMode(enabled bool) Option {
	return func(p *Provider) {
		p.yoloMode = enabled
	}
}

// WithSandbox enables sandbox mode
func WithSandbox(enabled bool) Option {
	return func(p *Provider) {
		p.sandbox = enabled
	}
}

// WithSystemPrompt sets the system prompt
func WithSystemPrompt(prompt string) Option {
	return func(p *Provider) {
		p.systemPrompt = prompt
	}
}

// New creates a new Gemini CLI provider
func New(opts ...Option) *Provider {
	p := &Provider{
		model:    "", // empty means auto (gemini-3)
		cliPath:  "gemini",
		yoloMode: true, // Default to yolo mode for agentic use
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gemini-cli"
}

// SupportedModels returns supported models
func (p *Provider) SupportedModels() []string {
	return []string{"gemini-3", "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"}
}

// SupportsFeature checks if a feature is supported
func (p *Provider) SupportsFeature(feature provider.Feature) bool {
	switch feature {
	case provider.FeatureStreaming, provider.FeatureToolUse:
		return true
	default:
		return false
	}
}

// CreateMessage performs a non-streaming completion
func (p *Provider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	reader, err := p.CreateMessageStream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var fullText strings.Builder
	for {
		event, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if delta, ok := event.(*provider.ContentBlockDeltaEvent); ok {
			if td, ok := delta.Delta.(*provider.TextDelta); ok {
				fullText.WriteString(td.Text)
			}
		}
	}

	return &provider.Response{
		Content:    []provider.ContentBlock{&provider.TextBlock{Text: fullText.String()}},
		StopReason: provider.StopReasonEndTurn,
	}, nil
}

// CreateMessageStream performs a streaming completion using Gemini CLI
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	// Build prompt from the last user message
	var prompt string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg.Role == provider.RoleUser {
			for _, block := range msg.Content {
				if tb, ok := block.(*provider.TextBlock); ok {
					prompt = tb.Text
					break
				}
			}
			break
		}
	}

	if prompt == "" {
		return nil, fmt.Errorf("no user message found")
	}

	// Build command arguments
	args := []string{
		"-o", "stream-json", // Stream JSON output
	}

	if p.yoloMode {
		args = append(args, "-y") // Auto approve all actions
	}

	if p.sandbox {
		args = append(args, "-s") // Sandbox mode
	}

	if p.model != "" {
		args = append(args, "-m", p.model)
	}

	// Add the prompt as positional argument
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, p.cliPath, args...)

	// Redirect stderr to discard (contains startup logs)
	cmd.Stderr = os.Stderr // or io.Discard if you want to hide all stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gemini cli: %w", err)
	}

	// Create scanner with larger buffer for long JSON lines
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max

	return &streamReader{
		cmd:     cmd,
		stdout:  stdout,
		scanner: scanner,
		done:    false,
	}, nil
}

// streamReader implements provider.StreamReader
type streamReader struct {
	cmd       *exec.Cmd
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	done      bool
	started   bool
	blockStarted bool
	lastText  string
	sessionID string
	model     string
}

func (r *streamReader) Recv() (provider.StreamingEvent, error) {
	if r.done {
		return nil, io.EOF
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			continue
		}

		// Skip non-JSON lines (startup logs)
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var event cliEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "init":
			// Session started
			r.sessionID = event.SessionID
			r.model = event.Model
			if !r.started {
				r.started = true
				return &provider.MessageStartEvent{
					Message: &provider.Response{
						ID:      r.sessionID,
						Model:   r.model,
						Content: make([]provider.ContentBlock, 0),
					},
				}, nil
			}

		case "message":
			// Only process assistant messages
			if event.Role != "assistant" {
				continue
			}

			// Send ContentBlockStartEvent if not started
			if !r.blockStarted {
				r.blockStarted = true
				return &provider.ContentBlockStartEvent{
					Index:        0,
					ContentBlock: &provider.TextBlock{},
				}, nil
			}

			// Handle delta content
			if event.Content != "" {
				// Gemini CLI sends full content with delta:true
				// We need to compute the actual delta
				fullText := event.Content
				if len(fullText) > len(r.lastText) {
					delta := fullText[len(r.lastText):]
					r.lastText = fullText
					return &provider.ContentBlockDeltaEvent{
						Index: 0,
						Delta: &provider.TextDelta{Text: delta},
					}, nil
				} else if fullText != r.lastText {
					// Content changed completely (shouldn't happen often)
					r.lastText = fullText
					return &provider.ContentBlockDeltaEvent{
						Index: 0,
						Delta: &provider.TextDelta{Text: fullText},
					}, nil
				}
			}

		case "result":
			// Turn complete
			r.done = true
			return &provider.MessageDeltaEvent{
				Delta: &provider.MessageDelta{
					StopReason: provider.StopReasonEndTurn,
				},
				Usage: &provider.Usage{
					InputTokens:  event.Stats.InputTokens,
					OutputTokens: event.Stats.OutputTokens,
				},
			}, nil
		}
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	r.done = true
	return nil, io.EOF
}

func (r *streamReader) Close() error {
	r.stdout.Close()
	return r.cmd.Wait()
}

// cliEvent represents a Gemini CLI JSON event
type cliEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Delta     bool   `json:"delta,omitempty"`
	Status    string `json:"status,omitempty"`
	Stats     struct {
		TotalTokens  int `json:"total_tokens"`
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		DurationMS   int `json:"duration_ms"`
		ToolCalls    int `json:"tool_calls"`
	} `json:"stats,omitempty"`
}
