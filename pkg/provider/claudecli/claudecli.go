// Package claudecli implements a provider that calls the local Claude Code CLI
package claudecli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

// Provider implements a provider using local Claude Code CLI
type Provider struct {
	model   string
	cliPath string
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

// New creates a new Claude CLI provider
func New(opts ...Option) *Provider {
	p := &Provider{
		model:   "sonnet",
		cliPath: "claude",
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "claude-cli"
}

// SupportedModels returns supported models
func (p *Provider) SupportedModels() []string {
	return []string{"sonnet", "opus", "haiku"}
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

// CreateMessageStream performs a streaming completion using Claude CLI
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	// Build prompt from messages
	var prompt strings.Builder
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if tb, ok := block.(*provider.TextBlock); ok {
				if msg.Role == provider.RoleUser {
					prompt.WriteString(tb.Text)
					prompt.WriteString("\n")
				}
			}
		}
	}

	// Build command
	args := []string{
		"-p",                             // Print mode (non-interactive)
		"--output-format", "stream-json", // Stream JSON output
		"--verbose",                      // Required for stream-json
		"--model", p.model,
		"--dangerously-skip-permissions", // Skip permission prompts
		prompt.String(),
	}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude cli: %w", err)
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
	cmd      *exec.Cmd
	stdout   io.ReadCloser
	scanner  *bufio.Scanner
	done     bool
	started  bool
	lastText string // Track sent text to calculate delta

	// Event queue for handling multiple events per line
	eventQueue []provider.StreamingEvent

	// Track tool use for correlating results
	lastToolName string
}

func (r *streamReader) Recv() (provider.StreamingEvent, error) {
	if r.done {
		return nil, io.EOF
	}

	// Return queued events first
	if len(r.eventQueue) > 0 {
		event := r.eventQueue[0]
		r.eventQueue = r.eventQueue[1:]
		return event, nil
	}

	// Send MessageStartEvent first
	if !r.started {
		r.started = true
		return &provider.MessageStartEvent{
			Message: &provider.Response{
				ID:      "claude-cli",
				Model:   "claude-cli",
				Content: make([]provider.ContentBlock, 0),
			},
		}, nil
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			continue
		}

		var event cliEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "assistant":
			// Process all content blocks
			for _, block := range event.Message.Content {
				switch block.Type {
				case "text":
					if block.Text != "" {
						// Calculate delta (new text since last)
						fullText := block.Text
						if len(fullText) > len(r.lastText) {
							delta := fullText[len(r.lastText):]
							r.lastText = fullText
							r.eventQueue = append(r.eventQueue, &provider.ContentBlockDeltaEvent{
								Index: 0,
								Delta: &provider.TextDelta{Text: delta},
							})
						}
					}
				case "tool_use":
					// Tool use event - emit ToolInfoEvent
					r.lastToolName = block.Name
					r.eventQueue = append(r.eventQueue, &provider.ToolInfoEvent{
						ID:    block.ID,
						Name:  block.Name,
						Input: block.Input,
					})
				}
			}
			// Return first event from queue
			if len(r.eventQueue) > 0 {
				ev := r.eventQueue[0]
				r.eventQueue = r.eventQueue[1:]
				return ev, nil
			}

		case "user":
			// User messages contain tool results
			for _, block := range event.Message.Content {
				if block.Type == "tool_result" {
					// Tool result event
					r.eventQueue = append(r.eventQueue, &provider.ToolResultInfoEvent{
						ToolUseID: block.ToolUseID,
						Name:      r.lastToolName,
						Content:   block.Content,
						IsError:   block.IsError,
					})
				}
			}
			// Return first event from queue
			if len(r.eventQueue) > 0 {
				ev := r.eventQueue[0]
				r.eventQueue = r.eventQueue[1:]
				return ev, nil
			}

		case "result":
			// Final result - stream complete
			r.done = true
			return &provider.MessageStopEvent{}, nil
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

// cliEvent represents a Claude CLI stream-json event
type cliEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		Content []struct {
			Type   string                 `json:"type"`
			Text   string                 `json:"text,omitempty"`
			ID     string                 `json:"id,omitempty"`
			Name   string                 `json:"name,omitempty"`
			Input  map[string]interface{} `json:"input,omitempty"`
			// For tool_result
			ToolUseID string `json:"tool_use_id,omitempty"`
			Content   string `json:"content,omitempty"`
			IsError   bool   `json:"is_error,omitempty"`
		} `json:"content"`
	} `json:"message"`
	ToolUseResult struct {
		Stdout      string `json:"stdout,omitempty"`
		Stderr      string `json:"stderr,omitempty"`
		Interrupted bool   `json:"interrupted,omitempty"`
	} `json:"tool_use_result,omitempty"`
	Result string `json:"result"`
}
