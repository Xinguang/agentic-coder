// Package claude implements the Claude AI provider
package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

const (
	defaultBaseURL = "https://api.anthropic.com/v1"
	apiVersion     = "2023-06-01"
)

// AuthType represents the authentication type
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
)

// Provider implements the Claude AI provider
type Provider struct {
	apiKey      string
	accessToken string
	authType    AuthType
	baseURL     string
	client      *http.Client
	betaHeader  string
}

// Option is a function that configures the Provider
type Option func(*Provider)

// WithBaseURL sets a custom base URL
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		p.baseURL = url
	}
}

// WithTimeout sets a custom timeout
func WithTimeout(timeout time.Duration) Option {
	return func(p *Provider) {
		p.client.Timeout = timeout
	}
}

// WithBeta enables beta features
func WithBeta(features ...string) Option {
	return func(p *Provider) {
		p.betaHeader = strings.Join(features, ",")
	}
}

// WithOAuthToken sets OAuth access token instead of API key
func WithOAuthToken(accessToken string) Option {
	return func(p *Provider) {
		p.accessToken = accessToken
		p.authType = AuthTypeOAuth
	}
}

// New creates a new Claude provider
func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:   apiKey,
		authType: AuthTypeAPIKey,
		baseURL:  defaultBaseURL,
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// NewWithOAuth creates a new Claude provider with OAuth token
func NewWithOAuth(accessToken string, opts ...Option) *Provider {
	p := &Provider{
		accessToken: accessToken,
		authType:    AuthTypeOAuth,
		baseURL:     defaultBaseURL,
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "claude"
}

// SupportedModels returns the list of supported models
func (p *Provider) SupportedModels() []string {
	return []string{
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251101",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
	}
}

// SupportsFeature checks if a feature is supported
func (p *Provider) SupportsFeature(feature provider.Feature) bool {
	switch feature {
	case provider.FeatureStreaming,
		provider.FeatureToolUse,
		provider.FeatureVision,
		provider.FeatureThinking,
		provider.FeatureCaching:
		return true
	default:
		return false
	}
}

// claudeRequest is the request format for Claude API
type claudeRequest struct {
	Model     string                   `json:"model"`
	Messages  []claudeMessage          `json:"messages"`
	MaxTokens int                      `json:"max_tokens"`
	System    interface{}              `json:"system,omitempty"` // string or []ContentBlock
	Tools     []claudeTool             `json:"tools,omitempty"`
	Stream    bool                     `json:"stream,omitempty"`
	Thinking  *provider.ThinkingConfig `json:"thinking,omitempty"`
}

type claudeMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

type claudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// claudeResponse is the response format from Claude API
type claudeResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Model        string               `json:"model"`
	Content      []claudeContentBlock `json:"content"`
	StopReason   string               `json:"stop_reason"`
	StopSequence string               `json:"stop_sequence"`
	Usage        provider.Usage       `json:"usage"`
}

type claudeContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Thinking  string                 `json:"thinking,omitempty"`
	Signature string                 `json:"signature,omitempty"`
}

// CreateMessage performs a non-streaming chat completion
func (p *Provider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	claudeReq := p.convertRequest(req)
	claudeReq.Stream = false

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertResponse(&claudeResp), nil
}

// CreateMessageStream performs a streaming chat completion
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	claudeReq := p.convertRequest(req)
	claudeReq.Stream = true

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return newSSEStreamReader(ctx, resp.Body), nil
}

// sseStreamReader implements provider.StreamReader for Claude SSE streams
type sseStreamReader struct {
	ctx     context.Context
	body    io.ReadCloser
	scanner *bufio.Scanner
	event   string
	data    strings.Builder
}

func newSSEStreamReader(ctx context.Context, body io.ReadCloser) *sseStreamReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB buffer
	return &sseStreamReader{
		ctx:     ctx,
		body:    body,
		scanner: scanner,
	}
}

func (r *sseStreamReader) Recv() (provider.StreamingEvent, error) {
	for r.scanner.Scan() {
		select {
		case <-r.ctx.Done():
			return nil, r.ctx.Err()
		default:
		}

		line := r.scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			r.event = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			r.data.WriteString(strings.TrimPrefix(line, "data: "))
			continue
		}

		if line == "" && r.data.Len() > 0 {
			event := r.parseSSEEvent(r.event, r.data.String())
			r.data.Reset()
			r.event = ""
			if event != nil {
				return event, nil
			}
		}
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func (r *sseStreamReader) Close() error {
	return r.body.Close()
}

func (r *sseStreamReader) parseSSEEvent(event string, data string) provider.StreamingEvent {
	switch event {
	case "message_start":
		var msg struct {
			Message claudeResponse `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &msg); err == nil {
			return &provider.MessageStartEvent{
				Message: &provider.Response{
					ID:    msg.Message.ID,
					Model: msg.Message.Model,
				},
			}
		}

	case "content_block_start":
		var block struct {
			Index        int                `json:"index"`
			ContentBlock claudeContentBlock `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &block); err == nil {
			var cb provider.ContentBlock
			switch block.ContentBlock.Type {
			case "text":
				cb = &provider.TextBlock{Text: block.ContentBlock.Text}
			case "tool_use":
				cb = &provider.ToolUseBlock{
					ID:   block.ContentBlock.ID,
					Name: block.ContentBlock.Name,
				}
			case "thinking":
				cb = &provider.ThinkingBlock{Thinking: block.ContentBlock.Thinking}
			}
			return &provider.ContentBlockStartEvent{
				Index:        block.Index,
				ContentBlock: cb,
			}
		}

	case "content_block_delta":
		var delta struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
				Thinking    string `json:"thinking,omitempty"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &delta); err == nil {
			var db provider.DeltaBlock
			switch delta.Delta.Type {
			case "text_delta":
				db = &provider.TextDelta{Text: delta.Delta.Text}
			case "input_json_delta":
				db = &provider.InputJSONDelta{PartialJSON: delta.Delta.PartialJSON}
			case "thinking_delta":
				db = &provider.ThinkingDelta{Thinking: delta.Delta.Thinking}
			}
			return &provider.ContentBlockDeltaEvent{
				Index: delta.Index,
				Delta: db,
			}
		}

	case "content_block_stop":
		var stop struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal([]byte(data), &stop); err == nil {
			return &provider.ContentBlockStopEvent{Index: stop.Index}
		}

	case "message_delta":
		var delta struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage provider.Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &delta); err == nil {
			return &provider.MessageDeltaEvent{
				Delta: &provider.MessageDelta{
					StopReason: provider.StopReason(delta.Delta.StopReason),
				},
				Usage: &delta.Usage,
			}
		}

	case "message_stop":
		return &provider.MessageStopEvent{}
	}

	return nil
}

// setHeaders sets the required headers for Claude API
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	// Set authentication header based on auth type
	if p.authType == AuthTypeOAuth && p.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.accessToken)
	} else if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}

	req.Header.Set("anthropic-version", apiVersion)
	if p.betaHeader != "" {
		req.Header.Set("anthropic-beta", p.betaHeader)
	}
}

// convertRequest converts a provider.Request to Claude format
func (p *Provider) convertRequest(req *provider.Request) *claudeRequest {
	messages := make([]claudeMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		content := make([]interface{}, 0, len(msg.Content))
		for _, block := range msg.Content {
			content = append(content, p.convertContentBlock(block))
		}
		messages = append(messages, claudeMessage{
			Role:    string(msg.Role),
			Content: content,
		})
	}

	tools := make([]claudeTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 32000
	}

	// Convert system prompt
	var system interface{}
	if len(req.System) > 0 {
		// Check if it's a simple text block
		if len(req.System) == 1 {
			if tb, ok := req.System[0].(*provider.TextBlock); ok {
				system = tb.Text
			}
		}
		// Otherwise use array format
		if system == nil {
			systemBlocks := make([]interface{}, 0, len(req.System))
			for _, block := range req.System {
				systemBlocks = append(systemBlocks, p.convertContentBlock(block))
			}
			system = systemBlocks
		}
	}

	return &claudeRequest{
		Model:     provider.ResolveModel(req.Model),
		Messages:  messages,
		MaxTokens: maxTokens,
		System:    system,
		Tools:     tools,
		Thinking:  req.Thinking,
	}
}

// convertContentBlock converts a provider.ContentBlock to Claude format
func (p *Provider) convertContentBlock(block provider.ContentBlock) interface{} {
	switch b := block.(type) {
	case *provider.TextBlock:
		return map[string]interface{}{
			"type": "text",
			"text": b.Text,
		}
	case *provider.ImageBlock:
		return map[string]interface{}{
			"type":   "image",
			"source": b.Source,
		}
	case *provider.ToolUseBlock:
		return map[string]interface{}{
			"type":  "tool_use",
			"id":    b.ID,
			"name":  b.Name,
			"input": b.Input,
		}
	case *provider.ToolResultBlock:
		return map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": b.ToolUseID,
			"content":     b.Content,
			"is_error":    b.IsError,
		}
	default:
		return nil
	}
}

// convertResponse converts a Claude response to provider.Response
func (p *Provider) convertResponse(resp *claudeResponse) *provider.Response {
	content := make([]provider.ContentBlock, 0, len(resp.Content))

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content = append(content, &provider.TextBlock{Text: block.Text})
		case "tool_use":
			content = append(content, &provider.ToolUseBlock{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		case "thinking":
			content = append(content, &provider.ThinkingBlock{
				Thinking:  block.Thinking,
				Signature: block.Signature,
			})
		}
	}

	return &provider.Response{
		ID:         resp.ID,
		Model:      resp.Model,
		Content:    content,
		StopReason: provider.StopReason(resp.StopReason),
		Usage:      resp.Usage,
	}
}

