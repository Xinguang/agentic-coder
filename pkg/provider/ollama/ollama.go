// Package ollama implements the Ollama local API provider
package ollama

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
	defaultBaseURL = "http://localhost:11434"
)

// Provider implements the Ollama API provider
type Provider struct {
	baseURL string
	client  *http.Client
	model   string
}

// Option is a function that configures the Provider
type Option func(*Provider)

// WithBaseURL sets a custom base URL
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		p.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithTimeout sets a custom timeout
func WithTimeout(timeout time.Duration) Option {
	return func(p *Provider) {
		p.client.Timeout = timeout
	}
}

// WithModel sets the default model
func WithModel(model string) Option {
	return func(p *Provider) {
		p.model = model
	}
}

// New creates a new Ollama provider
func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL: defaultBaseURL,
		model:   "qwen3",
		client: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for local models
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "ollama"
}

// SupportedModels returns the list of supported models
func (p *Provider) SupportedModels() []string {
	return []string{
		"gpt-oss:20b",
		"qwen3",
		"qwen3:32b",
		"llama3.3",
		"codellama",
		"mistral",
		"mixtral",
		"phi4",
		"deepseek-r1",
		"gemma3",
	}
}

// SupportsFeature checks if a feature is supported
func (p *Provider) SupportsFeature(feature provider.Feature) bool {
	switch feature {
	case provider.FeatureStreaming,
		provider.FeatureToolUse:
		return true
	case provider.FeatureVision:
		return true // Some models support vision
	default:
		return false
	}
}

// Ollama API types
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	// For tool response messages
	ToolName string `json:"tool_name,omitempty"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"function"`
}

type ollamaOptions struct {
	NumPredict  int     `json:"num_predict,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

type ollamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason,omitempty"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

// CreateMessage performs a non-streaming chat completion
func (p *Provider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	ollamaReq := p.convertRequest(req)
	ollamaReq.Stream = false

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertResponse(&ollamaResp), nil
}

// CreateMessageStream performs a streaming chat completion
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	ollamaReq := p.convertRequest(req)
	ollamaReq.Stream = true

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return newStreamReader(ctx, resp.Body, ollamaReq.Model), nil
}

// convertRequest converts a provider.Request to Ollama format
func (p *Provider) convertRequest(req *provider.Request) *ollamaRequest {
	ollamaReq := &ollamaRequest{
		Model:    p.resolveModel(req.Model),
		Messages: make([]ollamaMessage, 0),
	}

	// Add system message
	if len(req.System) > 0 {
		var systemText string
		for _, block := range req.System {
			if tb, ok := block.(*provider.TextBlock); ok {
				systemText += tb.Text
			}
		}
		if systemText != "" {
			ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		ollamaMsg := p.convertMessage(msg)
		if ollamaMsg != nil {
			ollamaReq.Messages = append(ollamaReq.Messages, *ollamaMsg)
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]ollamaTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			ollamaReq.Tools = append(ollamaReq.Tools, ollamaTool{
				Type: "function",
				Function: ollamaToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  cleanSchemaForOllama(t.InputSchema),
				},
			})
		}
	}

	// Options
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	ollamaReq.Options = &ollamaOptions{
		NumPredict:  maxTokens,
		Temperature: req.Temperature,
	}

	return ollamaReq
}

// convertMessage converts a provider.Message to Ollama message
func (p *Provider) convertMessage(msg provider.Message) *ollamaMessage {
	ollamaMsg := &ollamaMessage{}

	// Map roles
	switch msg.Role {
	case provider.RoleUser:
		ollamaMsg.Role = "user"
	case provider.RoleAssistant:
		ollamaMsg.Role = "assistant"
	default:
		return nil
	}

	var textContent strings.Builder
	var images []string
	var toolCalls []ollamaToolCall

	for _, block := range msg.Content {
		switch b := block.(type) {
		case *provider.TextBlock:
			textContent.WriteString(b.Text)

		case *provider.ImageBlock:
			if b.Source.Type == "base64" {
				images = append(images, b.Source.Data)
			}

		case *provider.ToolUseBlock:
			toolCalls = append(toolCalls, ollamaToolCall{
				Function: struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				}{
					Name:      b.Name,
					Arguments: b.Input,
				},
			})

		case *provider.ToolResultBlock:
			// Tool results use role "tool" with tool_name field
			ollamaMsg.Role = "tool"
			// Extract function name from tool ID (format: "call_N_functionName")
			toolName := b.ToolUseID
			if parts := strings.Split(b.ToolUseID, "_"); len(parts) >= 3 {
				toolName = strings.Join(parts[2:], "_") // Get everything after "call_N_"
			}
			ollamaMsg.ToolName = toolName
			textContent.WriteString(b.Content)
		}
	}

	ollamaMsg.Content = textContent.String()
	if len(images) > 0 {
		ollamaMsg.Images = images
	}
	if len(toolCalls) > 0 {
		ollamaMsg.ToolCalls = toolCalls
	}

	return ollamaMsg
}

// convertResponse converts an Ollama response to provider.Response
func (p *Provider) convertResponse(resp *ollamaResponse) *provider.Response {
	providerResp := &provider.Response{
		Model:   resp.Model,
		Content: make([]provider.ContentBlock, 0),
	}

	// Add text content
	if resp.Message.Content != "" {
		providerResp.Content = append(providerResp.Content, &provider.TextBlock{
			Text: resp.Message.Content,
		})
	}

	// Add tool calls
	for i, tc := range resp.Message.ToolCalls {
		// Generate a unique ID for the tool call
		toolID := fmt.Sprintf("call_%d_%s", i, tc.Function.Name)

		providerResp.Content = append(providerResp.Content, &provider.ToolUseBlock{
			ID:    toolID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	// Map stop reason
	if len(resp.Message.ToolCalls) > 0 {
		providerResp.StopReason = provider.StopReasonToolUse
	} else {
		switch resp.DoneReason {
		case "stop":
			providerResp.StopReason = provider.StopReasonEndTurn
		case "length":
			providerResp.StopReason = provider.StopReasonMaxTokens
		default:
			if resp.Done {
				providerResp.StopReason = provider.StopReasonEndTurn
			}
		}
	}

	// Usage
	providerResp.Usage = provider.Usage{
		InputTokens:  resp.PromptEvalCount,
		OutputTokens: resp.EvalCount,
	}

	return providerResp
}

// resolveModel maps model aliases to actual model names
func (p *Provider) resolveModel(model string) string {
	if model == "" {
		return p.model
	}

	aliases := map[string]string{
		"ollama":    "qwen3",
		"llama":     "llama3.3",
		"qwen":      "qwen3",
		"codellama": "codellama",
		"deepseek":  "deepseek-r1",
		"gemma":     "gemma3",
		"phi":       "phi4",
	}
	if resolved, ok := aliases[model]; ok {
		return resolved
	}
	return model
}

// cleanSchemaForOllama removes unsupported fields from JSON schema
func cleanSchemaForOllama(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 {
		return schema
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return schema
	}

	cleanSchemaMap(schemaMap)

	cleaned, err := json.Marshal(schemaMap)
	if err != nil {
		return schema
	}
	return cleaned
}

// cleanSchemaMap recursively removes unsupported fields
func cleanSchemaMap(m map[string]interface{}) {
	// Remove unsupported fields
	delete(m, "additionalProperties")
	delete(m, "$schema")
	delete(m, "definitions")
	delete(m, "$ref")

	// Recursively clean nested objects
	if props, ok := m["properties"].(map[string]interface{}); ok {
		for _, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				cleanSchemaMap(propMap)
			}
		}
	}

	// Clean items for arrays
	if items, ok := m["items"].(map[string]interface{}); ok {
		cleanSchemaMap(items)
	}
}

// Stream Reader for Ollama
type streamReader struct {
	ctx            context.Context
	body           io.ReadCloser
	scanner        *bufio.Scanner
	model          string
	started        bool
	blockStarted   bool
	pendingContent string // Content from first response that needs to be sent
}

func newStreamReader(ctx context.Context, body io.ReadCloser, model string) *streamReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	return &streamReader{
		ctx:     ctx,
		body:    body,
		scanner: scanner,
		model:   model,
	}
}

func (r *streamReader) Recv() (provider.StreamingEvent, error) {
	// If we have pending content from first response, send block start then content
	if r.pendingContent != "" {
		if !r.blockStarted {
			r.blockStarted = true
			return &provider.ContentBlockStartEvent{
				Index:        0,
				ContentBlock: &provider.TextBlock{Text: ""},
			}, nil
		}
		content := r.pendingContent
		r.pendingContent = ""
		return &provider.ContentBlockDeltaEvent{
			Index: 0,
			Delta: &provider.TextDelta{Text: content},
		}, nil
	}

	for r.scanner.Scan() {
		select {
		case <-r.ctx.Done():
			return nil, r.ctx.Err()
		default:
		}

		line := r.scanner.Text()
		if line == "" {
			continue
		}

		var resp ollamaResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}

		// First response: send MessageStartEvent, save content for later
		if !r.started {
			r.started = true
			if resp.Message.Content != "" {
				r.pendingContent = resp.Message.Content
			}
			return &provider.MessageStartEvent{
				Message: &provider.Response{
					Model: resp.Model,
				},
			}, nil
		}

		// Handle tool calls
		for i, tc := range resp.Message.ToolCalls {
			toolID := fmt.Sprintf("call_%d_%s", i, tc.Function.Name)

			return &provider.ContentBlockStartEvent{
				Index: i,
				ContentBlock: &provider.ToolUseBlock{
					ID:    toolID,
					Name:  tc.Function.Name,
					Input: tc.Function.Arguments,
				},
			}, nil
		}

		// Handle text content
		if resp.Message.Content != "" {
			// Send ContentBlockStartEvent first if not sent
			if !r.blockStarted {
				r.blockStarted = true
				r.pendingContent = resp.Message.Content
				return &provider.ContentBlockStartEvent{
					Index:        0,
					ContentBlock: &provider.TextBlock{Text: ""},
				}, nil
			}

			return &provider.ContentBlockDeltaEvent{
				Index: 0,
				Delta: &provider.TextDelta{Text: resp.Message.Content},
			}, nil
		}

		// Check for completion
		if resp.Done {
			var stopReason provider.StopReason
			if len(resp.Message.ToolCalls) > 0 {
				stopReason = provider.StopReasonToolUse
			} else {
				switch resp.DoneReason {
				case "stop":
					stopReason = provider.StopReasonEndTurn
				case "length":
					stopReason = provider.StopReasonMaxTokens
				default:
					stopReason = provider.StopReasonEndTurn
				}
			}

			return &provider.MessageDeltaEvent{
				Delta: &provider.MessageDelta{StopReason: stopReason},
				Usage: &provider.Usage{
					InputTokens:  resp.PromptEvalCount,
					OutputTokens: resp.EvalCount,
				},
			}, nil
		}
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func (r *streamReader) Close() error {
	return r.body.Close()
}
