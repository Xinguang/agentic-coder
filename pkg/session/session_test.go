package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

func TestNewSession(t *testing.T) {
	opts := &SessionOptions{
		ProjectPath: "/test/project",
		CWD:         "/test/project/src",
		Model:       "claude-sonnet-4-5",
		Version:     "1.0.0",
		MaxTokens:   100000,
	}

	sess := NewSession(opts)

	if sess.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if sess.ProjectPath != "/test/project" {
		t.Errorf("Expected ProjectPath '/test/project', got %s", sess.ProjectPath)
	}
	if sess.CWD != "/test/project/src" {
		t.Errorf("Expected CWD '/test/project/src', got %s", sess.CWD)
	}
	if sess.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected Model 'claude-sonnet-4-5', got %s", sess.Model)
	}
	if sess.MaxTokens != 100000 {
		t.Errorf("Expected MaxTokens 100000, got %d", sess.MaxTokens)
	}
	if sess.CompactPercent != 0.95 {
		t.Errorf("Expected CompactPercent 0.95, got %f", sess.CompactPercent)
	}
	if sess.Messages == nil {
		t.Error("Messages should be initialized")
	}
	if sess.MessageTree == nil {
		t.Error("MessageTree should be initialized")
	}
}

func TestSessionAddEntry(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	entry := &TranscriptEntry{
		Type: EntryTypeUser,
		Message: &Message{
			Role: "user",
			Content: []provider.ContentBlock{
				&provider.TextBlock{Text: "Hello"},
			},
		},
	}

	sess.AddEntry(entry)

	if len(sess.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(sess.Messages))
	}
	if entry.UUID == "" {
		t.Error("Entry UUID should be generated")
	}
	if entry.Timestamp.IsZero() {
		t.Error("Entry timestamp should be set")
	}
	if entry.SessionID != sess.ID {
		t.Errorf("Entry SessionID should match session ID")
	}
	if sess.CurrentUUID != entry.UUID {
		t.Error("CurrentUUID should be updated")
	}

	// Check message tree
	if _, ok := sess.MessageTree[entry.UUID]; !ok {
		t.Error("Entry should be in MessageTree")
	}
}

func TestSessionAddUserMessage(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	entry := sess.AddUserMessage("Hello, world!")

	if entry.Type != EntryTypeUser {
		t.Errorf("Expected type %s, got %s", EntryTypeUser, entry.Type)
	}
	if entry.Message.Role != "user" {
		t.Errorf("Expected role 'user', got %s", entry.Message.Role)
	}
	if len(entry.Message.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(entry.Message.Content))
	}

	textBlock, ok := entry.Message.Content[0].(*provider.TextBlock)
	if !ok {
		t.Fatal("Expected TextBlock")
	}
	if textBlock.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got %s", textBlock.Text)
	}
}

func TestSessionAddToolResult(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	entry := sess.AddToolResult("tool_123", "Result content", false, map[string]interface{}{"key": "value"})

	if entry.Type != EntryTypeUser {
		t.Errorf("Expected type %s, got %s", EntryTypeUser, entry.Type)
	}
	if len(entry.Message.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(entry.Message.Content))
	}

	resultBlock, ok := entry.Message.Content[0].(*provider.ToolResultBlock)
	if !ok {
		t.Fatal("Expected ToolResultBlock")
	}
	if resultBlock.ToolUseID != "tool_123" {
		t.Errorf("Expected ToolUseID 'tool_123', got %s", resultBlock.ToolUseID)
	}
	if resultBlock.Content != "Result content" {
		t.Errorf("Expected content 'Result content', got %s", resultBlock.Content)
	}
	if resultBlock.IsError {
		t.Error("Expected IsError to be false")
	}
}

func TestSessionAddToolResultError(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	entry := sess.AddToolResult("tool_456", "Error message", true, nil)

	resultBlock, ok := entry.Message.Content[0].(*provider.ToolResultBlock)
	if !ok {
		t.Fatal("Expected ToolResultBlock")
	}
	if !resultBlock.IsError {
		t.Error("Expected IsError to be true")
	}
}

func TestSessionAddAssistantMessage(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	resp := &provider.Response{
		ID:         "msg_123",
		Model:      "test-model",
		StopReason: provider.StopReasonEndTurn,
		Content: []provider.ContentBlock{
			&provider.TextBlock{Text: "Hello from assistant"},
		},
		Usage: provider.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	entry := sess.AddAssistantMessage(resp)

	if entry.Type != EntryTypeAssistant {
		t.Errorf("Expected type %s, got %s", EntryTypeAssistant, entry.Type)
	}
	if entry.Message.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %s", entry.Message.Role)
	}
	if entry.Message.ID != "msg_123" {
		t.Errorf("Expected ID 'msg_123', got %s", entry.Message.ID)
	}
	if entry.Message.StopReason != "end_turn" {
		t.Errorf("Expected StopReason 'end_turn', got %s", entry.Message.StopReason)
	}
}

func TestSessionGetMessages(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	// Add some messages
	sess.AddUserMessage("User message 1")
	sess.AddAssistantMessage(&provider.Response{
		Content: []provider.ContentBlock{
			&provider.TextBlock{Text: "Assistant response"},
		},
	})
	sess.AddUserMessage("User message 2")

	messages := sess.GetMessages()

	if len(messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(messages))
	}

	if messages[0].Role != provider.RoleUser {
		t.Errorf("Expected first message role 'user', got %s", messages[0].Role)
	}
	if messages[1].Role != provider.RoleAssistant {
		t.Errorf("Expected second message role 'assistant', got %s", messages[1].Role)
	}
	if messages[2].Role != provider.RoleUser {
		t.Errorf("Expected third message role 'user', got %s", messages[2].Role)
	}
}

func TestSessionUpdateTodos(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	todos := []Todo{
		{Content: "Task 1", Status: "pending", ActiveForm: "Starting task 1"},
		{Content: "Task 2", Status: "in_progress", ActiveForm: "Working on task 2"},
	}

	sess.UpdateTodos(todos)

	if len(sess.Todos) != 2 {
		t.Errorf("Expected 2 todos, got %d", len(sess.Todos))
	}
	if sess.Todos[0].Content != "Task 1" {
		t.Errorf("Expected first todo 'Task 1', got %s", sess.Todos[0].Content)
	}
	if sess.Todos[1].Status != "in_progress" {
		t.Errorf("Expected second todo status 'in_progress', got %s", sess.Todos[1].Status)
	}
}

func TestSessionShouldCompact(t *testing.T) {
	tests := []struct {
		name           string
		maxTokens      int
		tokenCount     int
		compactPercent float64
		expected       bool
	}{
		{"No max tokens", 0, 1000, 0.95, false},
		{"Below threshold", 100000, 90000, 0.95, false},
		{"At threshold", 100000, 95000, 0.95, true},
		{"Above threshold", 100000, 98000, 0.95, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := NewSession(&SessionOptions{
				MaxTokens: tt.maxTokens,
			})
			sess.TokenCount = tt.tokenCount
			sess.CompactPercent = tt.compactPercent

			result := sess.ShouldCompact()
			if result != tt.expected {
				t.Errorf("ShouldCompact() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSessionParentUUID(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	// First entry has no parent
	entry1 := sess.AddUserMessage("First message")
	if entry1.ParentUUID != nil {
		t.Error("First entry should have no parent")
	}

	// Second entry has first as parent
	entry2 := sess.AddUserMessage("Second message")
	if entry2.ParentUUID == nil {
		t.Fatal("Second entry should have parent")
	}
	if *entry2.ParentUUID != entry1.UUID {
		t.Errorf("Second entry parent should be first entry UUID")
	}
}

func TestTranscriptEntryMarshalJSON(t *testing.T) {
	entry := &TranscriptEntry{
		Type:      EntryTypeUser,
		UUID:      "test-uuid",
		Timestamp: time.Now(),
		SessionID: "session-123",
		CWD:       "/test",
		Version:   "1.0",
		Message: &Message{
			Role: "user",
			Content: []provider.ContentBlock{
				&provider.TextBlock{Text: "Hello"},
			},
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal TranscriptEntry: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result["type"] != string(EntryTypeUser) {
		t.Errorf("Expected type 'user', got %v", result["type"])
	}
	if result["uuid"] != "test-uuid" {
		t.Errorf("Expected uuid 'test-uuid', got %v", result["uuid"])
	}
}

func TestEntryTypeConstants(t *testing.T) {
	if EntryTypeUser != "user" {
		t.Errorf("EntryTypeUser = %q, want 'user'", EntryTypeUser)
	}
	if EntryTypeAssistant != "assistant" {
		t.Errorf("EntryTypeAssistant = %q, want 'assistant'", EntryTypeAssistant)
	}
	if EntryTypeSystem != "system" {
		t.Errorf("EntryTypeSystem = %q, want 'system'", EntryTypeSystem)
	}
}

func TestTodoStruct(t *testing.T) {
	todo := Todo{
		Content:    "Test task",
		Status:     "in_progress",
		ActiveForm: "Testing task",
	}

	if todo.Content != "Test task" {
		t.Errorf("Expected Content 'Test task', got %s", todo.Content)
	}
	if todo.Status != "in_progress" {
		t.Errorf("Expected Status 'in_progress', got %s", todo.Status)
	}
	if todo.ActiveForm != "Testing task" {
		t.Errorf("Expected ActiveForm 'Testing task', got %s", todo.ActiveForm)
	}
}

func TestThinkingMetadata(t *testing.T) {
	meta := &ThinkingMetadata{
		Level:    "high",
		Disabled: false,
		Triggers: []string{"complex", "analysis"},
	}

	if meta.Level != "high" {
		t.Errorf("Expected Level 'high', got %s", meta.Level)
	}
	if meta.Disabled {
		t.Error("Expected Disabled to be false")
	}
	if len(meta.Triggers) != 2 {
		t.Errorf("Expected 2 triggers, got %d", len(meta.Triggers))
	}
}

func TestSessionConcurrency(t *testing.T) {
	sess := NewSession(&SessionOptions{
		CWD:     "/test",
		Model:   "test-model",
		Version: "1.0",
	})

	// Run concurrent operations
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(n int) {
			sess.AddUserMessage("Message from goroutine")
			sess.GetMessages()
			sess.UpdateTodos([]Todo{{Content: "Todo", Status: "pending", ActiveForm: "Adding todo"}})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panic and messages were added
	messages := sess.GetMessages()
	if len(messages) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(messages))
	}
}
