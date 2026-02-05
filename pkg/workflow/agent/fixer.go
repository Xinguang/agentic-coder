package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/tool"
	"github.com/xinguang/agentic-coder/pkg/workflow"
)

// FixerAgent fixes issues identified during review
type FixerAgent struct {
	*BaseAgent
	engine *engine.Engine
}

// NewFixerAgent creates a new fixer agent
func NewFixerAgent(model string, eng *engine.Engine) *FixerAgent {
	return &FixerAgent{
		BaseAgent: NewBaseAgent(workflow.RoleFixer, model, nil),
		engine:    eng,
	}
}

const fixerPrompt = `You are a Fixer Agent responsible for fixing issues identified during code review.

Original Task: %s

Issues to fix:
%s

Fix Suggestion: %s

Instructions:
1. Focus only on fixing the identified issues
2. Do not make unrelated changes
3. Ensure fixes are correct and complete
4. Test your changes if possible

Begin fixing the issues now.`

// FixIssues fixes issues from a review
func (f *FixerAgent) FixIssues(ctx context.Context, task *workflow.Task, review *workflow.Review) (*workflow.Execution, error) {
	startTime := time.Now()

	exec := &workflow.Execution{
		ID:         uuid.New().String(),
		TaskID:     task.ID,
		ExecutorID: f.Model() + " (fixer)",
		StartedAt:  startTime,
		ToolsUsed:  make([]workflow.ToolUsage, 0),
	}

	// Format issues
	issuesStr := ""
	for _, issue := range review.Issues {
		issuesStr += fmt.Sprintf("- [%s/%s] %s", issue.Severity, issue.Type, issue.Description)
		if issue.Location != "" {
			issuesStr += fmt.Sprintf(" at %s", issue.Location)
		}
		if issue.Suggestion != "" {
			issuesStr += fmt.Sprintf("\n  Suggestion: %s", issue.Suggestion)
		}
		issuesStr += "\n"
	}

	// Track tool usage
	var toolUsages []workflow.ToolUsage
	var currentTool *workflow.ToolUsage
	var toolStartTime time.Time

	f.engine.SetCallbacks(&engine.CallbackOptions{
		OnToolUse: func(name string, input map[string]interface{}) {
			toolStartTime = time.Now()
			currentTool = &workflow.ToolUsage{
				Name:  name,
				Input: input,
			}
		},
		OnToolResult: func(name string, result *tool.Output) {
			if currentTool != nil {
				currentTool.Output = result.Content
				currentTool.Success = !result.IsError
				currentTool.Duration = time.Since(toolStartTime)
				toolUsages = append(toolUsages, *currentTool)

				if name == "Write" || name == "Edit" {
					if fp, ok := currentTool.Input["file_path"].(string); ok {
						exec.FilesChanged = append(exec.FilesChanged, fp)
					}
				}

				currentTool = nil
			}
		},
		OnText: func(text string) {
			exec.Output += text
		},
		OnError: func(err error) {
			exec.Error = err.Error()
		},
	})

	// Build prompt
	prompt := fmt.Sprintf(fixerPrompt, task.Title, issuesStr, review.FixSuggestion)

	// Execute
	err := f.engine.Run(ctx, prompt)

	exec.CompletedAt = time.Now()
	exec.Duration = exec.CompletedAt.Sub(startTime)
	exec.ToolsUsed = toolUsages

	if err != nil {
		exec.Success = false
		if exec.Error == "" {
			exec.Error = err.Error()
		}
		return exec, err
	}

	exec.Success = true
	return exec, nil
}

// FixerPool manages a pool of fixers
type FixerPool struct {
	fixers    []*FixerAgent
	available chan *FixerAgent
}

// NewFixerPool creates a pool of fixers
func NewFixerPool(size int, createEngine func() *engine.Engine, model string) *FixerPool {
	pool := &FixerPool{
		fixers:    make([]*FixerAgent, size),
		available: make(chan *FixerAgent, size),
	}

	for i := 0; i < size; i++ {
		fixer := NewFixerAgent(model, createEngine())
		pool.fixers[i] = fixer
		pool.available <- fixer
	}

	return pool
}

// Acquire gets a fixer from the pool
func (p *FixerPool) Acquire(ctx context.Context) (*FixerAgent, error) {
	select {
	case fixer := <-p.available:
		return fixer, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a fixer to the pool
func (p *FixerPool) Release(fixer *FixerAgent) {
	select {
	case p.available <- fixer:
	default:
	}
}

// Available returns the number of available fixers
func (p *FixerPool) Available() int {
	return len(p.available)
}
