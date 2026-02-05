package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/workflow"
)

// ReviewerAgent reviews task execution results
type ReviewerAgent struct {
	*BaseAgent
}

// NewReviewerAgent creates a new reviewer agent
func NewReviewerAgent(model string, prov provider.AIProvider) *ReviewerAgent {
	return &ReviewerAgent{
		BaseAgent: NewBaseAgent(workflow.RoleReviewer, model, prov),
	}
}

const reviewerPrompt = `You are a Code Reviewer Agent responsible for reviewing task execution results.

Review the execution and check:
1. Correctness - Does the execution accomplish the task?
2. Security - Are there any security issues?
3. Best practices - Does it follow coding conventions?
4. Completeness - Is anything missing?

Output JSON format:
{
  "result": "pass" | "fail" | "replan",
  "score": 0-100,
  "comments": "Overall assessment",
  "issues": [
    {
      "severity": "critical" | "major" | "minor",
      "type": "bug" | "security" | "style" | "performance",
      "description": "What's wrong",
      "location": "file:line if applicable",
      "suggestion": "How to fix"
    }
  ],
  "can_auto_fix": true | false,
  "fix_suggestion": "If can_auto_fix is true, describe the fix"
}

Rules:
- "pass" if no critical or major issues
- "fail" if issues can be fixed with a retry
- "replan" if fundamental problems require a new approach
- Score 90+ for excellent, 70-89 for good, 50-69 for acceptable, <50 for poor`

// ReviewExecution reviews a task execution
func (r *ReviewerAgent) ReviewExecution(ctx context.Context, task *workflow.Task, exec *workflow.Execution) (*workflow.Review, error) {
	input := fmt.Sprintf(`Task: %s
Description: %s

Execution Result:
- Success: %v
- Duration: %s
- Tools used: %d
- Files changed: %v
- Output: %s
- Error: %s`,
		task.Title,
		task.Description,
		exec.Success,
		exec.Duration,
		len(exec.ToolsUsed),
		exec.FilesChanged,
		truncate(exec.Output, 2000),
		exec.Error,
	)

	// Add tool usage details
	if len(exec.ToolsUsed) > 0 {
		input += "\n\nTool Usage Details:"
		for i, tu := range exec.ToolsUsed {
			if i >= 10 {
				input += fmt.Sprintf("\n... and %d more tools", len(exec.ToolsUsed)-10)
				break
			}
			input += fmt.Sprintf("\n- %s: success=%v", tu.Name, tu.Success)
		}
	}

	var result struct {
		Result        string            `json:"result"`
		Score         int               `json:"score"`
		Comments      string            `json:"comments"`
		Issues        []workflow.Issue  `json:"issues"`
		CanAutoFix    bool              `json:"can_auto_fix"`
		FixSuggestion string            `json:"fix_suggestion"`
	}

	if err := r.ChatJSON(ctx, reviewerPrompt, input, &result); err != nil {
		return nil, fmt.Errorf("failed to review execution: %w", err)
	}

	return &workflow.Review{
		ID:            uuid.New().String(),
		ExecutionID:   exec.ID,
		ReviewerID:    r.Model(),
		Result:        workflow.ReviewResult(result.Result),
		Score:         result.Score,
		Comments:      result.Comments,
		Issues:        result.Issues,
		CanAutoFix:    result.CanAutoFix,
		FixSuggestion: result.FixSuggestion,
		CreatedAt:     time.Now(),
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ReviewerPool manages a pool of reviewers
type ReviewerPool struct {
	reviewers []*ReviewerAgent
	available chan *ReviewerAgent
}

// NewReviewerPool creates a pool of reviewers
func NewReviewerPool(size int, model string, prov provider.AIProvider) *ReviewerPool {
	pool := &ReviewerPool{
		reviewers: make([]*ReviewerAgent, size),
		available: make(chan *ReviewerAgent, size),
	}

	for i := 0; i < size; i++ {
		reviewer := NewReviewerAgent(model, prov)
		pool.reviewers[i] = reviewer
		pool.available <- reviewer
	}

	return pool
}

// Acquire gets a reviewer from the pool
func (p *ReviewerPool) Acquire(ctx context.Context) (*ReviewerAgent, error) {
	select {
	case reviewer := <-p.available:
		return reviewer, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a reviewer to the pool
func (p *ReviewerPool) Release(reviewer *ReviewerAgent) {
	select {
	case p.available <- reviewer:
	default:
	}
}

// Available returns the number of available reviewers
func (p *ReviewerPool) Available() int {
	return len(p.available)
}
