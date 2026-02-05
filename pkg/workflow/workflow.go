package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/tool"
	"github.com/xinguang/agentic-coder/pkg/workflow/agent"
)

// Workflow orchestrates the multi-agent workflow
type Workflow struct {
	config *WorkflowConfig

	// Provider factory
	providerFactory agent.ProviderFactory
	engineFactory   func() *engine.Engine

	// Agents
	manager   *agent.ManagerAgent
	evaluator *agent.EvaluatorAgent

	// Agent pools
	executorPool *agent.ExecutorPool
	reviewerPool *agent.ReviewerPool
	fixerPool    *agent.FixerPool

	// State
	plan        *TaskPlan
	coordinator *Coordinator
	report      *FinalReport

	// Callbacks
	onProgress func(event *ProgressEvent)

	// Metrics
	startTime   time.Time
	totalTokens int
	totalCost   float64
}

// NewWorkflow creates a new workflow
func NewWorkflow(config *WorkflowConfig, provFactory agent.ProviderFactory, engFactory func() *engine.Engine) *Workflow {
	if config == nil {
		config = DefaultConfig()
	}
	config.Validate()

	w := &Workflow{
		config:          config,
		providerFactory: provFactory,
		engineFactory:   engFactory,
	}

	// Create manager agent
	managerModel := config.Models.GetModel(RoleManager)
	w.manager = agent.NewManagerAgent(managerModel, provFactory(managerModel))

	// Create evaluator agent
	evalModel := config.Models.GetModel(RoleEvaluator)
	w.evaluator = agent.NewEvaluatorAgent(evalModel, provFactory(evalModel))

	// Create executor pool
	execModel := config.Models.GetModel(RoleExecutor)
	w.executorPool = agent.NewExecutorPool(config.MaxExecutors, engFactory, execModel)

	// Create reviewer pool
	reviewModel := config.Models.GetModel(RoleReviewer)
	w.reviewerPool = agent.NewReviewerPool(config.MaxReviewers, reviewModel, provFactory(reviewModel))

	// Create fixer pool if enabled
	if config.EnableAutoFix {
		fixModel := config.Models.GetModel(RoleFixer)
		w.fixerPool = agent.NewFixerPool(config.MaxFixers, engFactory, fixModel)
	}

	return w
}

// SetProgressCallback sets the progress callback
func (w *Workflow) SetProgressCallback(cb func(*ProgressEvent)) {
	w.onProgress = cb
}

// Run executes the complete workflow
func (w *Workflow) Run(ctx context.Context, requirement string) (*FinalReport, error) {
	w.startTime = time.Now()

	// Phase 1: Manager analyzes and plans
	w.emit(&ProgressEvent{
		Type:    "analyzing",
		Message: "Manager analyzing requirement...",
	})

	plan, err := w.manager.AnalyzeRequirement(ctx, requirement)
	if err != nil {
		return nil, fmt.Errorf("manager analysis failed: %w", err)
	}
	w.plan = plan

	w.emit(&ProgressEvent{
		Type:    "plan_created",
		Message: fmt.Sprintf("Created plan with %d tasks", len(plan.Tasks)),
	})

	// Create coordinator
	w.coordinator, err = NewCoordinator(w.config, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator: %w", err)
	}

	// Phase 2: Execute all tasks
	if err := w.executeAllTasks(ctx); err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Phase 3: Evaluator evaluates
	w.emit(&ProgressEvent{
		Type:    "evaluating",
		Message: "Evaluator assessing results...",
	})

	eval, err := w.evaluator.Evaluate(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	// Phase 4: Manager generates report
	w.emit(&ProgressEvent{
		Type:    "reporting",
		Message: "Manager generating report...",
	})

	report, err := w.manager.GenerateReport(ctx, plan, eval)
	if err != nil {
		return nil, fmt.Errorf("report generation failed: %w", err)
	}

	report.TotalDuration = time.Since(w.startTime)
	report.TotalTokens = w.totalTokens
	report.TotalCost = w.totalCost
	w.report = report

	w.emit(&ProgressEvent{
		Type:    "completed",
		Message: "Workflow completed",
	})

	return report, nil
}

// executeAllTasks executes all tasks with concurrency control
func (w *Workflow) executeAllTasks(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	for {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if all done
		if w.coordinator.IsAllCompleted() {
			break
		}

		// Get ready tasks
		readyTasks := w.coordinator.GetReadyTasks()
		if len(readyTasks) == 0 {
			// No tasks ready, wait a bit
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Schedule ready tasks
		for _, task := range readyTasks {
			// Try to schedule
			if !w.coordinator.TryScheduleTask(ctx, task) {
				continue // Resources not available
			}

			wg.Add(1)
			go func(t *Task) {
				defer wg.Done()
				defer w.coordinator.ReleaseTask(t)

				if err := w.executeTaskWithReview(ctx, t); err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}(task)
		}

		// Small delay to prevent busy loop
		time.Sleep(50 * time.Millisecond)

		// Check for errors
		select {
		case err := <-errCh:
			// Cancel remaining and return error
			return err
		default:
		}
	}

	// Wait for all goroutines
	wg.Wait()

	// Final error check
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// executeTaskWithReview executes a task and handles review/retry loop
func (w *Workflow) executeTaskWithReview(ctx context.Context, task *Task) error {
	for retry := 0; retry <= w.config.MaxRetries; retry++ {
		task.RetryCount = retry

		// Execute
		w.emit(&ProgressEvent{
			Type:      "task_started",
			TaskID:    task.ID,
			TaskTitle: task.Title,
			Status:    TaskStatusRunning,
			Message:   fmt.Sprintf("Attempt %d/%d", retry+1, w.config.MaxRetries+1),
		})

		executor, err := w.executorPool.Acquire(ctx)
		if err != nil {
			return err
		}

		exec, execErr := executor.ExecuteTask(ctx, task)
		w.executorPool.Release(executor)

		task.Execution = exec

		if execErr != nil && !exec.Success {
			// Execution failed, try retry
			w.emit(&ProgressEvent{
				Type:      "task_failed",
				TaskID:    task.ID,
				TaskTitle: task.Title,
				Status:    TaskStatusFailed,
				Message:   exec.Error,
			})
			continue
		}

		// Review
		w.emit(&ProgressEvent{
			Type:      "reviewing",
			TaskID:    task.ID,
			TaskTitle: task.Title,
			Status:    TaskStatusReviewing,
		})

		reviewer, err := w.reviewerPool.Acquire(ctx)
		if err != nil {
			return err
		}

		review, reviewErr := reviewer.ReviewExecution(ctx, task, exec)
		w.reviewerPool.Release(reviewer)

		if reviewErr != nil {
			// Review failed, continue without review
			w.coordinator.MarkCompleted(task)
			return nil
		}

		task.Reviews = append(task.Reviews, review)

		switch review.Result {
		case ReviewPass:
			w.coordinator.MarkCompleted(task)
			w.emit(&ProgressEvent{
				Type:      "task_completed",
				TaskID:    task.ID,
				TaskTitle: task.Title,
				Status:    TaskStatusCompleted,
				Message:   fmt.Sprintf("Score: %d/100", review.Score),
			})
			return nil

		case ReviewFail:
			w.emit(&ProgressEvent{
				Type:      "review_failed",
				TaskID:    task.ID,
				TaskTitle: task.Title,
				Status:    TaskStatusFailed,
				Message:   review.Comments,
			})

			// Try auto-fix if enabled and possible
			if review.CanAutoFix && w.config.EnableAutoFix && w.fixerPool != nil && retry < w.config.MaxRetries {
				w.emit(&ProgressEvent{
					Type:      "fixing",
					TaskID:    task.ID,
					TaskTitle: task.Title,
					Status:    TaskStatusFixing,
					Message:   review.FixSuggestion,
				})

				fixer, err := w.fixerPool.Acquire(ctx)
				if err == nil {
					_, _ = fixer.FixIssues(ctx, task, review)
					w.fixerPool.Release(fixer)
				}
			}
			continue // Retry

		case ReviewReplan:
			// Serious issue, mark as failed
			w.coordinator.MarkFailed(task, fmt.Errorf("requires replanning: %s", review.Comments))
			return fmt.Errorf("task %s requires replanning", task.ID)
		}
	}

	// Max retries exceeded
	w.coordinator.MarkFailed(task, fmt.Errorf("max retries exceeded"))
	w.emit(&ProgressEvent{
		Type:      "task_failed",
		TaskID:    task.ID,
		TaskTitle: task.Title,
		Status:    TaskStatusFailed,
		Message:   fmt.Sprintf("Failed after %d retries", w.config.MaxRetries+1),
	})

	return nil // Don't fail the whole workflow for one task
}

// emit sends a progress event
func (w *Workflow) emit(event *ProgressEvent) {
	event.Timestamp = time.Now()
	if w.onProgress != nil {
		w.onProgress(event)
	}
}

// GetPlan returns the current plan
func (w *Workflow) GetPlan() *TaskPlan {
	return w.plan
}

// GetReport returns the final report
func (w *Workflow) GetReport() *FinalReport {
	return w.report
}

// GetProgress returns the current progress
func (w *Workflow) GetProgress() (completed, failed, total int) {
	if w.coordinator == nil {
		return 0, 0, 0
	}
	return w.coordinator.GetProgress()
}

// SimpleProviderFactory creates a simple provider factory for testing
func SimpleProviderFactory(prov provider.AIProvider) agent.ProviderFactory {
	return func(model string) provider.AIProvider {
		return prov
	}
}

// SimpleEngineFactory creates a simple engine factory
func SimpleEngineFactory(prov provider.AIProvider, registry *tool.Registry, cwd string) func() *engine.Engine {
	return func() *engine.Engine {
		return engine.NewEngine(&engine.EngineOptions{
			Provider: prov,
			Registry: registry,
		})
	}
}
