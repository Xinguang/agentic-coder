package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/task"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// TaskTool allows spawning subagents
type TaskTool struct {
	manager *task.Manager
}

// TaskInput represents the input for Task tool
type TaskInput struct {
	Description     string `json:"description"`
	Prompt          string `json:"prompt"`
	SubagentType    string `json:"subagent_type"`
	Model           string `json:"model,omitempty"`
	RunInBackground bool   `json:"run_in_background,omitempty"`
	Resume          string `json:"resume,omitempty"` // Task ID to resume
}

// NewTaskTool creates a new Task tool
func NewTaskTool(manager *task.Manager) *TaskTool {
	return &TaskTool{manager: manager}
}

func (t *TaskTool) Name() string {
	return "Task"
}

func (t *TaskTool) Description() string {
	// Build agent descriptions
	agents := t.manager.ListAgents()
	var agentDescs []string
	for _, a := range agents {
		agentDescs = append(agentDescs, fmt.Sprintf("- %s: %s", a.Name, a.Description))
	}

	return fmt.Sprintf(`Launch a new agent to handle complex, multi-step tasks autonomously.

Available agent types:
%s

Usage:
- Use subagent_type to select which agent type to use
- Include a short description (3-5 words) summarizing what the agent will do
- Launch multiple agents concurrently when possible
- Use run_in_background to run async and check results later with TaskOutput
- Agents can be resumed using the resume parameter by passing a previous task ID`, strings.Join(agentDescs, "\n"))
}

func (t *TaskTool) InputSchema() json.RawMessage {
	// Build enum of available agent types
	agents := t.manager.ListAgents()
	agentTypes := make([]string, len(agents))
	for i, a := range agents {
		agentTypes[i] = fmt.Sprintf(`"%s"`, a.Name)
	}

	return json.RawMessage(fmt.Sprintf(`{
		"type": "object",
		"properties": {
			"description": {
				"type": "string",
				"description": "A short (3-5 word) description of the task"
			},
			"prompt": {
				"type": "string",
				"description": "The task for the agent to perform"
			},
			"subagent_type": {
				"type": "string",
				"description": "The type of specialized agent to use",
				"enum": [%s]
			},
			"model": {
				"type": "string",
				"description": "Optional model override (sonnet, opus, haiku)",
				"enum": ["sonnet", "opus", "haiku"]
			},
			"run_in_background": {
				"type": "boolean",
				"description": "Run this agent in the background"
			},
			"resume": {
				"type": "string",
				"description": "Optional task ID to resume from"
			}
		},
		"required": ["description", "prompt", "subagent_type"]
	}`, strings.Join(agentTypes, ", ")))
}

func (t *TaskTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[TaskInput](input.Params)
	if err != nil {
		return err
	}

	if params.Description == "" {
		return fmt.Errorf("description is required")
	}

	if params.Prompt == "" && params.Resume == "" {
		return fmt.Errorf("prompt is required (unless resuming)")
	}

	if params.SubagentType == "" {
		return fmt.Errorf("subagent_type is required")
	}

	// Validate agent type exists
	if t.manager.GetAgentConfig(params.SubagentType) == nil {
		return fmt.Errorf("unknown subagent_type: %s", params.SubagentType)
	}

	return nil
}

func (t *TaskTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[TaskInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Handle resume
	if params.Resume != "" {
		existingTask := t.manager.GetTask(params.Resume)
		if existingTask == nil {
			return &tool.Output{
				Content: fmt.Sprintf("Task %s not found", params.Resume),
				IsError: true,
			}, nil
		}

		// Check if task is still running
		if existingTask.Status == task.TaskStatusRunning {
			return &tool.Output{
				Content: fmt.Sprintf("Task %s is still running", params.Resume),
			}, nil
		}

		// Return result if completed
		if existingTask.Status == task.TaskStatusCompleted && existingTask.Result != nil {
			return &tool.Output{
				Content: existingTask.Result.Output,
				Metadata: map[string]interface{}{
					"task_id": existingTask.ID,
					"status":  string(existingTask.Status),
				},
			}, nil
		}

		return &tool.Output{
			Content: fmt.Sprintf("Task %s status: %s", params.Resume, existingTask.Status),
			Metadata: map[string]interface{}{
				"task_id": existingTask.ID,
				"status":  string(existingTask.Status),
				"error":   existingTask.Error,
			},
		}, nil
	}

	// Create new task
	newTask, err := t.manager.CreateTask(&task.TaskOptions{
		Description:     params.Description,
		Prompt:          params.Prompt,
		SubagentType:    params.SubagentType,
		Model:           params.Model,
		RunInBackground: params.RunInBackground,
	})
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Failed to create task: %v", err),
			IsError: true,
		}, nil
	}

	// Run in background or synchronously
	if params.RunInBackground {
		t.manager.RunTaskAsync(ctx, newTask.ID)
		return &tool.Output{
			Content: fmt.Sprintf("Task %s started in background. Use TaskOutput to check results.", newTask.ID),
			Metadata: map[string]interface{}{
				"task_id": newTask.ID,
				"status":  "running",
			},
		}, nil
	}

	// Run synchronously
	err = t.manager.RunTask(ctx, newTask.ID)

	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Task failed: %v", err),
			IsError: true,
			Metadata: map[string]interface{}{
				"task_id": newTask.ID,
				"status":  string(newTask.Status),
			},
		}, nil
	}

	// Return result
	if newTask.Result != nil {
		return &tool.Output{
			Content: newTask.Result.Output,
			Metadata: map[string]interface{}{
				"task_id": newTask.ID,
				"status":  string(newTask.Status),
			},
		}, nil
	}

	return &tool.Output{
		Content: "Task completed with no output",
		Metadata: map[string]interface{}{
			"task_id": newTask.ID,
			"status":  string(newTask.Status),
		},
	}, nil
}
