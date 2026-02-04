// Package deepseek implements the DeepSeek API provider
// DeepSeek uses an OpenAI-compatible API format
package deepseek

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
	defaultBaseURL = "https://api.deepseek.com/v1"
)

// Provider implements the DeepSeek API provider
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
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

// New creates a new DeepSeek provider
func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
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
	return "deepseek"
}

// SupportedModels returns the list of supported models
func (p *Provider) SupportedModels() []string {
	return []string{
		"deepseek-chat",
		"deepseek-coder",
		"deepseek-reasoner",
	}
}

// SupportsFeature checks if a feature is supported
func (p *Provider) SupportsFeature(feature provider.Feature) bool {
	switch feature {
	case provider.FeatureStreaming,
		provider.FeatureToolUse:
		return true
	case provider.FeatureThinking:
		return true // DeepSeek Reasoner supports thinking
	case provider.FeatureVision:
		return false // DeepSeek doesn't support vision yet
	default:
		return false
	}
}

// DeepSeek API types (OpenAI-compatible)
type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Tools       []deepseekTool    `json:"tools,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
}

type deepseekMessage struct {
	Role         string             `json:"role"`
	Content      interface{}        `json:"content"` // string or null
	ToolCalls    []deepseekToolCall `json:"tool_calls,omitempty"`
	ToolCallID   string             `json:"tool_call_id,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"` // DeepSeek Reasoner
}

type deepseekTool struct {
	Type     string           `json:"type"`
	Function deepseekFunction `json:"function"`
}

type deepseekFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type deepseekToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	Index int `json:"index,omitempty"`
}

type deepseekResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Model   string           `json:"model"`
	Choices []deepseekChoice `json:"choices"`
	Usage   deepseekUsage    `json:"usage"`
}

type deepseekChoice struct {
	Index        int             `json:"index"`
	Message      deepseekMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Delta        *deepseekDelta  `json:"delta,omitempty"`
}

type deepseekDelta struct {
	Role             string             `json:"role,omitempty"`
	Content          string             `json:"content,omitempty"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	ToolCalls        []deepseekToolCall `json:"tool_calls,omitempty"`
}

type deepseekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CreateMessage performs a non-streaming chat completion
func (p *Provider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	deepseekReq := p.convertRequest(req)
	deepseekReq.Stream = false

	body, err := json.Marshal(deepseekReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
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

	var deepseekResp deepseekResponse
	if err := json.NewDecoder(resp.Body).Decode(&deepseekResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertResponse(&deepseekResp), nil
}

// CreateMessageStream performs a streaming chat completion
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	deepseekReq := p.convertRequest(req)
	deepseekReq.Stream = true

	body, err := json.Marshal(deepseekReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
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

	return newSSEStreamReader(ctx, resp.Body, req.Model), nil
}

// setHeaders sets the required headers for DeepSeek API
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

// convertRequest converts a provider.Request to DeepSeek format
func (p *Provider) convertRequest(req *provider.Request) *deepseekRequest {
	messages := make([]deepseekMessage, 0)

	// Add system message if present
	if len(req.System) > 0 {
		var systemContent string
		for _, block := range req.System {
			if tb, ok := block.(*provider.TextBlock); ok {
				systemContent += tb.Text
			}
		}
		if systemContent != "" {
			messages = append(messages, deepseekMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		converted := p.convertMessage(msg)
		messages = append(messages, converted...)
	}

	// Convert tools
	tools := make([]deepseekTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, deepseekTool{
			Type: "function",
			Function: deepseekFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return &deepseekRequest{
		Model:       p.resolveModel(req.Model),
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
	}
}

// convertMessage converts a provider.Message to DeepSeek format
func (p *Provider) convertMessage(msg provider.Message) []deepseekMessage {
	var messages []deepseekMessage

	switch msg.Role {
	case provider.RoleUser:
		// Check for tool results
		var toolResults []deepseekMessage
		var textContent string

		for _, block := range msg.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				textContent += b.Text
			case *provider.ToolResultBlock:
				toolResults = append(toolResults, deepseekMessage{
					Role:       "tool",
					Content:    b.Content,
					ToolCallID: b.ToolUseID,
				})
			}
		}

		// Add tool results first
		messages = append(messages, toolResults...)

		// Add user content
		if textContent != "" {
			messages = append(messages, deepseekMessage{
				Role:    "user",
				Content: textContent,
			})
		}

	case provider.RoleAssistant:
		assistantMsg := deepseekMessage{
			Role: "assistant",
		}

		var textContent string
		var reasoningContent string
		var toolCalls []deepseekToolCall

		for _, block := range msg.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				textContent += b.Text
			case *provider.ThinkingBlock:
				reasoningContent += b.Thinking
			case *provider.ToolUseBlock:
				inputJSON, _ := json.Marshal(b.Input)
				toolCalls = append(toolCalls, deepseekToolCall{
					ID:   b.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      b.Name,
						Arguments: string(inputJSON),
					},
				})
			}
		}

		if textContent != "" {
			assistantMsg.Content = textContent
		}
		if reasoningContent != "" {
			assistantMsg.ReasoningContent = reasoningContent
		}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}

		messages = append(messages, assistantMsg)
	}

	return messages
}

// convertResponse converts a DeepSeek response to provider.Response
func (p *Provider) convertResponse(resp *deepseekResponse) *provider.Response {
	if len(resp.Choices) == 0 {
		return &provider.Response{
			ID:    resp.ID,
			Model: resp.Model,
		}
	}

	choice := resp.Choices[0]
	content := make([]provider.ContentBlock, 0)

	// Add reasoning content (thinking) if present
	if choice.Message.ReasoningContent != "" {
		content = append(content, &provider.ThinkingBlock{
			Thinking: choice.Message.ReasoningContent,
		})
	}

	// Add text content
	if textContent, ok := choice.Message.Content.(string); ok && textContent != "" {
		content = append(content, &provider.TextBlock{Text: textContent})
	}

	// Add tool calls
	for _, tc := range choice.Message.ToolCalls {
		var input map[string]interface{}
		json.Unmarshal([]byte(tc.Function.Arguments), &input)

		content = append(content, &provider.ToolUseBlock{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	// Map finish reason
	var stopReason provider.StopReason
	switch choice.FinishReason {
	case "stop":
		stopReason = provider.StopReasonEndTurn
	case "tool_calls":
		stopReason = provider.StopReasonToolUse
	case "length":
		stopReason = provider.StopReasonMaxTokens
	default:
		stopReason = provider.StopReasonEndTurn
	}

	return &provider.Response{
		ID:         resp.ID,
		Model:      resp.Model,
		Content:    content,
		StopReason: stopReason,
		Usage: provider.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

// resolveModel maps model aliases to actual model IDs
func (p *Provider) resolveModel(model string) string {
	aliases := map[string]string{
		"deepseek":  "deepseek-chat",
		"coder":     "deepseek-coder",
		"reasoner":  "deepseek-reasoner",
		"r1":        "deepseek-reasoner",
	}
	if resolved, ok := aliases[model]; ok {
		return resolved
	}
	return model
}

// SSE Stream Reader for DeepSeek
type sseStreamReader struct {
	ctx     context.Context
	body    io.ReadCloser
	scanner *bufio.Scanner
	model   string
	msgID   string
}

func newSSEStreamReader(ctx context.Context, body io.ReadCloser, model string) *sseStreamReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	return &sseStreamReader{
		ctx:     ctx,
		body:    body,
		scanner: scanner,
		model:   model,
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

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return &provider.MessageStopEvent{}, io.EOF
		}

		var chunk deepseekResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.ID != "" {
			r.msgID = chunk.ID
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		if choice.Delta != nil {
			// Handle reasoning content (thinking)
			if choice.Delta.ReasoningContent != "" {
				return &provider.ContentBlockDeltaEvent{
					Index: 0,
					Delta: &provider.ThinkingDelta{Thinking: choice.Delta.ReasoningContent},
				}, nil
			}

			// Handle regular content
			if choice.Delta.Content != "" {
				return &provider.ContentBlockDeltaEvent{
					Index: 0,
					Delta: &provider.TextDelta{Text: choice.Delta.Content},
				}, nil
			}

			// Handle tool calls
			for _, tc := range choice.Delta.ToolCalls {
				if tc.Function.Name != "" {
					return &provider.ContentBlockStartEvent{
						Index: tc.Index,
						ContentBlock: &provider.ToolUseBlock{
							ID:   tc.ID,
							Name: tc.Function.Name,
						},
					}, nil
				}
				if tc.Function.Arguments != "" {
					return &provider.ContentBlockDeltaEvent{
						Index: tc.Index,
						Delta: &provider.InputJSONDelta{PartialJSON: tc.Function.Arguments},
					}, nil
				}
			}
		}

		// Handle finish
		if choice.FinishReason != "" {
			var stopReason provider.StopReason
			switch choice.FinishReason {
			case "stop":
				stopReason = provider.StopReasonEndTurn
			case "tool_calls":
				stopReason = provider.StopReasonToolUse
			case "length":
				stopReason = provider.StopReasonMaxTokens
			}

			return &provider.MessageDeltaEvent{
				Delta: &provider.MessageDelta{StopReason: stopReason},
			}, nil
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
