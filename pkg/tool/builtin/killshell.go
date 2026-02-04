// Package builtin provides built-in tools
package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// KillShellTool kills a running background shell
type KillShellTool struct {
	manager *ShellManager
}

// NewKillShellTool creates a new kill shell tool
func NewKillShellTool(manager *ShellManager) *KillShellTool {
	return &KillShellTool{
		manager: manager,
	}
}

func (t *KillShellTool) Name() string {
	return "KillShell"
}

func (t *KillShellTool) Description() string {
	return `Kills a running background bash shell by its ID.

- Takes a shell_id parameter identifying the shell to kill
- Returns a success or failure status
- Use this tool when you need to terminate a long-running shell
- Shell IDs can be found using the /tasks command`
}

func (t *KillShellTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"shell_id": {
				"type": "string",
				"description": "The ID of the background shell to kill"
			}
		},
		"required": ["shell_id"]
	}`)
}

type killShellParams struct {
	ShellID string `json:"shell_id"`
}

func (t *KillShellTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[killShellParams](input.Params)
	if err != nil {
		return err
	}

	if params.ShellID == "" {
		return fmt.Errorf("shell_id is required")
	}

	return nil
}

func (t *KillShellTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[killShellParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	if t.manager == nil {
		return &tool.Output{Content: "Shell manager not configured", IsError: true}, nil
	}

	shell := t.manager.Get(params.ShellID)
	if shell == nil {
		return &tool.Output{
			Content: fmt.Sprintf("Shell '%s' not found. Use /tasks to list running shells.", params.ShellID),
			IsError: true,
		}, nil
	}

	if err := t.manager.Kill(params.ShellID); err != nil {
		return &tool.Output{Content: fmt.Sprintf("Failed to kill shell: %v", err), IsError: true}, nil
	}

	return &tool.Output{
		Content: fmt.Sprintf("Successfully killed shell '%s' (command: %s)", params.ShellID, shell.Command),
		Metadata: map[string]interface{}{
			"shell_id": params.ShellID,
			"command":  shell.Command,
			"duration": shell.Duration().String(),
		},
	}, nil
}
