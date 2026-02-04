package workctx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewWorkContext(t *testing.T) {
	mgr := NewManager(t.TempDir())

	ctx := mgr.New("Test Task", "Complete the test")

	if ctx.ID == "" {
		t.Error("ID should not be empty")
	}
	if ctx.Title != "Test Task" {
		t.Errorf("Title = %q, want %q", ctx.Title, "Test Task")
	}
	if ctx.Goal != "Complete the test" {
		t.Errorf("Goal = %q, want %q", ctx.Goal, "Complete the test")
	}
	if ctx.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if ctx.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create and save
	ctx := mgr.New("Save Test", "Test save functionality")
	ctx.Background = "Some background info"
	ctx.AddProgress("Step 1 done")
	ctx.AddPending("Step 2 pending")
	ctx.AddKeyFile("main.go")
	ctx.AddNote("Important note")

	if err := mgr.Save(ctx); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	filename := filepath.Join(tmpDir, "workctx", ctx.ID+".json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Saved file should exist")
	}

	// Load and verify
	mgr2 := NewManager(tmpDir)
	loaded, err := mgr2.Load(ctx.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != ctx.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, ctx.ID)
	}
	if loaded.Title != ctx.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, ctx.Title)
	}
	if loaded.Goal != ctx.Goal {
		t.Errorf("Goal = %q, want %q", loaded.Goal, ctx.Goal)
	}
	if loaded.Background != ctx.Background {
		t.Errorf("Background = %q, want %q", loaded.Background, ctx.Background)
	}
	if len(loaded.Progress) != 1 {
		t.Errorf("Progress len = %d, want 1", len(loaded.Progress))
	}
	if len(loaded.Pending) != 1 {
		t.Errorf("Pending len = %d, want 1", len(loaded.Pending))
	}
	if len(loaded.KeyFiles) != 1 {
		t.Errorf("KeyFiles len = %d, want 1", len(loaded.KeyFiles))
	}
	if len(loaded.Notes) != 1 {
		t.Errorf("Notes len = %d, want 1", len(loaded.Notes))
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create multiple contexts
	ctx1 := mgr.New("Task 1", "Goal 1")
	mgr.Save(ctx1)

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	ctx2 := mgr.New("Task 2", "Goal 2")
	mgr.Save(ctx2)

	// List
	contexts, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(contexts) != 2 {
		t.Errorf("List len = %d, want 2", len(contexts))
	}

	// Should be sorted by UpdatedAt descending (newest first)
	if contexts[0].ID != ctx2.ID {
		t.Error("Newest context should be first")
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	contexts, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(contexts) != 0 {
		t.Errorf("List should be empty, got %d", len(contexts))
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	ctx := mgr.New("Delete Test", "To be deleted")
	mgr.Save(ctx)

	// Verify exists
	if _, err := mgr.Load(ctx.ID); err != nil {
		t.Fatal("Context should exist before delete")
	}

	// Delete
	if err := mgr.Delete(ctx.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	if _, err := mgr.Load(ctx.ID); err == nil {
		t.Error("Context should not exist after delete")
	}
}

func TestAddProgress(t *testing.T) {
	ctx := &WorkContext{
		Progress: make([]ProgressItem, 0),
	}

	ctx.AddProgress("Task completed")

	if len(ctx.Progress) != 1 {
		t.Fatalf("Progress len = %d, want 1", len(ctx.Progress))
	}
	if ctx.Progress[0].Description != "Task completed" {
		t.Errorf("Description = %q, want %q", ctx.Progress[0].Description, "Task completed")
	}
	if ctx.Progress[0].Status != "done" {
		t.Errorf("Status = %q, want %q", ctx.Progress[0].Status, "done")
	}
	if ctx.Progress[0].CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestAddPending(t *testing.T) {
	ctx := &WorkContext{
		Pending: make([]ProgressItem, 0),
	}

	ctx.AddPending("Task to do")

	if len(ctx.Pending) != 1 {
		t.Fatalf("Pending len = %d, want 1", len(ctx.Pending))
	}
	if ctx.Pending[0].Description != "Task to do" {
		t.Errorf("Description = %q, want %q", ctx.Pending[0].Description, "Task to do")
	}
	if ctx.Pending[0].Status != "pending" {
		t.Errorf("Status = %q, want %q", ctx.Pending[0].Status, "pending")
	}
}

func TestCompletePending(t *testing.T) {
	ctx := &WorkContext{
		Progress: make([]ProgressItem, 0),
		Pending:  make([]ProgressItem, 0),
	}

	ctx.AddPending("Task 1")
	ctx.AddPending("Task 2")

	ctx.CompletePending(0)

	if len(ctx.Pending) != 1 {
		t.Errorf("Pending len = %d, want 1", len(ctx.Pending))
	}
	if len(ctx.Progress) != 1 {
		t.Errorf("Progress len = %d, want 1", len(ctx.Progress))
	}
	if ctx.Progress[0].Description != "Task 1" {
		t.Errorf("Completed task should be Task 1, got %q", ctx.Progress[0].Description)
	}
	if ctx.Progress[0].Status != "done" {
		t.Errorf("Status should be done, got %q", ctx.Progress[0].Status)
	}
}

func TestCompletePendingOutOfBounds(t *testing.T) {
	ctx := &WorkContext{
		Progress: make([]ProgressItem, 0),
		Pending:  make([]ProgressItem, 0),
	}

	ctx.AddPending("Task 1")

	// Should not panic
	ctx.CompletePending(-1)
	ctx.CompletePending(10)

	if len(ctx.Pending) != 1 {
		t.Error("Pending should remain unchanged for out of bounds")
	}
}

func TestAddKeyFile(t *testing.T) {
	ctx := &WorkContext{
		KeyFiles: make([]string, 0),
	}

	ctx.AddKeyFile("main.go")
	ctx.AddKeyFile("main.go") // Duplicate
	ctx.AddKeyFile("util.go")

	if len(ctx.KeyFiles) != 2 {
		t.Errorf("KeyFiles len = %d, want 2 (no duplicates)", len(ctx.KeyFiles))
	}
}

func TestAddNote(t *testing.T) {
	ctx := &WorkContext{
		Notes: make([]string, 0),
	}

	ctx.AddNote("Note 1")
	ctx.AddNote("Note 2")

	if len(ctx.Notes) != 2 {
		t.Errorf("Notes len = %d, want 2", len(ctx.Notes))
	}
}

func TestUpdateTokens(t *testing.T) {
	ctx := &WorkContext{}

	ctx.UpdateTokens("claude", 1000)
	ctx.UpdateTokens("openai", 500)
	ctx.UpdateTokens("claude", 500)

	if ctx.TokensUsed["claude"] != 1500 {
		t.Errorf("Claude tokens = %d, want 1500", ctx.TokensUsed["claude"])
	}
	if ctx.TokensUsed["openai"] != 500 {
		t.Errorf("OpenAI tokens = %d, want 500", ctx.TokensUsed["openai"])
	}
}

func TestGenerateHandoff(t *testing.T) {
	ctx := &WorkContext{
		ID:         "test123",
		Title:      "Test Task",
		Goal:       "Complete testing",
		Background: "This is a test",
		Progress: []ProgressItem{
			{Description: "Step 1", Status: "done"},
		},
		Pending: []ProgressItem{
			{Description: "Step 2", Status: "pending"},
		},
		KeyFiles:   []string{"main.go"},
		Notes:      []string{"Important note"},
		TokensUsed: map[string]int64{"claude": 1000},
		UpdatedAt:  time.Now(),
	}

	handoff := ctx.GenerateHandoff()

	// Check required sections exist
	requiredSections := []string{
		"# Work Context Handoff",
		"**ID:** test123",
		"**Title:** Test Task",
		"## Goal",
		"Complete testing",
		"## Background",
		"This is a test",
		"## Completed",
		"[x] Step 1",
		"## Remaining Tasks",
		"[ ] Step 2",
		"## Key Files",
		"`main.go`",
		"## Important Notes",
		"Important note",
		"## Token Usage",
		"claude: 1000 tokens",
		"## Instructions for Continuation",
		"**Step 2**",
	}

	for _, section := range requiredSections {
		if !strings.Contains(handoff, section) {
			t.Errorf("Handoff should contain %q", section)
		}
	}
}

func TestGenerateHandoffCN(t *testing.T) {
	ctx := &WorkContext{
		ID:         "test123",
		Title:      "测试任务",
		Goal:       "完成测试",
		Background: "这是一个测试",
		Progress: []ProgressItem{
			{Description: "步骤1", Status: "done"},
		},
		Pending: []ProgressItem{
			{Description: "步骤2", Status: "pending"},
		},
		KeyFiles:   []string{"main.go"},
		Notes:      []string{"重要备注"},
		TokensUsed: map[string]int64{"claude": 1000},
		UpdatedAt:  time.Now(),
	}

	handoff := ctx.GenerateHandoffCN()

	// Check Chinese sections
	requiredSections := []string{
		"# 工作上下文交接",
		"**标题:** 测试任务",
		"## 目标",
		"## 背景",
		"## 已完成",
		"## 待完成",
		"## 关键文件",
		"## 重要说明",
		"## Token 使用情况",
		"## 继续说明",
	}

	for _, section := range requiredSections {
		if !strings.Contains(handoff, section) {
			t.Errorf("Chinese handoff should contain %q", section)
		}
	}
}

func TestGenerateHandoffEmpty(t *testing.T) {
	ctx := &WorkContext{
		ID:        "test123",
		Title:     "Empty Task",
		Goal:      "No progress yet",
		UpdatedAt: time.Now(),
	}

	handoff := ctx.GenerateHandoff()

	// Should not contain empty sections
	if strings.Contains(handoff, "## Background") {
		t.Error("Should not have Background section when empty")
	}
	if strings.Contains(handoff, "## Completed") {
		t.Error("Should not have Completed section when empty")
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name     string
		progress int
		pending  int
		want     string
	}{
		{"no tasks", 0, 0, "0% (0/0 done)"},
		{"all done", 3, 0, "100% (3/3 done)"},
		{"half done", 2, 2, "50% (2/4 done)"},
		{"one third", 1, 2, "33% (1/3 done)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &WorkContext{
				ID:       "abc",
				Title:    "Test",
				Progress: make([]ProgressItem, tt.progress),
				Pending:  make([]ProgressItem, tt.pending),
			}

			summary := ctx.Summary()
			if !strings.Contains(summary, tt.want) {
				t.Errorf("Summary = %q, should contain %q", summary, tt.want)
			}
		})
	}
}

func TestCurrent(t *testing.T) {
	mgr := NewManager(t.TempDir())

	// Initially nil
	if mgr.Current() != nil {
		t.Error("Current should be nil initially")
	}

	// After New
	ctx := mgr.New("Test", "Goal")
	if mgr.Current() != ctx {
		t.Error("Current should return the newly created context")
	}

	// After SetCurrent
	ctx2 := &WorkContext{ID: "custom"}
	mgr.SetCurrent(ctx2)
	if mgr.Current() != ctx2 {
		t.Error("Current should return the set context")
	}
}

func TestLoadNotFound(t *testing.T) {
	mgr := NewManager(t.TempDir())

	_, err := mgr.Load("nonexistent")
	if err == nil {
		t.Error("Load should return error for nonexistent context")
	}
}

func TestSaveNil(t *testing.T) {
	mgr := NewManager(t.TempDir())

	err := mgr.Save(nil)
	if err == nil {
		t.Error("Save should return error for nil context")
	}
}

func TestDefaultConfigDir(t *testing.T) {
	mgr := NewManager("")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "agentic-coder", "workctx")

	if mgr.contextsDir() != expected {
		t.Errorf("contextsDir = %q, want %q", mgr.contextsDir(), expected)
	}
}
