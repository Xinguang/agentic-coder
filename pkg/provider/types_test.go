package provider

import (
	"encoding/json"
	"testing"
)

func TestTextBlockMarshalJSON(t *testing.T) {
	block := &TextBlock{Text: "Hello, world!"}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal TextBlock: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["type"] != string(ContentTypeText) {
		t.Errorf("Expected type=%s, got %v", ContentTypeText, result["type"])
	}
	if result["text"] != "Hello, world!" {
		t.Errorf("Expected text='Hello, world!', got %v", result["text"])
	}
}

func TestTextBlockType(t *testing.T) {
	block := &TextBlock{Text: "test"}
	if block.Type() != ContentTypeText {
		t.Errorf("Expected type=%s, got %s", ContentTypeText, block.Type())
	}
}

func TestToolUseBlockMarshalJSON(t *testing.T) {
	block := &ToolUseBlock{
		ID:    "tool_123",
		Name:  "read_file",
		Input: map[string]interface{}{"path": "/test.txt"},
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal ToolUseBlock: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["type"] != string(ContentTypeToolUse) {
		t.Errorf("Expected type=%s, got %v", ContentTypeToolUse, result["type"])
	}
	if result["id"] != "tool_123" {
		t.Errorf("Expected id='tool_123', got %v", result["id"])
	}
	if result["name"] != "read_file" {
		t.Errorf("Expected name='read_file', got %v", result["name"])
	}
}

func TestToolResultBlockMarshalJSON(t *testing.T) {
	block := &ToolResultBlock{
		ToolUseID: "tool_123",
		Content:   "File contents here",
		IsError:   false,
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal ToolResultBlock: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["type"] != string(ContentTypeToolResult) {
		t.Errorf("Expected type=%s, got %v", ContentTypeToolResult, result["type"])
	}
	if result["tool_use_id"] != "tool_123" {
		t.Errorf("Expected tool_use_id='tool_123', got %v", result["tool_use_id"])
	}
}

func TestThinkingBlockMarshalJSON(t *testing.T) {
	block := &ThinkingBlock{
		Thinking:  "Let me think about this...",
		Signature: "sig_abc",
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal ThinkingBlock: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["type"] != string(ContentTypeThinking) {
		t.Errorf("Expected type=%s, got %v", ContentTypeThinking, result["type"])
	}
	if result["thinking"] != "Let me think about this..." {
		t.Errorf("Expected thinking text, got %v", result["thinking"])
	}
}

func TestImageBlockMarshalJSON(t *testing.T) {
	block := &ImageBlock{
		Source: ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "base64data",
		},
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal ImageBlock: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["type"] != string(ContentTypeImage) {
		t.Errorf("Expected type=%s, got %v", ContentTypeImage, result["type"])
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sonnet", "claude-sonnet-4-5-20250929"},
		{"opus", "claude-opus-4-5-20251101"},
		{"haiku", "claude-haiku-4-5-20251101"},
		{"gpt-4", "gpt-4"},
		{"custom-model", "custom-model"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ResolveModel(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveModel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"claude-sonnet-4-5", ProviderTypeClaude},
		{"sonnet", ProviderTypeClaude},
		{"opus", ProviderTypeClaude},
		{"haiku", ProviderTypeClaude},
		{"gpt-4o", ProviderTypeOpenAI},
		{"gpt-4", ProviderTypeOpenAI},
		{"o1", ProviderTypeOpenAI},
		{"gemini-1.5-pro", ProviderTypeGemini},
		{"gemini-flash", ProviderTypeGemini},
		{"deepseek-chat", ProviderTypeDeepSeek},
		{"deepseek-coder", ProviderTypeDeepSeek},
		{"r1", ProviderTypeDeepSeek},
		{"unknown-model", ProviderTypeClaude}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestProviderFactory(t *testing.T) {
	// Create a factory without environment variables
	factory := &ProviderFactory{}

	// Test with no API keys
	available := factory.AvailableProviders()
	if len(available) != 0 {
		t.Errorf("Expected 0 available providers without API keys, got %d", len(available))
	}

	// Set API key
	factory.SetAPIKey(ProviderTypeClaude, "test-key")

	// Check available providers
	available = factory.AvailableProviders()
	if len(available) != 1 {
		t.Errorf("Expected 1 available provider, got %d", len(available))
	}

	// Get API key
	key := factory.GetAPIKey(ProviderTypeClaude)
	if key != "test-key" {
		t.Errorf("Expected 'test-key', got %q", key)
	}
}

func TestStopReasonConstants(t *testing.T) {
	// Ensure constants are defined correctly
	if StopReasonEndTurn != "end_turn" {
		t.Errorf("StopReasonEndTurn = %q, want 'end_turn'", StopReasonEndTurn)
	}
	if StopReasonToolUse != "tool_use" {
		t.Errorf("StopReasonToolUse = %q, want 'tool_use'", StopReasonToolUse)
	}
	if StopReasonMaxTokens != "max_tokens" {
		t.Errorf("StopReasonMaxTokens = %q, want 'max_tokens'", StopReasonMaxTokens)
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleUser != "user" {
		t.Errorf("RoleUser = %q, want 'user'", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q, want 'assistant'", RoleAssistant)
	}
	if RoleSystem != "system" {
		t.Errorf("RoleSystem = %q, want 'system'", RoleSystem)
	}
}

func TestFeatureConstants(t *testing.T) {
	features := []Feature{
		FeatureStreaming,
		FeatureToolUse,
		FeatureVision,
		FeatureThinking,
		FeatureCodeExec,
		FeatureWebSearch,
		FeatureCaching,
	}

	// Just ensure they're unique
	seen := make(map[Feature]bool)
	for _, f := range features {
		if seen[f] {
			t.Errorf("Duplicate feature: %s", f)
		}
		seen[f] = true
	}
}
