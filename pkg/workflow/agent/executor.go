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

// ExecutorAgent executes tasks using tools
type ExecutorAgent struct {
	*BaseAgent
	engine *engine.Engine
}

// NewExecutorAgent creates a new executor agent
func NewExecutorAgent(model string, eng *engine.Engine) *ExecutorAgent {
	return &ExecutorAgent{
		BaseAgent: NewBaseAgent(workflow.RoleExecutor, model, nil),
		engine:    eng,
	}
}

const executorPrompt = `You are an Executor Agent responsible for completing a specific task.

Task: %s

Description: %s

Instructions:
1. Use the available tools to complete this task
2. Focus only on this specific task
3. Do not make changes beyond what is required
4. Report clearly when the task is complete

Begin executing the task now.`

// ExecuteTask executes a single task
func (e *ExecutorAgent) ExecuteTask(ctx context.Context, task *workflow.Task) (*workflow.Execution, error) {
	startTime := time.Now()

	exec := &workflow.Execution{
		ID:         uuid.New().String(),
		TaskID:     task.ID,
		ExecutorID: e.Model(),
		StartedAt:  startTime,
		ToolsUsed:  make([]workflow.ToolUsage, 0),
	}

	// Track tool usage
	var toolUsages []workflow.ToolUsage
	var currentTool *workflow.ToolUsage
	var toolStartTime time.Time

	e.engine.SetCallbacks(&engine.CallbackOptions{
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

				// Track files changed
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
	prompt := fmt.Sprintf(executorPrompt, task.Title, task.Description)

	// Execute
	err := e.engine.Run(ctx, prompt)

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

// ExecutorPool manages a pool of executors
type ExecutorPool struct {
	executors []*ExecutorAgent
	available chan *ExecutorAgent
}

// NewExecutorPool creates a pool of executors
func NewExecutorPool(size int, createEngine func() *engine.Engine, model string) *ExecutorPool {
	pool := &ExecutorPool{
		executors: make([]*ExecutorAgent, size),
		available: make(chan *ExecutorAgent, size),
	}

	for i := 0; i < size; i++ {
		executor := NewExecutorAgent(model, createEngine())
		pool.executors[i] = executor
		pool.available <- executor
	}

	return pool
}

// Acquire gets an executor from the pool
func (p *ExecutorPool) Acquire(ctx context.Context) (*ExecutorAgent, error) {
	select {
	case executor := <-p.available:
		return executor, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns an executor to the pool
func (p *ExecutorPool) Release(executor *ExecutorAgent) {
	select {
	case p.available <- executor:
	default:
		// Pool is full, this shouldn't happen
	}
}

// Size returns the pool size
func (p *ExecutorPool) Size() int {
	return len(p.executors)
}

// Available returns the number of available executors
func (p *ExecutorPool) Available() int {
	return len(p.available)
}
