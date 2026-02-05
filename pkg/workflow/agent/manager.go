package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/workflow"
)

// ManagerAgent handles requirement analysis and task planning
type ManagerAgent struct {
	*BaseAgent
}

// NewManagerAgent creates a new manager agent
func NewManagerAgent(model string, prov provider.AIProvider) *ManagerAgent {
	return &ManagerAgent{
		BaseAgent: NewBaseAgent(workflow.RoleManager, model, prov),
	}
}

const managerAnalyzePrompt = `You are a Project Manager Agent responsible for analyzing requirements and planning tasks.

Your responsibilities:
1. Analyze user requirements thoroughly
2. Break down into concrete, actionable tasks
3. Identify dependencies between tasks
4. Prioritize tasks (1=highest priority, 5=lowest)
5. Identify resources (files) each task will modify

Output JSON format:
{
  "analysis": "Your detailed analysis of the requirement",
  "tasks": [
    {
      "id": "task-1",
      "title": "Short descriptive title",
      "description": "Detailed description of what needs to be done",
      "priority": 1,
      "depends_on": [],
      "resources": ["path/to/file.go"]
    }
  ]
}

Rules:
- Each task should be independently executable
- Tasks should be small and focused (can be completed in one session)
- Clearly specify dependencies using task IDs
- List all files that will be created or modified
- Use meaningful task IDs (task-1, task-2, etc.)
- Order tasks logically`

// AnalyzeRequirement analyzes a requirement and creates a task plan
func (m *ManagerAgent) AnalyzeRequirement(ctx context.Context, requirement string) (*workflow.TaskPlan, error) {
	var result struct {
		Analysis string `json:"analysis"`
		Tasks    []struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Priority    int      `json:"priority"`
			DependsOn   []string `json:"depends_on"`
			Resources   []string `json:"resources"`
		} `json:"tasks"`
	}

	if err := m.ChatJSON(ctx, managerAnalyzePrompt, requirement, &result); err != nil {
		return nil, fmt.Errorf("failed to analyze requirement: %w", err)
	}

	// Convert to workflow.Task
	tasks := make([]*workflow.Task, len(result.Tasks))
	for i, t := range result.Tasks {
		tasks[i] = &workflow.Task{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Priority:    t.Priority,
			DependsOn:   t.DependsOn,
			Resources:   t.Resources,
			Status:      workflow.TaskStatusPending,
			CreatedAt:   time.Now(),
		}
	}

	return &workflow.TaskPlan{
		ID:          uuid.New().String(),
		Requirement: requirement,
		Analysis:    result.Analysis,
		Tasks:       tasks,
		CreatedAt:   time.Now(),
		CreatedBy:   m.Model(),
	}, nil
}

const managerReplanPrompt = `You are a Project Manager Agent. A task has failed and needs replanning.

Original requirement: %s
Failed task: %s
Failure reason: %s

Analyze what went wrong and create a new plan to achieve the requirement.
You may split the failed task into smaller tasks or take a different approach.

Output JSON format:
{
  "analysis": "Analysis of the failure and new approach",
  "tasks": [
    {
      "id": "task-1",
      "title": "Task title",
      "description": "Task description",
      "priority": 1,
      "depends_on": [],
      "resources": []
    }
  ]
}`

// Replan creates a new plan after a failure
func (m *ManagerAgent) Replan(ctx context.Context, requirement, failedTask, failureReason string) (*workflow.TaskPlan, error) {
	prompt := fmt.Sprintf(managerReplanPrompt, requirement, failedTask, failureReason)

	var result struct {
		Analysis string `json:"analysis"`
		Tasks    []struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Priority    int      `json:"priority"`
			DependsOn   []string `json:"depends_on"`
			Resources   []string `json:"resources"`
		} `json:"tasks"`
	}

	if err := m.ChatJSON(ctx, prompt, "Please create a new plan.", &result); err != nil {
		return nil, fmt.Errorf("failed to replan: %w", err)
	}

	tasks := make([]*workflow.Task, len(result.Tasks))
	for i, t := range result.Tasks {
		tasks[i] = &workflow.Task{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Priority:    t.Priority,
			DependsOn:   t.DependsOn,
			Resources:   t.Resources,
			Status:      workflow.TaskStatusPending,
			CreatedAt:   time.Now(),
		}
	}

	return &workflow.TaskPlan{
		ID:          uuid.New().String(),
		Requirement: requirement,
		Analysis:    result.Analysis,
		Tasks:       tasks,
		CreatedAt:   time.Now(),
		CreatedBy:   m.Model(),
	}, nil
}

const managerReportPrompt = `You are a Project Manager generating a final report.

Given the execution results, generate a comprehensive summary including:
1. What was accomplished
2. Any issues encountered
3. Suggested next steps

Be concise but thorough. Focus on value delivered.`

// GenerateReport creates a final report
func (m *ManagerAgent) GenerateReport(ctx context.Context, plan *workflow.TaskPlan, eval *workflow.Evaluation) (*workflow.FinalReport, error) {
	// Build context for the report
	input := fmt.Sprintf(`Original requirement: %s

Analysis: %s

Tasks: %d total
%s

Evaluation:
- Meets requirement: %v
- Quality score: %d/100
- Strengths: %v
- Weaknesses: %v
- Suggestions: %v`,
		plan.Requirement,
		plan.Analysis,
		len(plan.Tasks),
		formatTaskSummaries(plan.Tasks),
		eval.MeetsRequirement,
		eval.QualityScore,
		eval.Strengths,
		eval.Weaknesses,
		eval.Suggestions,
	)

	conclusion, err := m.Chat(ctx, managerReportPrompt, input)
	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}

	// Calculate statistics
	var completed, failed, totalRetries int
	var totalDuration time.Duration
	summaries := make([]workflow.TaskSummary, len(plan.Tasks))

	for i, task := range plan.Tasks {
		switch task.Status {
		case workflow.TaskStatusCompleted:
			completed++
		case workflow.TaskStatusFailed, workflow.TaskStatusCancelled:
			failed++
		}
		totalRetries += task.RetryCount

		var duration time.Duration
		if task.StartedAt != nil && task.CompletedAt != nil {
			duration = task.CompletedAt.Sub(*task.StartedAt)
			totalDuration += duration
		}

		summaries[i] = workflow.TaskSummary{
			TaskID:     task.ID,
			Title:      task.Title,
			Status:     task.Status,
			RetryCount: task.RetryCount,
			Duration:   duration,
		}
	}

	status := "completed"
	if failed > 0 && completed > 0 {
		status = "partial"
	} else if failed > 0 && completed == 0 {
		status = "failed"
	}

	return &workflow.FinalReport{
		ID:            uuid.New().String(),
		PlanID:        plan.ID,
		Requirement:   plan.Requirement,
		Status:        status,
		TotalTasks:    len(plan.Tasks),
		Completed:     completed,
		Failed:        failed,
		TotalRetries:  totalRetries,
		TaskSummaries: summaries,
		Evaluation:    eval,
		Conclusion:    conclusion,
		TotalDuration: totalDuration,
		CreatedAt:     time.Now(),
	}, nil
}

func formatTaskSummaries(tasks []*workflow.Task) string {
	var result string
	for _, t := range tasks {
		status := "⬜"
		switch t.Status {
		case workflow.TaskStatusCompleted:
			status = "✅"
		case workflow.TaskStatusFailed:
			status = "❌"
		case workflow.TaskStatusCancelled:
			status = "⏹️"
		case workflow.TaskStatusRunning:
			status = "🔵"
		}
		result += fmt.Sprintf("  %s %s: %s\n", status, t.ID, t.Title)
	}
	return result
}
