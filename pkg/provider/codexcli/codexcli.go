// Package codexcli implements a provider that calls the local Codex CLI
package codexcli

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

// Provider implements a provider using local Codex CLI
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

// New creates a new Codex CLI provider
func New(opts ...Option) *Provider {
	p := &Provider{
		model:   "o3-mini",
		cliPath: "codex",
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "codex-cli"
}

// SupportedModels returns supported models
func (p *Provider) SupportedModels() []string {
	return []string{"o3-mini", "o3", "o4-mini", "gpt-4o"}
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

// CreateMessageStream performs a streaming completion using Codex CLI
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
		"exec",
		"--json", // JSONL output
		"--dangerously-bypass-approvals-and-sandbox",
		"-m", p.model,
		prompt.String(),
	}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex cli: %w", err)
	}

	return &streamReader{
		cmd:     cmd,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
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
	lastText string
}

func (r *streamReader) Recv() (provider.StreamingEvent, error) {
	if r.done {
		return nil, io.EOF
	}

	// Send MessageStartEvent first
	if !r.started {
		r.started = true
		return &provider.MessageStartEvent{
			Message: &provider.Response{
				ID:      "codex-cli",
				Model:   "codex-cli",
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
		case "message":
			// Extract text content
			if event.Message.Role == "assistant" {
				for _, block := range event.Message.Content {
					if block.Type == "text" && block.Text != "" {
						fullText := block.Text
						if len(fullText) > len(r.lastText) {
							delta := fullText[len(r.lastText):]
							r.lastText = fullText
							return &provider.ContentBlockDeltaEvent{
								Index: 0,
								Delta: &provider.TextDelta{Text: delta},
							}, nil
						}
					}
				}
			}
		case "task_complete", "agent_finished":
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

// cliEvent represents a Codex CLI JSON event
type cliEvent struct {
	Type    string `json:"type"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}
