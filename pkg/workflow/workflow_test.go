package workflow

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDAG_TopologicalSort(t *testing.T) {
	tasks := []*Task{
		{ID: "1", Title: "Task 1", Priority: 1, DependsOn: []string{}},
		{ID: "2", Title: "Task 2", Priority: 2, DependsOn: []string{"1"}},
		{ID: "3", Title: "Task 3", Priority: 2, DependsOn: []string{"1"}},
		{ID: "4", Title: "Task 4", Priority: 3, DependsOn: []string{"2", "3"}},
	}

	dag := NewDAG(tasks)

	sorted, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(sorted) != 4 {
		t.Errorf("Expected 4 tasks, got %d", len(sorted))
	}

	// Task 1 should be first
	if sorted[0].ID != "1" {
		t.Errorf("Expected task 1 first, got %s", sorted[0].ID)
	}

	// Task 4 should be last
	if sorted[3].ID != "4" {
		t.Errorf("Expected task 4 last, got %s", sorted[3].ID)
	}
}

func TestDAG_CircularDependency(t *testing.T) {
	tasks := []*Task{
		{ID: "1", Title: "Task 1", DependsOn: []string{"3"}},
		{ID: "2", Title: "Task 2", DependsOn: []string{"1"}},
		{ID: "3", Title: "Task 3", DependsOn: []string{"2"}},
	}

	dag := NewDAG(tasks)

	_, err := dag.TopologicalSort()
	if err == nil {
		t.Error("Expected circular dependency error")
	}
}

func TestDAG_GetReadyTasks(t *testing.T) {
	tasks := []*Task{
		{ID: "1", Title: "Task 1", Priority: 1, DependsOn: []string{}, Status: TaskStatusPending},
		{ID: "2", Title: "Task 2", Priority: 2, DependsOn: []string{"1"}, Status: TaskStatusPending},
		{ID: "3", Title: "Task 3", Priority: 1, DependsOn: []string{}, Status: TaskStatusPending},
	}

	dag := NewDAG(tasks)

	// Initially, tasks 1 and 3 should be ready
	ready := dag.GetReadyTasks(map[string]bool{})
	if len(ready) != 2 {
		t.Errorf("Expected 2 ready tasks, got %d", len(ready))
	}

	// After completing task 1, task 2 should also be ready
	ready = dag.GetReadyTasks(map[string]bool{"1": true})
	if len(ready) != 2 { // task 2 and 3
		t.Errorf("Expected 2 ready tasks after completing 1, got %d", len(ready))
	}
}

func TestDAG_GetExecutionLevels(t *testing.T) {
	tasks := []*Task{
		{ID: "1", Title: "Task 1", Priority: 1, DependsOn: []string{}},
		{ID: "2", Title: "Task 2", Priority: 2, DependsOn: []string{"1"}},
		{ID: "3", Title: "Task 3", Priority: 2, DependsOn: []string{"1"}},
		{ID: "4", Title: "Task 4", Priority: 3, DependsOn: []string{"2", "3"}},
	}

	dag := NewDAG(tasks)

	levels, err := dag.GetExecutionLevels()
	if err != nil {
		t.Fatalf("GetExecutionLevels failed: %v", err)
	}

	if len(levels) != 3 {
		t.Errorf("Expected 3 levels, got %d", len(levels))
	}

	// Level 0: Task 1
	if len(levels[0]) != 1 {
		t.Errorf("Level 0 should have 1 task, got %d", len(levels[0]))
	}

	// Level 1: Tasks 2 and 3
	if len(levels[1]) != 2 {
		t.Errorf("Level 1 should have 2 tasks, got %d", len(levels[1]))
	}

	// Level 2: Task 4
	if len(levels[2]) != 1 {
		t.Errorf("Level 2 should have 1 task, got %d", len(levels[2]))
	}
}

func TestSemaphore_Basic(t *testing.T) {
	sem := NewSemaphore(3)

	if sem.Capacity() != 3 {
		t.Errorf("Expected capacity 3, got %d", sem.Capacity())
	}

	if sem.Available() != 3 {
		t.Errorf("Expected 3 available, got %d", sem.Available())
	}

	// Acquire all slots
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := sem.Acquire(ctx); err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}
	}

	if sem.Available() != 0 {
		t.Errorf("Expected 0 available, got %d", sem.Available())
	}

	// TryAcquire should fail
	if sem.TryAcquire() {
		t.Error("TryAcquire should have failed")
	}

	// Release one
	sem.Release()

	if sem.Available() != 1 {
		t.Errorf("Expected 1 available, got %d", sem.Available())
	}

	// TryAcquire should succeed now
	if !sem.TryAcquire() {
		t.Error("TryAcquire should have succeeded")
	}
}

func TestSemaphore_Concurrent(t *testing.T) {
	sem := NewSemaphore(2)
	ctx := context.Background()

	var wg sync.WaitGroup
	counter := 0
	maxConcurrent := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := sem.Acquire(ctx); err != nil {
				return
			}
			defer sem.Release()

			mu.Lock()
			counter++
			if counter > maxConcurrent {
				maxConcurrent = counter
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			counter--
			mu.Unlock()
		}()
	}

	wg.Wait()

	if maxConcurrent > 2 {
		t.Errorf("Max concurrent exceeded limit: got %d, expected <= 2", maxConcurrent)
	}
}

func TestResourceLock_Basic(t *testing.T) {
	rl := NewResourceLock()

	// Lock resources for task 1
	if !rl.TryLock("task1", []string{"file1.go", "file2.go"}) {
		t.Error("TryLock should succeed for task1")
	}

	// Task 2 should fail to lock the same resources
	if rl.TryLock("task2", []string{"file1.go"}) {
		t.Error("TryLock should fail for task2 on locked resource")
	}

	// Task 2 can lock different resources
	if !rl.TryLock("task2", []string{"file3.go"}) {
		t.Error("TryLock should succeed for task2 on different resource")
	}

	// Check holders
	holder, ok := rl.GetHolder("file1.go")
	if !ok || holder != "task1" {
		t.Errorf("Expected holder task1, got %s", holder)
	}

	// Unlock task 1
	rl.Unlock("task1")

	// Now task 2 can lock file1.go
	if !rl.TryLock("task2", []string{"file1.go"}) {
		t.Error("TryLock should succeed after unlock")
	}
}

func TestResourceLock_EmptyResources(t *testing.T) {
	rl := NewResourceLock()

	// Empty resources should always succeed
	if !rl.TryLock("task1", []string{}) {
		t.Error("TryLock with empty resources should succeed")
	}

	if !rl.TryLock("task1", nil) {
		t.Error("TryLock with nil resources should succeed")
	}
}

func TestConfig_GetModel(t *testing.T) {
	models := RoleModels{
		Default:  "sonnet",
		Manager:  "opus",
		Reviewer: "",
	}

	// Manager has specific model
	if models.GetModel(RoleManager) != "opus" {
		t.Error("Manager should use opus")
	}

	// Reviewer falls back to default
	if models.GetModel(RoleReviewer) != "sonnet" {
		t.Error("Reviewer should fall back to sonnet")
	}

	// Executor falls back to default
	if models.GetModel(RoleExecutor) != "sonnet" {
		t.Error("Executor should fall back to sonnet")
	}
}

func TestConfig_Default(t *testing.T) {
	config := DefaultConfig()

	if config.MaxExecutors != 5 {
		t.Errorf("Expected MaxExecutors 5, got %d", config.MaxExecutors)
	}

	if config.MaxReviewers != 2 {
		t.Errorf("Expected MaxReviewers 2, got %d", config.MaxReviewers)
	}

	if config.Models.Default != "sonnet" {
		t.Errorf("Expected default model sonnet, got %s", config.Models.Default)
	}
}

func TestCoordinator_Basic(t *testing.T) {
	config := DefaultConfig()
	plan := &TaskPlan{
		ID:          "plan1",
		Requirement: "Test requirement",
		Tasks: []*Task{
			{ID: "1", Title: "Task 1", Priority: 1, DependsOn: []string{}, Status: TaskStatusPending},
			{ID: "2", Title: "Task 2", Priority: 2, DependsOn: []string{"1"}, Status: TaskStatusPending},
		},
	}

	coord, err := NewCoordinator(config, plan)
	if err != nil {
		t.Fatalf("NewCoordinator failed: %v", err)
	}

	// Initially only task 1 is ready
	ready := coord.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "1" {
		t.Error("Only task 1 should be ready initially")
	}

	// Schedule task 1
	ctx := context.Background()
	if !coord.TryScheduleTask(ctx, plan.Tasks[0]) {
		t.Error("TryScheduleTask should succeed for task 1")
	}

	// Mark task 1 completed
	coord.ReleaseTask(plan.Tasks[0])
	coord.MarkCompleted(plan.Tasks[0])

	// Now task 2 should be ready
	ready = coord.GetReadyTasks()
	if len(ready) != 1 || ready[0].ID != "2" {
		t.Error("Task 2 should be ready after task 1 completes")
	}
}

func TestCoordinator_Progress(t *testing.T) {
	config := DefaultConfig()
	plan := &TaskPlan{
		ID: "plan1",
		Tasks: []*Task{
			{ID: "1", Title: "Task 1", Status: TaskStatusPending},
			{ID: "2", Title: "Task 2", Status: TaskStatusPending},
			{ID: "3", Title: "Task 3", Status: TaskStatusPending},
		},
	}

	coord, _ := NewCoordinator(config, plan)

	completed, failed, total := coord.GetProgress()
	if total != 3 || completed != 0 || failed != 0 {
		t.Errorf("Expected 0/0/3, got %d/%d/%d", completed, failed, total)
	}

	coord.MarkCompleted(plan.Tasks[0])
	coord.MarkFailed(plan.Tasks[1], nil)

	completed, failed, total = coord.GetProgress()
	if completed != 1 || failed != 1 {
		t.Errorf("Expected 1/1/3, got %d/%d/%d", completed, failed, total)
	}
}
