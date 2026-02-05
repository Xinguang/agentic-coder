package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xinguang/agentic-coder/pkg/provider"
)

// EvaluatorAgent evaluates the overall workflow result
type EvaluatorAgent struct {
	*BaseAgent
}

// NewEvaluatorAgent creates a new evaluator agent
func NewEvaluatorAgent(model string, prov provider.AIProvider) *EvaluatorAgent {
	return &EvaluatorAgent{
		BaseAgent: NewBaseAgent(RoleEvaluator, model, prov),
	}
}

const evaluatorPrompt = `You are an Evaluator Agent responsible for assessing the overall quality of a completed workflow.

Compare the final result against the original requirement and evaluate:
1. Does it meet the requirement?
2. What is the quality of the implementation?
3. What are the strengths?
4. What are the weaknesses?
5. What improvements could be made?

Output JSON format:
{
  "meets_requirement": true | false,
  "quality_score": 0-100,
  "strengths": ["list of strengths"],
  "weaknesses": ["list of weaknesses"],
  "suggestions": ["suggestions for improvement"]
}

Scoring guide:
- 90-100: Excellent - exceeds expectations
- 80-89: Very good - meets all requirements with quality
- 70-79: Good - meets requirements
- 60-69: Acceptable - meets most requirements
- 50-59: Below average - missing some requirements
- <50: Poor - significant issues

Be objective and constructive in your evaluation.`

// Evaluate evaluates the overall workflow result
func (e *EvaluatorAgent) Evaluate(ctx context.Context, plan *TaskPlan) (*Evaluation, error) {
	// Build evaluation context
	input := fmt.Sprintf(`Original Requirement: %s

Analysis: %s

Task Results:
%s

Overall Statistics:
- Total tasks: %d
- Completed: %d
- Failed: %d`,
		plan.Requirement,
		plan.Analysis,
		formatTaskResults(plan.Tasks),
		len(plan.Tasks),
		countByStatus(plan.Tasks, TaskStatusCompleted),
		countByStatus(plan.Tasks, TaskStatusFailed)+countByStatus(plan.Tasks, TaskStatusCancelled),
	)

	var result struct {
		MeetsRequirement bool     `json:"meets_requirement"`
		QualityScore     int      `json:"quality_score"`
		Strengths        []string `json:"strengths"`
		Weaknesses       []string `json:"weaknesses"`
		Suggestions      []string `json:"suggestions"`
	}

	if err := e.ChatJSON(ctx, evaluatorPrompt, input, &result); err != nil {
		return nil, fmt.Errorf("failed to evaluate: %w", err)
	}

	return &Evaluation{
		ID:               uuid.New().String(),
		PlanID:           plan.ID,
		EvaluatorID:      e.Model(),
		MeetsRequirement: result.MeetsRequirement,
		QualityScore:     result.QualityScore,
		Strengths:        result.Strengths,
		Weaknesses:       result.Weaknesses,
		Suggestions:      result.Suggestions,
		CreatedAt:        time.Now(),
	}, nil
}

func formatTaskResults(tasks []*Task) string {
	result := ""
	for _, task := range tasks {
		status := string(task.Status)
		result += fmt.Sprintf("\n[%s] %s: %s", status, task.ID, task.Title)

		if task.Execution != nil {
			result += fmt.Sprintf("\n  - Duration: %s", task.Execution.Duration)
			result += fmt.Sprintf("\n  - Tools used: %d", len(task.Execution.ToolsUsed))
			result += fmt.Sprintf("\n  - Files changed: %v", task.Execution.FilesChanged)
			if task.Execution.Error != "" {
				result += fmt.Sprintf("\n  - Error: %s", task.Execution.Error)
			}
		}

		if len(task.Reviews) > 0 {
			lastReview := task.Reviews[len(task.Reviews)-1]
			result += fmt.Sprintf("\n  - Review: %s (score: %d)", lastReview.Result, lastReview.Score)
		}

		result += "\n"
	}
	return result
}

func countByStatus(tasks []*Task, status TaskStatus) int {
	count := 0
	for _, task := range tasks {
		if task.Status == status {
			count++
		}
	}
	return count
}
