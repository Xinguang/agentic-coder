package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// BashTool implements command execution
type BashTool struct {
	ShellPath      string
	MaxOutputLen   int
	DefaultTimeout time.Duration
}

// BashInput represents the input for Bash tool
type BashInput struct {
	Command                   string `json:"command"`
	Timeout                   int    `json:"timeout,omitempty"`
	Description               string `json:"description,omitempty"`
	RunInBackground           bool   `json:"run_in_background,omitempty"`
	DangerouslyDisableSandbox bool   `json:"dangerouslyDisableSandbox,omitempty"`
}

// BashOutput holds bash execution metadata
type BashOutput struct {
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exit_code"`
	Interrupted bool   `json:"interrupted"`
}

// NewBashTool creates a new Bash tool
func NewBashTool() *BashTool {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	return &BashTool{
		ShellPath:      shell,
		MaxOutputLen:   30000,
		DefaultTimeout: 2 * time.Minute,
	}
}

func (b *BashTool) Name() string {
	return "Bash"
}

func (b *BashTool) Description() string {
	return `Executes a given bash command in a persistent shell session with optional timeout.
- Always quote file paths that contain spaces
- The command argument is required
- You can specify an optional timeout in milliseconds (max 600000ms / 10 minutes)
- If the output exceeds 30000 characters, it will be truncated`
}

func (b *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The command to execute"
			},
			"timeout": {
				"type": "number",
				"description": "Optional timeout in milliseconds (max 600000)"
			},
			"description": {
				"type": "string",
				"description": "Clear, concise description of what this command does in 5-10 words"
			},
			"run_in_background": {
				"type": "boolean",
				"description": "Set to true to run this command in the background"
			}
		},
		"required": ["command"]
	}`)
}

func (b *BashTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[BashInput](input.Params)
	if err != nil {
		return err
	}

	if params.Command == "" {
		return fmt.Errorf("command is required")
	}

	if params.Timeout > 600000 {
		return fmt.Errorf("timeout cannot exceed 600000ms (10 minutes)")
	}

	return nil
}

func (b *BashTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[BashInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Set timeout
	timeout := b.DefaultTimeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, b.ShellPath, "-c", params.Command)

	// Set working directory
	if input.Context != nil && input.Context.CWD != "" {
		cmd.Dir = input.Context.CWD
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment
	cmd.Env = os.Environ()

	// Run command
	err = cmd.Run()

	// Get exit code
	exitCode := 0
	interrupted := false

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			interrupted = true
			exitCode = -1
		}
	}

	// Combine output
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Truncate if too long
	if len(output) > b.MaxOutputLen {
		output = output[:b.MaxOutputLen] + "\n... (output truncated)"
	}

	// Format result
	var content string
	if exitCode != 0 || interrupted {
		if interrupted {
			content = fmt.Sprintf("Command timed out after %v\n%s", timeout, output)
		} else {
			content = fmt.Sprintf("Exit code %d\n%s", exitCode, output)
		}
	} else {
		content = output
	}

	if strings.TrimSpace(content) == "" {
		content = "(no output)"
	}

	return &tool.Output{
		Content: content,
		IsError: exitCode != 0 || interrupted,
		Metadata: &BashOutput{
			Stdout:      stdout.String(),
			Stderr:      stderr.String(),
			ExitCode:    exitCode,
			Interrupted: interrupted,
		},
	}, nil
}
