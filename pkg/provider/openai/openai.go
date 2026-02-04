// Package openai implements the OpenAI API provider
package openai

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
	defaultBaseURL = "https://api.openai.com/v1"
)

// Provider implements the OpenAI API provider
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	orgID   string
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

// WithOrganization sets the organization ID
func WithOrganization(orgID string) Option {
	return func(p *Provider) {
		p.orgID = orgID
	}
}

// New creates a new OpenAI provider
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

// NewWithSessionToken creates a provider using ChatGPT session token
// Note: Session token auth is experimental and may not work reliably
func NewWithSessionToken(sessionToken string) *Provider {
	// For now, use the session token as API key
	// In the future, we could implement proper ChatGPT web API support
	return &Provider{
		apiKey:  sessionToken,
		baseURL: defaultBaseURL,
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// NewWithAccessToken creates a provider using ChatGPT access token
// This uses the ChatGPT backend API with the access token from session
func NewWithAccessToken(accessToken string) *Provider {
	return &Provider{
		apiKey:  accessToken,
		baseURL: "https://chatgpt.com/backend-api", // ChatGPT backend API
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openai"
}

// SupportedModels returns the list of supported models
func (p *Provider) SupportedModels() []string {
	return []string{
		"gpt-5.2",
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1",
		"o1-mini",
		"o1-preview",
		"o3",
		"o3-mini",
	}
}

// SupportsFeature checks if a feature is supported
func (p *Provider) SupportsFeature(feature provider.Feature) bool {
	switch feature {
	case provider.FeatureStreaming,
		provider.FeatureToolUse,
		provider.FeatureVision:
		return true
	case provider.FeatureThinking:
		return false // OpenAI doesn't have extended thinking like Claude
	default:
		return false
	}
}

// OpenAI API types
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []openaiTool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role       string            `json:"role"`
	Content    interface{}       `json:"content"` // string or []contentPart
	ToolCalls  []openaiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type openaiTool struct {
	Type     string           `json:"type"`
	Function openaiFunction   `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openaiToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
	Delta        *openaiDelta  `json:"delta,omitempty"`
}

type openaiDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CreateMessage performs a non-streaming chat completion
func (p *Provider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	openaiReq := p.convertRequest(req)
	openaiReq.Stream = false

	body, err := json.Marshal(openaiReq)
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

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertResponse(&openaiResp), nil
}

// CreateMessageStream performs a streaming chat completion
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	openaiReq := p.convertRequest(req)
	openaiReq.Stream = true

	body, err := json.Marshal(openaiReq)
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

// setHeaders sets the required headers for OpenAI API
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}
}

// convertRequest converts a provider.Request to OpenAI format
func (p *Provider) convertRequest(req *provider.Request) *openaiRequest {
	messages := make([]openaiMessage, 0)

	// Add system message if present
	if len(req.System) > 0 {
		var systemContent string
		for _, block := range req.System {
			if tb, ok := block.(*provider.TextBlock); ok {
				systemContent += tb.Text
			}
		}
		if systemContent != "" {
			messages = append(messages, openaiMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiMsg := p.convertMessage(msg)
		messages = append(messages, openaiMsg...)
	}

	// Convert tools
	tools := make([]openaiTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, openaiTool{
			Type: "function",
			Function: openaiFunction{
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

	return &openaiRequest{
		Model:       p.resolveModel(req.Model),
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
	}
}

// convertMessage converts a provider.Message to OpenAI format
func (p *Provider) convertMessage(msg provider.Message) []openaiMessage {
	var messages []openaiMessage

	switch msg.Role {
	case provider.RoleUser:
		// Check if it contains tool results
		var toolResults []openaiMessage
		var contentParts []contentPart

		for _, block := range msg.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				contentParts = append(contentParts, contentPart{
					Type: "text",
					Text: b.Text,
				})
			case *provider.ImageBlock:
				contentParts = append(contentParts, contentPart{
					Type: "image_url",
					ImageURL: &imageURL{
						URL: fmt.Sprintf("data:%s;base64,%s", b.Source.MediaType, b.Source.Data),
					},
				})
			case *provider.ToolResultBlock:
				toolResults = append(toolResults, openaiMessage{
					Role:       "tool",
					Content:    b.Content,
					ToolCallID: b.ToolUseID,
				})
			}
		}

		// Add tool results first (OpenAI requires them in separate messages)
		messages = append(messages, toolResults...)

		// Add user content if any
		if len(contentParts) > 0 {
			if len(contentParts) == 1 && contentParts[0].Type == "text" {
				messages = append(messages, openaiMessage{
					Role:    "user",
					Content: contentParts[0].Text,
				})
			} else {
				messages = append(messages, openaiMessage{
					Role:    "user",
					Content: contentParts,
				})
			}
		}

	case provider.RoleAssistant:
		assistantMsg := openaiMessage{
			Role: "assistant",
		}

		var textContent string
		var toolCalls []openaiToolCall

		for _, block := range msg.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				textContent += b.Text
			case *provider.ToolUseBlock:
				inputJSON, _ := json.Marshal(b.Input)
				toolCalls = append(toolCalls, openaiToolCall{
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
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}

		messages = append(messages, assistantMsg)
	}

	return messages
}

// convertResponse converts an OpenAI response to provider.Response
func (p *Provider) convertResponse(resp *openaiResponse) *provider.Response {
	if len(resp.Choices) == 0 {
		return &provider.Response{
			ID:    resp.ID,
			Model: resp.Model,
		}
	}

	choice := resp.Choices[0]
	content := make([]provider.ContentBlock, 0)

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
		"gpt4":     "gpt-4o",
		"gpt4o":    "gpt-4o",
		"gpt4mini": "gpt-4o-mini",
	}
	if resolved, ok := aliases[model]; ok {
		return resolved
	}
	return model
}

// SSE Stream Reader for OpenAI
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

		var chunk openaiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Store message ID
		if chunk.ID != "" {
			r.msgID = chunk.ID
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Handle delta content
		if choice.Delta != nil {
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
