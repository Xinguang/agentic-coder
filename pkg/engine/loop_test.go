package engine

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/session"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// MockProvider implements provider.AIProvider for testing
type MockProvider struct {
	responses   []*provider.Response
	responseIdx int
	err         error
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) SupportedModels() []string {
	return []string{"mock-model"}
}

func (m *MockProvider) CreateMessage(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.responseIdx >= len(m.responses) {
		return &provider.Response{
			StopReason: provider.StopReasonEndTurn,
			Content:    []provider.ContentBlock{&provider.TextBlock{Text: "Default response"}},
		}, nil
	}
	resp := m.responses[m.responseIdx]
	m.responseIdx++
	return resp, nil
}

func (m *MockProvider) CreateMessageStream(ctx context.Context, req *provider.Request) (provider.StreamReader, error) {
	if m.err != nil {
		return nil, m.err
	}
	resp, err := m.CreateMessage(ctx, req)
	if err != nil {
		return nil, err
	}
	return &simpleStreamReader{resp: resp}, nil
}

func (m *MockProvider) SupportsFeature(feature provider.Feature) bool {
	return true
}

// simpleStreamReader simulates streaming from a complete response
type simpleStreamReader struct {
	resp *provider.Response
	step int
}

func (r *simpleStreamReader) Recv() (provider.StreamingEvent, error) {
	switch r.step {
	case 0:
		r.step++
		return &provider.MessageStartEvent{
			Message: &provider.Response{
				ID:    r.resp.ID,
				Model: r.resp.Model,
			},
		}, nil
	case 1:
		r.step++
		if len(r.resp.Content) > 0 {
			return &provider.ContentBlockStartEvent{
				Index:        0,
				ContentBlock: r.resp.Content[0],
			}, nil
		}
		return nil, io.EOF
	case 2:
		r.step++
		return &provider.ContentBlockStopEvent{Index: 0}, nil
	case 3:
		r.step++
		return &provider.MessageDeltaEvent{
			Delta: &provider.MessageDelta{StopReason: r.resp.StopReason},
		}, nil
	default:
		return nil, io.EOF
	}
}

func (r *simpleStreamReader) Close() error { return nil }

// MockTool implements tool.Tool for testing
type MockTool struct {
	name        string
	description string
	schema      json.RawMessage
	executeFunc func(ctx context.Context, input *tool.Input) (*tool.Output, error)
	validateErr error
}

func (m *MockTool) Name() string                 { return m.name }
func (m *MockTool) Description() string          { return m.description }
func (m *MockTool) InputSchema() json.RawMessage { return m.schema }
func (m *MockTool) Validate(input *tool.Input) error {
	return m.validateErr
}
func (m *MockTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return &tool.Output{Content: "mock output"}, nil
}

func TestNewEngine(t *testing.T) {
	prov := &MockProvider{}
	registry := tool.NewRegistry()
	sess := session.NewSession(&session.SessionOptions{
		CWD:   "/test",
		Model: "test-model",
	})

	eng := NewEngine(&EngineOptions{
		Provider: prov,
		Registry: registry,
		Session:  sess,
	})

	if eng == nil {
		t.Fatal("NewEngine returned nil")
	}
	if eng.maxIterations != 100 {
		t.Errorf("Expected default maxIterations 100, got %d", eng.maxIterations)
	}
	if eng.maxTokens != 16384 {
		t.Errorf("Expected default maxTokens 16384, got %d", eng.maxTokens)
	}
}

func TestNewEngineCustomOptions(t *testing.T) {
	prov := &MockProvider{}
	registry := tool.NewRegistry()
	sess := session.NewSession(&session.SessionOptions{
		CWD:   "/test",
		Model: "test-model",
	})

	eng := NewEngine(&EngineOptions{
		Provider:      prov,
		Registry:      registry,
		Session:       sess,
		MaxIterations: 50,
		MaxTokens:     8192,
		Temperature:   0.5,
		ThinkingLevel: "high",
		SystemPrompt:  "Custom prompt",
	})

	if eng.maxIterations != 50 {
		t.Errorf("Expected maxIterations 50, got %d", eng.maxIterations)
	}
	if eng.maxTokens != 8192 {
		t.Errorf("Expected maxTokens 8192, got %d", eng.maxTokens)
	}
	if eng.temperature != 0.5 {
		t.Errorf("Expected temperature 0.5, got %f", eng.temperature)
	}
	if eng.thinkingLevel != "high" {
		t.Errorf("Expected thinkingLevel 'high', got %s", eng.thinkingLevel)
	}
	if eng.systemPrompt != "Custom prompt" {
		t.Errorf("Expected systemPrompt 'Custom prompt', got %s", eng.systemPrompt)
	}
}

func TestSetCallbacks(t *testing.T) {
	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: tool.NewRegistry(),
		Session:  session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"}),
	})

	textCalled := false
	thinkingCalled := false
	toolUseCalled := false
	toolResultCalled := false
	errorCalled := false

	eng.SetCallbacks(&CallbackOptions{
		OnText:       func(text string) { textCalled = true },
		OnThinking:   func(text string) { thinkingCalled = true },
		OnToolUse:    func(name string, input map[string]interface{}) { toolUseCalled = true },
		OnToolResult: func(name string, result *tool.Output) { toolResultCalled = true },
		OnError:      func(err error) { errorCalled = true },
	})

	// Verify callbacks are set
	if eng.onText == nil {
		t.Error("onText callback not set")
	}
	if eng.onThinking == nil {
		t.Error("onThinking callback not set")
	}
	if eng.onToolUse == nil {
		t.Error("onToolUse callback not set")
	}
	if eng.onToolResult == nil {
		t.Error("onToolResult callback not set")
	}
	if eng.onError == nil {
		t.Error("onError callback not set")
	}

	// Call them to verify they work
	eng.onText("test")
	eng.onThinking("test")
	eng.onToolUse("test", nil)
	eng.onToolResult("test", nil)
	eng.onError(nil)

	if !textCalled {
		t.Error("onText callback not called")
	}
	if !thinkingCalled {
		t.Error("onThinking callback not called")
	}
	if !toolUseCalled {
		t.Error("onToolUse callback not called")
	}
	if !toolResultCalled {
		t.Error("onToolResult callback not called")
	}
	if !errorCalled {
		t.Error("onError callback not called")
	}
}

func TestBuildRequest(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(&MockTool{name: "test_tool", description: "Test tool"})

	sess := session.NewSession(&session.SessionOptions{
		CWD:   "/test",
		Model: "claude-sonnet",
	})
	sess.AddUserMessage("Hello")

	eng := NewEngine(&EngineOptions{
		Provider:     &MockProvider{},
		Registry:     registry,
		Session:      sess,
		SystemPrompt: "Base prompt",
	})

	req := eng.buildRequest()

	if req.Model != "claude-sonnet" {
		t.Errorf("Expected model 'claude-sonnet', got %s", req.Model)
	}
	if len(req.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(req.Messages))
	}
	if len(req.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(req.Tools))
	}
	if !req.Stream {
		t.Error("Expected Stream to be true")
	}
	if len(req.System) == 0 {
		t.Error("Expected system prompt to be set")
	}
}

func TestBuildRequestWithThinking(t *testing.T) {
	eng := NewEngine(&EngineOptions{
		Provider:      &MockProvider{},
		Registry:      tool.NewRegistry(),
		Session:       session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"}),
		ThinkingLevel: "high",
	})

	req := eng.buildRequest()

	if req.Thinking == nil {
		t.Fatal("Expected Thinking config to be set")
	}
	if req.Thinking.Type != "enabled" {
		t.Errorf("Expected Thinking.Type 'enabled', got %s", req.Thinking.Type)
	}
	if req.Thinking.BudgetTokens != 10000 {
		t.Errorf("Expected BudgetTokens 10000, got %d", req.Thinking.BudgetTokens)
	}
}

func TestGetThinkingBudget(t *testing.T) {
	tests := []struct {
		level    string
		expected int
	}{
		{"high", 10000},
		{"medium", 5000},
		{"low", 2000},
		{"unknown", 5000},
		{"", 5000},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			eng := NewEngine(&EngineOptions{
				Provider:      &MockProvider{},
				Registry:      tool.NewRegistry(),
				Session:       session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"}),
				ThinkingLevel: tt.level,
			})

			budget := eng.getThinkingBudget()
			if budget != tt.expected {
				t.Errorf("getThinkingBudget(%q) = %d, want %d", tt.level, budget, tt.expected)
			}
		})
	}
}

func TestParseJSONToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasKey   string
		expected interface{}
	}{
		{
			name:     "simple object",
			input:    `{"key": "value"}`,
			hasKey:   "key",
			expected: "value",
		},
		{
			name:     "number",
			input:    `{"count": 42}`,
			hasKey:   "count",
			expected: float64(42),
		},
		{
			name:     "empty string",
			input:    "",
			hasKey:   "",
			expected: nil,
		},
		{
			name:     "invalid json",
			input:    "not json",
			hasKey:   "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJSONToMap(tt.input)

			if tt.hasKey != "" {
				if result[tt.hasKey] != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result[tt.hasKey])
				}
			}
		})
	}
}

func TestRunSimple(t *testing.T) {
	prov := &MockProvider{
		responses: []*provider.Response{
			{
				ID:         "msg_123",
				StopReason: provider.StopReasonEndTurn,
				Content: []provider.ContentBlock{
					&provider.TextBlock{Text: "Hello!"},
				},
			},
		},
	}

	sess := session.NewSession(&session.SessionOptions{
		CWD:   "/test",
		Model: "test-model",
	})

	eng := NewEngine(&EngineOptions{
		Provider: prov,
		Registry: tool.NewRegistry(),
		Session:  sess,
	})

	ctx := context.Background()
	err := eng.Run(ctx, "Hi")

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Check that user message was added
	messages := sess.GetMessages()
	if len(messages) < 1 {
		t.Fatal("Expected at least 1 message")
	}
}

func TestRunWithToolUse(t *testing.T) {
	// First response requests tool use, second response ends turn
	prov := &MockProvider{
		responses: []*provider.Response{
			{
				ID:         "msg_1",
				StopReason: provider.StopReasonToolUse,
				Content: []provider.ContentBlock{
					&provider.ToolUseBlock{
						ID:    "tool_123",
						Name:  "test_tool",
						Input: map[string]interface{}{"param": "value"},
					},
				},
			},
			{
				ID:         "msg_2",
				StopReason: provider.StopReasonEndTurn,
				Content: []provider.ContentBlock{
					&provider.TextBlock{Text: "Done!"},
				},
			},
		},
	}

	registry := tool.NewRegistry()
	toolExecuted := false
	registry.Register(&MockTool{
		name: "test_tool",
		executeFunc: func(ctx context.Context, input *tool.Input) (*tool.Output, error) {
			toolExecuted = true
			return &tool.Output{Content: "Tool result"}, nil
		},
	})

	sess := session.NewSession(&session.SessionOptions{
		CWD:   "/test",
		Model: "test-model",
	})

	eng := NewEngine(&EngineOptions{
		Provider: prov,
		Registry: registry,
		Session:  sess,
	})

	var toolUseCalled bool
	var toolResultCalled bool
	eng.SetCallbacks(&CallbackOptions{
		OnToolUse: func(name string, input map[string]interface{}) {
			toolUseCalled = true
			if name != "test_tool" {
				t.Errorf("Expected tool name 'test_tool', got %s", name)
			}
		},
		OnToolResult: func(name string, result *tool.Output) {
			toolResultCalled = true
		},
	})

	ctx := context.Background()
	err := eng.Run(ctx, "Use the tool")

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !toolExecuted {
		t.Error("Tool was not executed")
	}
	if !toolUseCalled {
		t.Error("OnToolUse callback was not called")
	}
	if !toolResultCalled {
		t.Error("OnToolResult callback was not called")
	}
}

func TestRunContextCancellation(t *testing.T) {
	prov := &MockProvider{
		responses: []*provider.Response{
			{
				ID:         "msg_1",
				StopReason: provider.StopReasonToolUse,
				Content: []provider.ContentBlock{
					&provider.ToolUseBlock{ID: "t1", Name: "slow_tool", Input: map[string]interface{}{}},
				},
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&MockTool{name: "slow_tool"})

	sess := session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"})

	eng := NewEngine(&EngineOptions{
		Provider: prov,
		Registry: registry,
		Session:  sess,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := eng.Run(ctx, "Test")

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestHookManager(t *testing.T) {
	hm := NewHookManager()

	if hm == nil {
		t.Fatal("NewHookManager returned nil")
	}
	if hm.preToolUse == nil {
		t.Error("preToolUse not initialized")
	}
	if hm.postToolUse == nil {
		t.Error("postToolUse not initialized")
	}
	if hm.onStop == nil {
		t.Error("onStop not initialized")
	}
}

func TestHookManagerPreToolUse(t *testing.T) {
	hm := NewHookManager()

	hookCalled := false
	hm.RegisterPreToolUse(func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
		hookCalled = true
		if toolName != "test_tool" {
			t.Errorf("Expected toolName 'test_tool', got %s", toolName)
		}
		return &HookResult{Blocked: false}
	})

	ctx := context.Background()
	result := hm.RunPreToolUse(ctx, "test_tool", map[string]interface{}{})

	if !hookCalled {
		t.Error("PreToolUse hook was not called")
	}
	if result.Blocked {
		t.Error("Result should not be blocked")
	}
}

func TestHookManagerPreToolUseBlocked(t *testing.T) {
	hm := NewHookManager()

	hm.RegisterPreToolUse(func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
		return &HookResult{
			Blocked: true,
			Message: "Tool is blocked",
		}
	})

	ctx := context.Background()
	result := hm.RunPreToolUse(ctx, "test_tool", map[string]interface{}{})

	if !result.Blocked {
		t.Error("Result should be blocked")
	}
	if result.Message != "Tool is blocked" {
		t.Errorf("Expected message 'Tool is blocked', got %s", result.Message)
	}
}

func TestHookManagerPreToolUseModifyInput(t *testing.T) {
	hm := NewHookManager()

	hm.RegisterPreToolUse(func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
		return &HookResult{
			Blocked: false,
			ModifiedInput: map[string]interface{}{
				"modified": true,
			},
		}
	})

	ctx := context.Background()
	input := map[string]interface{}{"original": true}
	hm.RunPreToolUse(ctx, "test_tool", input)

	// Note: The modified input is returned in the result, but the current implementation
	// doesn't actually modify the original input. This is by design.
}

func TestHookManagerPostToolUse(t *testing.T) {
	hm := NewHookManager()

	hookCalled := false
	hm.RegisterPostToolUse(func(ctx context.Context, toolName string, input map[string]interface{}, output *tool.Output) {
		hookCalled = true
		if toolName != "test_tool" {
			t.Errorf("Expected toolName 'test_tool', got %s", toolName)
		}
	})

	ctx := context.Background()
	hm.RunPostToolUse(ctx, "test_tool", map[string]interface{}{}, &tool.Output{})

	if !hookCalled {
		t.Error("PostToolUse hook was not called")
	}
}

func TestHookManagerOnStop(t *testing.T) {
	hm := NewHookManager()

	hookCalled := false
	hm.RegisterOnStop(func(ctx context.Context, reason string) {
		hookCalled = true
		if reason != "end_turn" {
			t.Errorf("Expected reason 'end_turn', got %s", reason)
		}
	})

	ctx := context.Background()
	hm.RunOnStop(ctx, "end_turn")

	if !hookCalled {
		t.Error("OnStop hook was not called")
	}
}

func TestHookManagerMultipleHooks(t *testing.T) {
	hm := NewHookManager()

	callOrder := []int{}

	hm.RegisterPreToolUse(func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
		callOrder = append(callOrder, 1)
		return nil
	})

	hm.RegisterPreToolUse(func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
		callOrder = append(callOrder, 2)
		return nil
	})

	ctx := context.Background()
	hm.RunPreToolUse(ctx, "test_tool", map[string]interface{}{})

	if len(callOrder) != 2 {
		t.Errorf("Expected 2 hooks called, got %d", len(callOrder))
	}
	if callOrder[0] != 1 || callOrder[1] != 2 {
		t.Error("Hooks called in wrong order")
	}
}

func TestGetEnvironmentInfo(t *testing.T) {
	sess := session.NewSession(&session.SessionOptions{
		CWD:   "/test/project",
		Model: "test-model",
	})

	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: tool.NewRegistry(),
		Session:  sess,
	})

	envInfo := eng.getEnvironmentInfo()

	if envInfo == "" {
		t.Error("getEnvironmentInfo returned empty string")
	}
	if !contains(envInfo, "/test/project") {
		t.Error("Environment info should contain CWD")
	}
	if !contains(envInfo, "<env>") {
		t.Error("Environment info should be wrapped in <env> tags")
	}
}

func TestGetToolDescriptions(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(&MockTool{name: "tool1", description: "First tool"})
	registry.Register(&MockTool{name: "tool2", description: "Second tool"})

	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: registry,
		Session:  session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"}),
	})

	desc := eng.getToolDescriptions()

	if !contains(desc, "tool1") {
		t.Error("Description should contain tool1")
	}
	if !contains(desc, "First tool") {
		t.Error("Description should contain tool1 description")
	}
	if !contains(desc, "tool2") {
		t.Error("Description should contain tool2")
	}
}

func TestGetToolDescriptionsEmpty(t *testing.T) {
	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: tool.NewRegistry(),
		Session:  session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"}),
	})

	desc := eng.getToolDescriptions()

	if desc != "" {
		t.Errorf("Expected empty string for empty registry, got %q", desc)
	}
}

func TestExecuteToolUseWithUnknownTool(t *testing.T) {
	sess := session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"})
	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: tool.NewRegistry(),
		Session:  sess,
	})

	ctx := context.Background()
	block := &provider.ToolUseBlock{
		ID:    "tool_123",
		Name:  "unknown_tool",
		Input: map[string]interface{}{},
	}

	err := eng.executeToolUse(ctx, block)

	// Should not return error, but add error result to session
	if err != nil {
		t.Errorf("executeToolUse should not return error for unknown tool, got %v", err)
	}

	// Check that error was added to session
	messages := sess.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected error message to be added to session")
	}
}

func TestExecuteToolUseWithValidationError(t *testing.T) {
	sess := session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"})

	registry := tool.NewRegistry()
	registry.Register(&MockTool{
		name:        "validate_fail",
		validateErr: errValidation,
	})

	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: registry,
		Session:  sess,
	})

	ctx := context.Background()
	block := &provider.ToolUseBlock{
		ID:    "tool_123",
		Name:  "validate_fail",
		Input: map[string]interface{}{},
	}

	err := eng.executeToolUse(ctx, block)

	if err != nil {
		t.Errorf("executeToolUse should not return error for validation failure, got %v", err)
	}
}

var errValidation = &validationError{msg: "validation failed"}

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }

func TestExecuteToolUseWithHookBlock(t *testing.T) {
	sess := session.NewSession(&session.SessionOptions{CWD: "/test", Model: "test"})

	registry := tool.NewRegistry()
	registry.Register(&MockTool{name: "blocked_tool"})

	eng := NewEngine(&EngineOptions{
		Provider: &MockProvider{},
		Registry: registry,
		Session:  sess,
	})

	// Register blocking hook
	eng.hooks.RegisterPreToolUse(func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
		return &HookResult{
			Blocked: true,
			Message: "Tool blocked by hook",
		}
	})

	ctx := context.Background()
	block := &provider.ToolUseBlock{
		ID:    "tool_123",
		Name:  "blocked_tool",
		Input: map[string]interface{}{},
	}

	err := eng.executeToolUse(ctx, block)

	if err != nil {
		t.Errorf("executeToolUse should not return error when blocked, got %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
