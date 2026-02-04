// Package provider defines the AI provider interface and common types
package provider

import (
	"context"
	"encoding/json"
)

// Role represents the role of a message sender
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// ContentType represents the type of content block
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeImage      ContentType = "image"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeThinking   ContentType = "thinking"
)

// ContentBlock is the interface for all content block types
type ContentBlock interface {
	Type() ContentType
	json.Marshaler
}

// TextBlock represents text content
type TextBlock struct {
	Text string `json:"text"`
}

func (t *TextBlock) Type() ContentType { return ContentTypeText }

func (t *TextBlock) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": ContentTypeText,
		"text": t.Text,
	})
}

// ImageBlock represents image content
type ImageBlock struct {
	Source ImageSource `json:"source"`
}

type ImageSource struct {
	Type      string `json:"type"`       // base64, url
	MediaType string `json:"media_type"` // image/png, image/jpeg
	Data      string `json:"data"`
}

func (i *ImageBlock) Type() ContentType { return ContentTypeImage }

func (i *ImageBlock) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":   ContentTypeImage,
		"source": i.Source,
	})
}

// ToolUseBlock represents a tool call from the assistant
type ToolUseBlock struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

func (t *ToolUseBlock) Type() ContentType { return ContentTypeToolUse }

func (t *ToolUseBlock) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":  ContentTypeToolUse,
		"id":    t.ID,
		"name":  t.Name,
		"input": t.Input,
	})
}

// ToolResultBlock represents the result of a tool execution
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

func (t *ToolResultBlock) Type() ContentType { return ContentTypeToolResult }

func (t *ToolResultBlock) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":        ContentTypeToolResult,
		"tool_use_id": t.ToolUseID,
		"content":     t.Content,
		"is_error":    t.IsError,
	})
}

// ThinkingBlock represents the thinking process (Claude extended thinking)
type ThinkingBlock struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature,omitempty"`
}

func (t *ThinkingBlock) Type() ContentType { return ContentTypeThinking }

func (t *ThinkingBlock) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":      ContentTypeThinking,
		"thinking":  t.Thinking,
		"signature": t.Signature,
	})
}

// Message represents a conversation message
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// Tool represents a tool definition for the API
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// StopReason represents why the model stopped generating
type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonStop      StopReason = "stop_sequence"
)

// Usage represents token usage statistics
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// StreamEvent represents the type of streaming event
type StreamEvent string

const (
	StreamEventStart      StreamEvent = "message_start"
	StreamEventDelta      StreamEvent = "content_block_delta"
	StreamEventBlockStart StreamEvent = "content_block_start"
	StreamEventBlockStop  StreamEvent = "content_block_stop"
	StreamEventStop       StreamEvent = "message_stop"
)

// Request represents an AI completion request
type Request struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages"`
	Tools       []Tool         `json:"tools,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	System      []ContentBlock `json:"system,omitempty"`
	Stream      bool           `json:"stream,omitempty"`

	// Extended thinking (Claude specific)
	Thinking *ThinkingConfig `json:"thinking,omitempty"`

	// Provider-specific extra fields
	Extra map[string]interface{} `json:"-"`
}

// ThinkingConfig configures extended thinking
type ThinkingConfig struct {
	Type         string `json:"type"`          // "enabled"
	BudgetTokens int    `json:"budget_tokens"` // max tokens for thinking
}

// Response represents an AI completion response
type Response struct {
	ID         string         `json:"id"`
	Model      string         `json:"model"`
	Content    []ContentBlock `json:"content"`
	StopReason StopReason     `json:"stop_reason"`
	Usage      Usage          `json:"usage"`

	// For streaming responses
	Event StreamEvent `json:"-"`
}

// Feature represents a capability that a provider may support
type Feature string

const (
	FeatureStreaming Feature = "streaming"
	FeatureToolUse   Feature = "tool_use"
	FeatureVision    Feature = "vision"
	FeatureThinking  Feature = "thinking"  // Extended thinking
	FeatureCodeExec  Feature = "code_exec" // Code execution
	FeatureWebSearch Feature = "web_search"
	FeatureCaching   Feature = "caching" // Prompt caching
)

// AIProvider is the interface that all AI providers must implement
type AIProvider interface {
	// Name returns the provider name
	Name() string

	// CreateMessage performs a single chat completion (non-streaming)
	CreateMessage(ctx context.Context, req *Request) (*Response, error)

	// CreateMessageStream performs a streaming chat completion
	CreateMessageStream(ctx context.Context, req *Request) (StreamReader, error)

	// SupportedModels returns the list of supported models
	SupportedModels() []string

	// SupportsFeature checks if a feature is supported
	SupportsFeature(feature Feature) bool
}

// StreamReader is the interface for reading streaming responses
type StreamReader interface {
	Recv() (StreamingEvent, error)
	Close() error
}

// StreamingEvent is the interface for all streaming events
type StreamingEvent interface {
	EventType() StreamEvent
}

// MessageStartEvent indicates the start of a message
type MessageStartEvent struct {
	Message *Response `json:"message"`
}

func (e *MessageStartEvent) EventType() StreamEvent { return StreamEventStart }

// ContentBlockStartEvent indicates the start of a content block
type ContentBlockStartEvent struct {
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

func (e *ContentBlockStartEvent) EventType() StreamEvent { return StreamEventBlockStart }

// ContentBlockDeltaEvent contains a delta update to a content block
type ContentBlockDeltaEvent struct {
	Index int         `json:"index"`
	Delta DeltaBlock  `json:"delta"`
}

func (e *ContentBlockDeltaEvent) EventType() StreamEvent { return StreamEventDelta }

// ContentBlockStopEvent indicates the end of a content block
type ContentBlockStopEvent struct {
	Index int `json:"index"`
}

func (e *ContentBlockStopEvent) EventType() StreamEvent { return StreamEventBlockStop }

// MessageDeltaEvent contains a delta update to the message
type MessageDeltaEvent struct {
	Delta *MessageDelta `json:"delta"`
	Usage *Usage        `json:"usage"`
}

func (e *MessageDeltaEvent) EventType() StreamEvent { return StreamEventStop }

// MessageStopEvent indicates the message is complete
type MessageStopEvent struct{}

func (e *MessageStopEvent) EventType() StreamEvent { return StreamEventStop }

// DeltaBlock is the interface for delta updates
type DeltaBlock interface {
	DeltaType() string
}

// TextDelta represents a text delta
type TextDelta struct {
	Text string `json:"text"`
}

func (d *TextDelta) DeltaType() string { return "text_delta" }

// ThinkingDelta represents a thinking delta
type ThinkingDelta struct {
	Thinking string `json:"thinking"`
}

func (d *ThinkingDelta) DeltaType() string { return "thinking_delta" }

// InputJSONDelta represents a tool input delta
type InputJSONDelta struct {
	PartialJSON string `json:"partial_json"`
}

func (d *InputJSONDelta) DeltaType() string { return "input_json_delta" }

// MessageDelta represents message-level delta information
type MessageDelta struct {
	StopReason StopReason `json:"stop_reason"`
}

// ProviderConfig holds configuration for a provider
type ProviderConfig struct {
	APIKey  string
	BaseURL string
	Timeout int // seconds
}

// ModelAlias maps model aliases to actual model IDs
var ModelAlias = map[string]string{
	// Claude models
	"sonnet": "claude-sonnet-4-5-20250929",
	"opus":   "claude-opus-4-5-20251101",
	"haiku":  "claude-haiku-4-5-20251101",
	// Gemini models
	"gemini":       "gemini-2.0-flash",
	"gemini-pro":   "gemini-1.5-pro",
	"gemini-flash": "gemini-2.0-flash",
	// OpenAI models
	"gpt5":  "gpt-5.2",
	"gpt4":  "gpt-4-turbo",
	"gpt4o": "gpt-4o",
	"o3":    "o3",
	// DeepSeek models
	"deepseek": "deepseek-chat",
	"r1":       "deepseek-reasoner",
	// Ollama models
	"ollama": "qwen3",
	"llama":  "llama3.3",
	"qwen":   "qwen3",
}

// ResolveModel resolves a model alias to the actual model ID
func ResolveModel(model string) string {
	if resolved, ok := ModelAlias[model]; ok {
		return resolved
	}
	return model
}
