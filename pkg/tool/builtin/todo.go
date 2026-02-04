package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/session"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// TodoWriteTool manages the todo list
type TodoWriteTool struct {
	session *session.Session
}

// TodoItem represents a single todo item
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending, in_progress, completed
	ActiveForm string `json:"activeForm"`
}

// TodoWriteInput represents the input for TodoWrite tool
type TodoWriteInput struct {
	Todos []TodoItem `json:"todos"`
}

// NewTodoWriteTool creates a new TodoWrite tool
func NewTodoWriteTool(sess *session.Session) *TodoWriteTool {
	return &TodoWriteTool{session: sess}
}

func (t *TodoWriteTool) Name() string {
	return "TodoWrite"
}

func (t *TodoWriteTool) Description() string {
	return `Use this tool to create and manage a structured task list.

When to use:
1. Complex multi-step tasks (3+ steps)
2. User provides multiple tasks
3. User explicitly requests todo list

When NOT to use:
1. Single trivial task
2. Task can be completed in <3 steps

Task states:
- pending: Not yet started
- in_progress: Currently working on
- completed: Task finished

Important:
- Mark tasks complete IMMEDIATELY after finishing
- Only ONE task should be in_progress at a time
- Provide both content (imperative) and activeForm (present continuous)`
}

func (t *TodoWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"todos": {
				"type": "array",
				"description": "The updated todo list",
				"items": {
					"type": "object",
					"properties": {
						"content": {
							"type": "string",
							"minLength": 1
						},
						"status": {
							"type": "string",
							"enum": ["pending", "in_progress", "completed"]
						},
						"activeForm": {
							"type": "string",
							"minLength": 1
						}
					},
					"required": ["content", "status", "activeForm"]
				}
			}
		},
		"required": ["todos"]
	}`)
}

func (t *TodoWriteTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[TodoWriteInput](input.Params)
	if err != nil {
		return err
	}

	for i, todo := range params.Todos {
		if todo.Content == "" {
			return fmt.Errorf("todo[%d].content is required", i)
		}
		if todo.Status == "" {
			return fmt.Errorf("todo[%d].status is required", i)
		}
		if todo.ActiveForm == "" {
			return fmt.Errorf("todo[%d].activeForm is required", i)
		}

		// Validate status
		switch todo.Status {
		case "pending", "in_progress", "completed":
			// valid
		default:
			return fmt.Errorf("todo[%d].status must be pending, in_progress, or completed", i)
		}
	}

	return nil
}

func (t *TodoWriteTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[TodoWriteInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Convert to session.Todo format
	todos := make([]session.Todo, len(params.Todos))
	for i, item := range params.Todos {
		todos[i] = session.Todo{
			Content:    item.Content,
			Status:     item.Status,
			ActiveForm: item.ActiveForm,
		}
	}

	// Update session todos
	t.session.UpdateTodos(todos)

	// Build summary
	var pending, inProgress, completed int
	for _, todo := range params.Todos {
		switch todo.Status {
		case "pending":
			pending++
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		}
	}

	return &tool.Output{
		Content: fmt.Sprintf("Todos updated: %d pending, %d in progress, %d completed",
			pending, inProgress, completed),
		Metadata: map[string]interface{}{
			"pending":     pending,
			"in_progress": inProgress,
			"completed":   completed,
			"total":       len(params.Todos),
		},
	}, nil
}

// TodoReadTool reads the current todo list (for subagents/reference)
type TodoReadTool struct {
	session *session.Session
}

// NewTodoReadTool creates a new TodoRead tool
func NewTodoReadTool(sess *session.Session) *TodoReadTool {
	return &TodoReadTool{session: sess}
}

func (t *TodoReadTool) Name() string {
	return "TodoRead"
}

func (t *TodoReadTool) Description() string {
	return "Reads the current todo list to check task status and progress."
}

func (t *TodoReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`)
}

func (t *TodoReadTool) Validate(input *tool.Input) error {
	return nil
}

func (t *TodoReadTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	todos := t.session.Todos

	if len(todos) == 0 {
		return &tool.Output{
			Content: "No todos in the list",
		}, nil
	}

	// Format as readable list
	var sb strings.Builder
	sb.WriteString("Current todos:\n")

	for i, todo := range todos {
		status := "[ ]"
		switch todo.Status {
		case "in_progress":
			status = "[→]"
		case "completed":
			status = "[✓]"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, status, todo.Content))
	}

	return &tool.Output{
		Content: sb.String(),
		Metadata: map[string]interface{}{
			"todos": todos,
		},
	}, nil
}
