package ollama

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("Provider should not be nil")
	}

	if p.Name() != "ollama" {
		t.Errorf("Name mismatch: got %s, want ollama", p.Name())
	}

	if p.baseURL != "http://localhost:11434" {
		t.Errorf("BaseURL mismatch: got %s", p.baseURL)
	}

	if p.model != "qwen3" {
		t.Errorf("Default model mismatch: got %s, want qwen3", p.model)
	}
}

func TestWithOptions(t *testing.T) {
	p := New(
		WithBaseURL("http://192.168.1.100:11434"),
		WithModel("mistral"),
	)

	if p.baseURL != "http://192.168.1.100:11434" {
		t.Errorf("BaseURL mismatch: got %s", p.baseURL)
	}

	if p.model != "mistral" {
		t.Errorf("Model mismatch: got %s", p.model)
	}
}

func TestConvertMessage(t *testing.T) {
	p := New()

	// Test user message
	userMsg := provider.Message{
		Role: provider.RoleUser,
		Content: []provider.ContentBlock{
			&provider.TextBlock{Text: "Hello"},
		},
	}

	result := p.convertMessage(userMsg)
	if result.Role != "user" {
		t.Errorf("Role mismatch: got %s, want user", result.Role)
	}
	if result.Content != "Hello" {
		t.Errorf("Content mismatch: got %s", result.Content)
	}

	// Test assistant message with tool call
	assistantMsg := provider.Message{
		Role: provider.RoleAssistant,
		Content: []provider.ContentBlock{
			&provider.ToolUseBlock{
				ID:    "call_0_get_weather",
				Name:  "get_weather",
				Input: map[string]interface{}{"city": "Tokyo"},
			},
		},
	}

	result = p.convertMessage(assistantMsg)
	if result.Role != "assistant" {
		t.Errorf("Role mismatch: got %s, want assistant", result.Role)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Tool name mismatch: got %s", result.ToolCalls[0].Function.Name)
	}
}

func TestConvertToolResult(t *testing.T) {
	p := New()

	// Test tool result message
	toolResultMsg := provider.Message{
		Role: provider.RoleUser, // Tool results come as user role in our system
		Content: []provider.ContentBlock{
			&provider.ToolResultBlock{
				ToolUseID: "call_0_get_weather",
				Content:   "25°C, sunny",
			},
		},
	}

	result := p.convertMessage(toolResultMsg)
	if result.Role != "tool" {
		t.Errorf("Role mismatch: got %s, want tool", result.Role)
	}
	if result.ToolName != "get_weather" {
		t.Errorf("ToolName mismatch: got %s, want get_weather", result.ToolName)
	}
	if result.Content != "25°C, sunny" {
		t.Errorf("Content mismatch: got %s", result.Content)
	}
}

func TestCleanSchemaForOllama(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"city": {"type": "string"}
		},
		"additionalProperties": false,
		"required": ["city"]
	}`)

	cleaned := cleanSchemaForOllama(schema)

	var result map[string]interface{}
	if err := json.Unmarshal(cleaned, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, exists := result["additionalProperties"]; exists {
		t.Error("additionalProperties should be removed")
	}

	if _, exists := result["required"]; !exists {
		t.Error("required should be preserved")
	}
}

func TestResolveModel(t *testing.T) {
	p := New()

	tests := []struct {
		input    string
		expected string
	}{
		{"", "qwen3"},
		{"ollama", "qwen3"},
		{"llama", "llama3.3"},
		{"qwen", "qwen3"},
		{"mistral", "mistral"},
		{"qwen3:32b", "qwen3:32b"},
	}

	for _, tt := range tests {
		result := p.resolveModel(tt.input)
		if result != tt.expected {
			t.Errorf("resolveModel(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestStreamReaderEventSequence(t *testing.T) {
	// Simulate Ollama streaming response
	streamData := `{"model":"qwen3","message":{"content":"Hello"},"done":false}
{"model":"qwen3","message":{"content":" world"},"done":false}
{"model":"qwen3","message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}
`
	ctx := context.Background()
	body := io.NopCloser(strings.NewReader(streamData))
	reader := newStreamReader(ctx, body, "qwen3")

	// Event 1: MessageStartEvent
	ev1, err := reader.Recv()
	if err != nil {
		t.Fatalf("Event 1 error: %v", err)
	}
	if _, ok := ev1.(*provider.MessageStartEvent); !ok {
		t.Errorf("Event 1: expected MessageStartEvent, got %T", ev1)
	}

	// Event 2: ContentBlockStartEvent (triggered by pending content)
	ev2, err := reader.Recv()
	if err != nil {
		t.Fatalf("Event 2 error: %v", err)
	}
	if _, ok := ev2.(*provider.ContentBlockStartEvent); !ok {
		t.Errorf("Event 2: expected ContentBlockStartEvent, got %T", ev2)
	}

	// Event 3: ContentBlockDeltaEvent with "Hello"
	ev3, err := reader.Recv()
	if err != nil {
		t.Fatalf("Event 3 error: %v", err)
	}
	if delta, ok := ev3.(*provider.ContentBlockDeltaEvent); ok {
		if td, ok := delta.Delta.(*provider.TextDelta); ok {
			if td.Text != "Hello" {
				t.Errorf("Event 3: expected 'Hello', got '%s'", td.Text)
			}
		} else {
			t.Errorf("Event 3: expected TextDelta, got %T", delta.Delta)
		}
	} else {
		t.Errorf("Event 3: expected ContentBlockDeltaEvent, got %T", ev3)
	}

	// Event 4: ContentBlockDeltaEvent with " world"
	ev4, err := reader.Recv()
	if err != nil {
		t.Fatalf("Event 4 error: %v", err)
	}
	if delta, ok := ev4.(*provider.ContentBlockDeltaEvent); ok {
		if td, ok := delta.Delta.(*provider.TextDelta); ok {
			if td.Text != " world" {
				t.Errorf("Event 4: expected ' world', got '%s'", td.Text)
			}
		}
	} else {
		t.Errorf("Event 4: expected ContentBlockDeltaEvent, got %T", ev4)
	}

	// Event 5: MessageDeltaEvent (done)
	ev5, err := reader.Recv()
	if err != nil {
		t.Fatalf("Event 5 error: %v", err)
	}
	if delta, ok := ev5.(*provider.MessageDeltaEvent); ok {
		if delta.Delta.StopReason != provider.StopReasonEndTurn {
			t.Errorf("Event 5: expected StopReasonEndTurn, got %v", delta.Delta.StopReason)
		}
		if delta.Usage.InputTokens != 10 || delta.Usage.OutputTokens != 5 {
			t.Errorf("Event 5: unexpected usage: %+v", delta.Usage)
		}
	} else {
		t.Errorf("Event 5: expected MessageDeltaEvent, got %T", ev5)
	}

	// Event 6: EOF
	_, err = reader.Recv()
	if err != io.EOF {
		t.Errorf("Event 6: expected EOF, got %v", err)
	}
}
