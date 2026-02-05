package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Coordinator manages task scheduling and concurrency
type Coordinator struct {
	config *WorkflowConfig
	plan   *TaskPlan
	dag    *DAG

	// Concurrency control
	execSem   *Semaphore
	reviewSem *Semaphore
	fixSem    *Semaphore

	// Resource locking
	resourceLock *ResourceLock

	// Status tracking
	mu        sync.RWMutex
	completed map[string]bool
	failed    map[string]bool

	// Callbacks
	onTaskReady    func(task *Task)
	onTaskStart    func(task *Task)
	onTaskComplete func(task *Task)
	onTaskFail     func(task *Task, err error)
}

// NewCoordinator creates a new coordinator
func NewCoordinator(config *WorkflowConfig, plan *TaskPlan) (*Coordinator, error) {
	dag := NewDAG(plan.Tasks)

	// Validate DAG
	if err := dag.Validate(); err != nil {
		return nil, fmt.Errorf("invalid task plan: %w", err)
	}

	return &Coordinator{
		config:       config,
		plan:         plan,
		dag:          dag,
		execSem:      NewSemaphore(config.MaxExecutors),
		reviewSem:    NewSemaphore(config.MaxReviewers),
		fixSem:       NewSemaphore(config.MaxFixers),
		resourceLock: NewResourceLock(),
		completed:    make(map[string]bool),
		failed:       make(map[string]bool),
	}, nil
}

// SetCallbacks sets the coordinator callbacks
func (c *Coordinator) SetCallbacks(
	onTaskReady func(*Task),
	onTaskStart func(*Task),
	onTaskComplete func(*Task),
	onTaskFail func(*Task, error),
) {
	c.onTaskReady = onTaskReady
	c.onTaskStart = onTaskStart
	c.onTaskComplete = onTaskComplete
	c.onTaskFail = onTaskFail
}

// GetReadyTasks returns tasks that are ready to execute
func (c *Coordinator) GetReadyTasks() []*Task {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.dag.GetReadyTasks(c.completed)
}

// TryScheduleTask attempts to schedule a task for execution
// Returns true if the task was scheduled, false if resources are unavailable
func (c *Coordinator) TryScheduleTask(ctx context.Context, task *Task) bool {
	// Try to acquire executor semaphore
	if !c.execSem.TryAcquire() {
		return false
	}

	// Try to acquire resource locks
	if !c.resourceLock.TryLock(task.ID, task.Resources) {
		c.execSem.Release()
		return false
	}

	// Update task status
	c.mu.Lock()
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	c.mu.Unlock()

	if c.onTaskStart != nil {
		c.onTaskStart(task)
	}

	return true
}

// ReleaseTask releases resources held by a task
func (c *Coordinator) ReleaseTask(task *Task) {
	c.resourceLock.Unlock(task.ID)
	c.execSem.Release()
}

// MarkCompleted marks a task as completed
func (c *Coordinator) MarkCompleted(task *Task) {
	c.mu.Lock()
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	c.completed[task.ID] = true
	c.mu.Unlock()

	if c.onTaskComplete != nil {
		c.onTaskComplete(task)
	}
}

// MarkFailed marks a task as failed
func (c *Coordinator) MarkFailed(task *Task, err error) {
	c.mu.Lock()
	task.Status = TaskStatusFailed
	now := time.Now()
	task.CompletedAt = &now
	c.failed[task.ID] = true
	c.mu.Unlock()

	if c.onTaskFail != nil {
		c.onTaskFail(task, err)
	}

	// Cancel dependent tasks
	c.cancelDependentTasks(task.ID)
}

// cancelDependentTasks cancels all tasks that depend on a failed task
func (c *Coordinator) cancelDependentTasks(taskID string) {
	dependents := c.dag.GetDependents(taskID)
	for _, depID := range dependents {
		if task, ok := c.dag.GetTask(depID); ok {
			c.mu.Lock()
			if task.Status == TaskStatusPending || task.Status == TaskStatusReady {
				task.Status = TaskStatusCancelled
				c.failed[depID] = true
			}
			c.mu.Unlock()

			// Recursively cancel
			c.cancelDependentTasks(depID)
		}
	}
}

// AcquireReviewer acquires a reviewer slot
func (c *Coordinator) AcquireReviewer(ctx context.Context) error {
	return c.reviewSem.Acquire(ctx)
}

// ReleaseReviewer releases a reviewer slot
func (c *Coordinator) ReleaseReviewer() {
	c.reviewSem.Release()
}

// AcquireFixer acquires a fixer slot
func (c *Coordinator) AcquireFixer(ctx context.Context) error {
	return c.fixSem.Acquire(ctx)
}

// ReleaseFixer releases a fixer slot
func (c *Coordinator) ReleaseFixer() {
	c.fixSem.Release()
}

// IsAllCompleted checks if all tasks are completed
func (c *Coordinator) IsAllCompleted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, task := range c.dag.GetAllTasks() {
		if task.Status != TaskStatusCompleted && task.Status != TaskStatusFailed && task.Status != TaskStatusCancelled {
			return false
		}
	}
	return true
}

// GetProgress returns the current progress
func (c *Coordinator) GetProgress() (completed, failed, total int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total = len(c.dag.nodes)
	for _, task := range c.dag.GetAllTasks() {
		switch task.Status {
		case TaskStatusCompleted:
			completed++
		case TaskStatusFailed, TaskStatusCancelled:
			failed++
		}
	}
	return
}

// GetStats returns coordinator statistics
func (c *Coordinator) GetStats() CoordinatorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CoordinatorStats{
		TotalTasks:         len(c.dag.nodes),
		ExecutorsAvailable: c.execSem.Available(),
		ExecutorsInUse:     c.execSem.InUse(),
		ReviewersAvailable: c.reviewSem.Available(),
		ReviewersInUse:     c.reviewSem.InUse(),
		FixersAvailable:    c.fixSem.Available(),
		FixersInUse:        c.fixSem.InUse(),
		ResourcesLocked:    len(c.resourceLock.GetAllLocks()),
	}

	for _, task := range c.dag.GetAllTasks() {
		switch task.Status {
		case TaskStatusPending, TaskStatusReady:
			stats.PendingTasks++
		case TaskStatusRunning:
			stats.RunningTasks++
		case TaskStatusReviewing:
			stats.ReviewingTasks++
		case TaskStatusFixing:
			stats.FixingTasks++
		case TaskStatusCompleted:
			stats.CompletedTasks++
		case TaskStatusFailed, TaskStatusCancelled:
			stats.FailedTasks++
		}
	}

	return stats
}

// CoordinatorStats holds coordinator statistics
type CoordinatorStats struct {
	TotalTasks     int
	PendingTasks   int
	RunningTasks   int
	ReviewingTasks int
	FixingTasks    int
	CompletedTasks int
	FailedTasks    int

	ExecutorsAvailable int
	ExecutorsInUse     int
	ReviewersAvailable int
	ReviewersInUse     int
	FixersAvailable    int
	FixersInUse        int

	ResourcesLocked int
}
