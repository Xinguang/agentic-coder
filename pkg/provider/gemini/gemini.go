// Package gemini implements the Google Gemini API provider
package gemini

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
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// Provider implements the Google Gemini API provider
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

// New creates a new Gemini provider
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
	return "gemini"
}

// SupportedModels returns the list of supported models
func (p *Provider) SupportedModels() []string {
	return []string{
		"gemini-2.0-flash-exp",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"gemini-1.5-flash-8b",
		"gemini-1.0-pro",
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
		return true // Gemini 2.0 supports thinking
	default:
		return false
	}
}

// Gemini API types
type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent        `json:"systemInstruction,omitempty"`
	Tools            []geminiTool           `json:"tools,omitempty"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text           string              `json:"text,omitempty"`
	InlineData     *geminiInlineData   `json:"inlineData,omitempty"`
	FunctionCall   *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string `json:"name"`
	Response struct {
		Content string `json:"content"`
	} `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

type geminiResponse struct {
	Candidates     []geminiCandidate `json:"candidates"`
	UsageMetadata  *geminiUsage      `json:"usageMetadata,omitempty"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason,omitempty"`
	} `json:"promptFeedback,omitempty"`
}

type geminiCandidate struct {
	Content       geminiContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// CreateMessage performs a non-streaming chat completion
func (p *Provider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	model := p.resolveModel(req.Model)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, model, p.apiKey)

	geminiReq := p.convertRequest(req)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertResponse(&geminiResp, model), nil
}

// CreateMessageStream performs a streaming chat completion
func (p *Provider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	model := p.resolveModel(req.Model)
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", p.baseURL, model, p.apiKey)

	geminiReq := p.convertRequest(req)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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

	return newSSEStreamReader(ctx, resp.Body, model), nil
}

// convertRequest converts a provider.Request to Gemini format
func (p *Provider) convertRequest(req *provider.Request) *geminiRequest {
	geminiReq := &geminiRequest{
		Contents: make([]geminiContent, 0),
	}

	// Add system instruction
	if len(req.System) > 0 {
		var systemText string
		for _, block := range req.System {
			if tb, ok := block.(*provider.TextBlock); ok {
				systemText += tb.Text
			}
		}
		if systemText != "" {
			geminiReq.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: systemText}},
			}
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		content := p.convertMessage(msg)
		if content != nil {
			geminiReq.Contents = append(geminiReq.Contents, *content)
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		funcDecls := make([]geminiFunctionDecl, 0, len(req.Tools))
		for _, t := range req.Tools {
			// Clean schema for Gemini compatibility
			cleanedSchema := cleanSchemaForGemini(t.InputSchema)
			funcDecls = append(funcDecls, geminiFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  cleanedSchema,
			})
		}
		geminiReq.Tools = []geminiTool{{FunctionDeclarations: funcDecls}}
	}

	// Generation config
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	geminiReq.GenerationConfig = &geminiGenerationConfig{
		MaxOutputTokens: maxTokens,
		Temperature:     req.Temperature,
	}

	return geminiReq
}

// convertMessage converts a provider.Message to Gemini content
func (p *Provider) convertMessage(msg provider.Message) *geminiContent {
	content := &geminiContent{
		Parts: make([]geminiPart, 0),
	}

	// Map roles
	switch msg.Role {
	case provider.RoleUser:
		content.Role = "user"
	case provider.RoleAssistant:
		content.Role = "model"
	default:
		return nil
	}

	for _, block := range msg.Content {
		switch b := block.(type) {
		case *provider.TextBlock:
			content.Parts = append(content.Parts, geminiPart{Text: b.Text})

		case *provider.ImageBlock:
			content.Parts = append(content.Parts, geminiPart{
				InlineData: &geminiInlineData{
					MimeType: b.Source.MediaType,
					Data:     b.Source.Data,
				},
			})

		case *provider.ToolUseBlock:
			content.Parts = append(content.Parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					Name: b.Name,
					Args: b.Input,
				},
			})

		case *provider.ToolResultBlock:
			content.Parts = append(content.Parts, geminiPart{
				FunctionResponse: &geminiFunctionResponse{
					Name: b.ToolUseID, // Gemini uses function name, not ID
					Response: struct {
						Content string `json:"content"`
					}{Content: b.Content},
				},
			})
		}
	}

	if len(content.Parts) == 0 {
		return nil
	}

	return content
}

// convertResponse converts a Gemini response to provider.Response
func (p *Provider) convertResponse(resp *geminiResponse, model string) *provider.Response {
	providerResp := &provider.Response{
		Model:   model,
		Content: make([]provider.ContentBlock, 0),
	}

	if len(resp.Candidates) == 0 {
		return providerResp
	}

	candidate := resp.Candidates[0]

	// Convert parts to content blocks
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			providerResp.Content = append(providerResp.Content, &provider.TextBlock{
				Text: part.Text,
			})
		}

		if part.FunctionCall != nil {
			providerResp.Content = append(providerResp.Content, &provider.ToolUseBlock{
				ID:    part.FunctionCall.Name, // Gemini doesn't have tool IDs
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		}
	}

	// Map finish reason
	switch candidate.FinishReason {
	case "STOP":
		providerResp.StopReason = provider.StopReasonEndTurn
	case "MAX_TOKENS":
		providerResp.StopReason = provider.StopReasonMaxTokens
	case "SAFETY":
		providerResp.StopReason = provider.StopReasonStop
	default:
		providerResp.StopReason = provider.StopReasonEndTurn
	}

	// Usage
	if resp.UsageMetadata != nil {
		providerResp.Usage = provider.Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	return providerResp
}

// cleanSchemaForGemini removes unsupported fields from JSON schema
func cleanSchemaForGemini(schema json.RawMessage) json.RawMessage {
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

	// Clean anyOf, oneOf, allOf
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if arr, ok := m[key].([]interface{}); ok {
			for _, item := range arr {
				if itemMap, ok := item.(map[string]interface{}); ok {
					cleanSchemaMap(itemMap)
				}
			}
		}
	}
}

// resolveModel maps model aliases to actual model IDs
func (p *Provider) resolveModel(model string) string {
	aliases := map[string]string{
		"gemini":      "gemini-1.5-pro",
		"gemini-pro":  "gemini-1.5-pro",
		"gemini-flash": "gemini-1.5-flash",
		"gemini2":     "gemini-2.0-flash-exp",
	}
	if resolved, ok := aliases[model]; ok {
		return resolved
	}
	return model
}

// SSE Stream Reader for Gemini
type sseStreamReader struct {
	ctx     context.Context
	body    io.ReadCloser
	scanner *bufio.Scanner
	model   string
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

		var resp geminiResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		candidate := resp.Candidates[0]

		// Process parts
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				return &provider.ContentBlockDeltaEvent{
					Index: 0,
					Delta: &provider.TextDelta{Text: part.Text},
				}, nil
			}

			if part.FunctionCall != nil {
				return &provider.ContentBlockStartEvent{
					Index: 0,
					ContentBlock: &provider.ToolUseBlock{
						ID:    part.FunctionCall.Name,
						Name:  part.FunctionCall.Name,
						Input: part.FunctionCall.Args,
					},
				}, nil
			}
		}

		// Check for finish
		if candidate.FinishReason != "" {
			var stopReason provider.StopReason
			switch candidate.FinishReason {
			case "STOP":
				stopReason = provider.StopReasonEndTurn
			case "MAX_TOKENS":
				stopReason = provider.StopReasonMaxTokens
			default:
				stopReason = provider.StopReasonEndTurn
			}

			return &provider.MessageDeltaEvent{
				Delta: &provider.MessageDelta{StopReason: stopReason},
				Usage: &provider.Usage{
					InputTokens:  resp.UsageMetadata.PromptTokenCount,
					OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
				},
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
