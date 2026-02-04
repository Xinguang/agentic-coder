package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xinguang/agentic-coder/pkg/task"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// TaskOutputTool retrieves output from tasks
type TaskOutputTool struct {
	manager *task.Manager
}

// TaskOutputInput represents the input for TaskOutput tool
type TaskOutputInput struct {
	TaskID  string `json:"task_id"`
	Block   bool   `json:"block"`
	Timeout int    `json:"timeout,omitempty"` // milliseconds
}

// NewTaskOutputTool creates a new TaskOutput tool
func NewTaskOutputTool(manager *task.Manager) *TaskOutputTool {
	return &TaskOutputTool{manager: manager}
}

func (t *TaskOutputTool) Name() string {
	return "TaskOutput"
}

func (t *TaskOutputTool) Description() string {
	return `Retrieves output from a running or completed task.
- Takes a task_id parameter identifying the task
- Returns the task output along with status information
- Use block=true (default) to wait for task completion
- Use block=false for non-blocking check of current status`
}

func (t *TaskOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task_id": {
				"type": "string",
				"description": "The task ID to get output from"
			},
			"block": {
				"type": "boolean",
				"default": true,
				"description": "Whether to wait for completion"
			},
			"timeout": {
				"type": "number",
				"default": 30000,
				"description": "Max wait time in ms",
				"minimum": 0,
				"maximum": 600000
			}
		},
		"required": ["task_id"]
	}`)
}

func (t *TaskOutputTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[TaskOutputInput](input.Params)
	if err != nil {
		return err
	}

	if params.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}

	return nil
}

func (t *TaskOutputTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[TaskOutputInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Get task
	taskObj := t.manager.GetTask(params.TaskID)
	if taskObj == nil {
		return &tool.Output{
			Content: fmt.Sprintf("Task %s not found", params.TaskID),
			IsError: true,
		}, nil
	}

	// Set defaults
	block := true
	if !params.Block {
		block = params.Block
	}

	timeout := 30 * time.Second
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Millisecond
	}

	// If not blocking, just return current status
	if !block {
		return t.formatResult(taskObj), nil
	}

	// Wait for completion
	result, err := t.manager.WaitForTask(ctx, params.TaskID, timeout)
	if err != nil {
		// Get fresh task status
		taskObj = t.manager.GetTask(params.TaskID)
		return &tool.Output{
			Content: fmt.Sprintf("Task error: %v", err),
			IsError: true,
			Metadata: map[string]interface{}{
				"task_id": params.TaskID,
				"status":  string(taskObj.Status),
				"error":   taskObj.Error,
			},
		}, nil
	}

	return &tool.Output{
		Content: result.Output,
		Metadata: map[string]interface{}{
			"task_id": params.TaskID,
			"status":  "completed",
		},
	}, nil
}

func (t *TaskOutputTool) formatResult(taskObj *task.Task) *tool.Output {
	content := fmt.Sprintf("Task %s status: %s", taskObj.ID, taskObj.Status)

	metadata := map[string]interface{}{
		"task_id": taskObj.ID,
		"status":  string(taskObj.Status),
	}

	if taskObj.Status == task.TaskStatusCompleted && taskObj.Result != nil {
		content = taskObj.Result.Output
	}

	if taskObj.Status == task.TaskStatusFailed {
		metadata["error"] = taskObj.Error
	}

	return &tool.Output{
		Content:  content,
		Metadata: metadata,
	}
}
