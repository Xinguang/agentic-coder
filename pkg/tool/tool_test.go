package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// MockTool is a mock tool for testing
type MockTool struct {
	name        string
	description string
	schema      json.RawMessage
	executeFunc func(ctx context.Context, input *Input) (*Output, error)
	validateErr error
}

func (m *MockTool) Name() string                { return m.name }
func (m *MockTool) Description() string         { return m.description }
func (m *MockTool) InputSchema() json.RawMessage { return m.schema }
func (m *MockTool) Validate(input *Input) error { return m.validateErr }
func (m *MockTool) Execute(ctx context.Context, input *Input) (*Output, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return &Output{Content: "mock output"}, nil
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if registry.tools == nil {
		t.Error("tools map not initialized")
	}
	if registry.aliases == nil {
		t.Error("aliases map not initialized")
	}
	if registry.disabled == nil {
		t.Error("disabled map not initialized")
	}
}

func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()

	tool := &MockTool{name: "test_tool", description: "A test tool"}

	// Register tool
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Try to register again - should fail
	err = registry.Register(tool)
	if err == nil {
		t.Error("Expected error when registering duplicate tool")
	}
}

func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()

	tool := &MockTool{name: "test_tool", description: "A test tool"}
	registry.Register(tool)

	// Get existing tool
	retrieved, err := registry.Get("test_tool")
	if err != nil {
		t.Fatalf("Failed to get tool: %v", err)
	}
	if retrieved.Name() != "test_tool" {
		t.Errorf("Expected name 'test_tool', got %s", retrieved.Name())
	}

	// Get non-existing tool
	_, err = registry.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent tool")
	}
}

func TestRegistryAlias(t *testing.T) {
	registry := NewRegistry()

	tool := &MockTool{name: "original_name", description: "A test tool"}
	registry.Register(tool)

	// Register alias
	registry.RegisterAlias("alias_name", "original_name")

	// Get by alias
	retrieved, err := registry.Get("alias_name")
	if err != nil {
		t.Fatalf("Failed to get tool by alias: %v", err)
	}
	if retrieved.Name() != "original_name" {
		t.Errorf("Expected name 'original_name', got %s", retrieved.Name())
	}
}

func TestRegistryDisable(t *testing.T) {
	registry := NewRegistry()

	tool := &MockTool{name: "test_tool", description: "A test tool"}
	registry.Register(tool)

	// Tool should be accessible initially
	_, err := registry.Get("test_tool")
	if err != nil {
		t.Fatalf("Tool should be accessible: %v", err)
	}

	// Disable tool
	registry.Disable("test_tool")

	// Tool should not be accessible
	_, err = registry.Get("test_tool")
	if err == nil {
		t.Error("Expected error for disabled tool")
	}

	// Enable tool
	registry.Enable("test_tool")

	// Tool should be accessible again
	_, err = registry.Get("test_tool")
	if err != nil {
		t.Fatalf("Tool should be accessible after enable: %v", err)
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	tool1 := &MockTool{name: "tool1"}
	tool2 := &MockTool{name: "tool2"}
	tool3 := &MockTool{name: "tool3"}

	registry.Register(tool1)
	registry.Register(tool2)
	registry.Register(tool3)

	// List all
	tools := registry.List()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}

	// Disable one
	registry.Disable("tool2")

	// List should have 2
	tools = registry.List()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools after disable, got %d", len(tools))
	}
}

func TestRegistryNames(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&MockTool{name: "alpha"})
	registry.Register(&MockTool{name: "beta"})

	names := registry.Names()
	if len(names) != 2 {
		t.Errorf("Expected 2 names, got %d", len(names))
	}

	// Check names exist (order not guaranteed)
	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}
	if !nameMap["alpha"] || !nameMap["beta"] {
		t.Error("Missing expected tool names")
	}
}

func TestRegistryToAPITools(t *testing.T) {
	registry := NewRegistry()

	tool := &MockTool{
		name:        "test_tool",
		description: "A test description",
		schema:      json.RawMessage(`{"type": "object"}`),
	}
	registry.Register(tool)

	apiTools := registry.ToAPITools()
	if len(apiTools) != 1 {
		t.Fatalf("Expected 1 API tool, got %d", len(apiTools))
	}

	apiTool := apiTools[0]
	if apiTool.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got %s", apiTool.Name)
	}
	if apiTool.Description != "A test description" {
		t.Errorf("Expected description 'A test description', got %s", apiTool.Description)
	}
}

func TestRegistryFilteredRegistry(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&MockTool{name: "allowed1"})
	registry.Register(&MockTool{name: "allowed2"})
	registry.Register(&MockTool{name: "disallowed1"})

	// Filter with allowed list
	filtered := registry.FilteredRegistry([]string{"allowed1", "allowed2"}, nil)
	if len(filtered.List()) != 2 {
		t.Errorf("Expected 2 tools with allowed filter, got %d", len(filtered.List()))
	}

	// Filter with disallowed list
	filtered = registry.FilteredRegistry(nil, []string{"disallowed1"})
	if len(filtered.List()) != 2 {
		t.Errorf("Expected 2 tools with disallowed filter, got %d", len(filtered.List()))
	}
}

func TestParamsTo(t *testing.T) {
	type TestParams struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	params := map[string]interface{}{
		"name":  "test",
		"count": 42,
	}

	result, err := ParamsTo[TestParams](params)
	if err != nil {
		t.Fatalf("ParamsTo failed: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("Expected name 'test', got %s", result.Name)
	}
	if result.Count != 42 {
		t.Errorf("Expected count 42, got %d", result.Count)
	}
}

func TestParamsToInvalid(t *testing.T) {
	type TestParams struct {
		Count int `json:"count"`
	}

	// Try with invalid type
	params := map[string]interface{}{
		"count": "not a number",
	}

	_, err := ParamsTo[TestParams](params)
	if err == nil {
		t.Error("Expected error for invalid params")
	}
}

func TestExecutionContext(t *testing.T) {
	ctx := &ExecutionContext{
		SessionID:      "session_123",
		CWD:            "/test/path",
		ProjectPath:    "/project",
		PermissionMode: PermissionModeDefault,
	}

	if ctx.SessionID != "session_123" {
		t.Errorf("Expected SessionID 'session_123', got %s", ctx.SessionID)
	}
	if ctx.CWD != "/test/path" {
		t.Errorf("Expected CWD '/test/path', got %s", ctx.CWD)
	}
}

func TestPermissionModeConstants(t *testing.T) {
	modes := []PermissionMode{
		PermissionModeDefault,
		PermissionModePlan,
		PermissionModeAcceptEdits,
		PermissionModeDontAsk,
		PermissionModeBypassPermissions,
	}

	// Ensure all are unique
	seen := make(map[PermissionMode]bool)
	for _, m := range modes {
		if seen[m] {
			t.Errorf("Duplicate permission mode: %s", m)
		}
		seen[m] = true
	}
}

func TestOutput(t *testing.T) {
	output := &Output{
		Content:  "Test content",
		IsError:  false,
		Metadata: map[string]interface{}{"key": "value"},
	}

	if output.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got %s", output.Content)
	}
	if output.IsError {
		t.Error("Expected IsError to be false")
	}
	if metadata, ok := output.Metadata.(map[string]interface{}); ok {
		if metadata["key"] != "value" {
			t.Error("Metadata not set correctly")
		}
	} else {
		t.Error("Metadata is not a map")
	}
}

func TestInput(t *testing.T) {
	input := &Input{
		ID:     "input_123",
		Name:   "test_tool",
		Params: map[string]interface{}{"param": "value"},
		Context: &ExecutionContext{
			CWD: "/test",
		},
	}

	if input.ID != "input_123" {
		t.Errorf("Expected ID 'input_123', got %s", input.ID)
	}
	if input.Name != "test_tool" {
		t.Errorf("Expected Name 'test_tool', got %s", input.Name)
	}
	if input.Params["param"] != "value" {
		t.Error("Params not set correctly")
	}
	if input.Context.CWD != "/test" {
		t.Error("Context CWD not set correctly")
	}
}
