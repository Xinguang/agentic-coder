package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// WriteTool implements file writing
type WriteTool struct{}

// WriteInput represents the input for Write tool
type WriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// NewWriteTool creates a new Write tool
func NewWriteTool() *WriteTool {
	return &WriteTool{}
}

func (w *WriteTool) Name() string {
	return "Write"
}

func (w *WriteTool) Description() string {
	return `Writes a file to the local filesystem.
- This tool will overwrite the existing file if there is one at the provided path.
- If this is an existing file, you MUST use the Read tool first to read the file's contents.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.`
}

func (w *WriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The absolute path to the file to write (must be absolute, not relative)"
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file"
			}
		},
		"required": ["file_path", "content"]
	}`)
}

func (w *WriteTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[WriteInput](input.Params)
	if err != nil {
		return err
	}

	if params.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}

	// Use secure path validation
	if err := ValidateSecurePath(params.FilePath); err != nil {
		return err
	}

	return nil
}

func (w *WriteTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[WriteInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Create directory if not exists
	dir := filepath.Dir(params.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Error creating directory: %v", err),
			IsError: true,
		}, nil
	}

	// Write file
	if err := os.WriteFile(params.FilePath, []byte(params.Content), 0644); err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Error writing file: %v", err),
			IsError: true,
		}, nil
	}

	return &tool.Output{
		Content: fmt.Sprintf("Successfully wrote %d bytes to %s", len(params.Content), params.FilePath),
	}, nil
}
