package agent

import (
	"testing"
)

func TestExtractJSON_RawJSON(t *testing.T) {
	input := `{"key": "value", "number": 42}`
	result := extractJSON(input)

	if result != input {
		t.Errorf("Expected %s, got %s", input, result)
	}
}

func TestExtractJSON_MarkdownCodeBlock(t *testing.T) {
	input := "Here's the result:\n```json\n{\"key\": \"value\"}\n```\nThat's all."
	expected := `{"key": "value"}`
	result := extractJSON(input)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestExtractJSON_GenericCodeBlock(t *testing.T) {
	input := "Result:\n```\n{\"key\": \"value\"}\n```"
	expected := `{"key": "value"}`
	result := extractJSON(input)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestExtractJSON_EmbeddedJSON(t *testing.T) {
	input := `Some text before {"key": "value"} some text after`
	expected := `{"key": "value"}`
	result := extractJSON(input)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestExtractJSON_NestedJSON(t *testing.T) {
	input := `{"outer": {"inner": "value"}, "array": [1, 2, 3]}`
	result := extractJSON(input)

	if result != input {
		t.Errorf("Expected %s, got %s", input, result)
	}
}

func TestExtractJSON_JSONWithStrings(t *testing.T) {
	input := `{"message": "Hello \"world\"", "path": "C:\\Users"}`
	result := extractJSON(input)

	if result != input {
		t.Errorf("Expected %s, got %s", input, result)
	}
}

func TestExtractJSON_Array(t *testing.T) {
	input := `[{"id": 1}, {"id": 2}]`
	result := extractJSON(input)

	if result != input {
		t.Errorf("Expected %s, got %s", input, result)
	}
}

func TestBaseAgent_Role(t *testing.T) {
	agent := NewBaseAgent(RoleManager, "sonnet", nil)

	if agent.Role() != RoleManager {
		t.Errorf("Expected role Manager, got %s", agent.Role())
	}

	if agent.Model() != "sonnet" {
		t.Errorf("Expected model sonnet, got %s", agent.Model())
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestFormatTaskResults(t *testing.T) {
	tasks := []*Task{
		{
			ID:     "task-1",
			Title:  "Test Task",
			Status: TaskStatusCompleted,
		},
	}

	result := formatTaskResults(tasks)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	if !contains(result, "task-1") {
		t.Error("Expected result to contain task ID")
	}

	if !contains(result, "completed") {
		t.Error("Expected result to contain status")
	}
}

func TestCountByStatus(t *testing.T) {
	tasks := []*Task{
		{ID: "1", Status: TaskStatusCompleted},
		{ID: "2", Status: TaskStatusCompleted},
		{ID: "3", Status: TaskStatusFailed},
		{ID: "4", Status: TaskStatusPending},
	}

	completed := countByStatus(tasks, TaskStatusCompleted)
	if completed != 2 {
		t.Errorf("Expected 2 completed, got %d", completed)
	}

	failed := countByStatus(tasks, TaskStatusFailed)
	if failed != 1 {
		t.Errorf("Expected 1 failed, got %d", failed)
	}

	pending := countByStatus(tasks, TaskStatusPending)
	if pending != 1 {
		t.Errorf("Expected 1 pending, got %d", pending)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
