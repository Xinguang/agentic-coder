package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// EditTool implements file editing via string replacement
type EditTool struct {
	// FileHistory tracks file changes for undo (optional)
	FileHistory FileHistory
}

// FileHistory interface for tracking file history
type FileHistory interface {
	Save(path, content string) error
}

// EditInput represents the input for Edit tool
type EditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// NewEditTool creates a new Edit tool
func NewEditTool() *EditTool {
	return &EditTool{}
}

func (e *EditTool) Name() string {
	return "Edit"
}

func (e *EditTool) Description() string {
	return `Performs exact string replacements in files.
- You must use your Read tool at least once before editing.
- The edit will FAIL if old_string is not unique in the file. Either provide more context or use replace_all.
- Use replace_all for replacing and renaming strings across the file.`
}

func (e *EditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The absolute path to the file to modify"
			},
			"old_string": {
				"type": "string",
				"description": "The text to replace"
			},
			"new_string": {
				"type": "string",
				"description": "The text to replace it with (must be different from old_string)"
			},
			"replace_all": {
				"type": "boolean",
				"default": false,
				"description": "Replace all occurrences of old_string (default false)"
			}
		},
		"required": ["file_path", "old_string", "new_string"]
	}`)
}

func (e *EditTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[EditInput](input.Params)
	if err != nil {
		return err
	}

	if params.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}

	if params.OldString == "" {
		return fmt.Errorf("old_string is required")
	}

	if params.OldString == params.NewString {
		return fmt.Errorf("old_string and new_string must be different")
	}

	return nil
}

func (e *EditTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[EditInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Read file
	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &tool.Output{
				Content: fmt.Sprintf("Error: File not found: %s", params.FilePath),
				IsError: true,
			}, nil
		}
		return &tool.Output{
			Content: fmt.Sprintf("Error reading file: %v", err),
			IsError: true,
		}, nil
	}

	oldContent := string(content)

	// Check if old_string exists and is unique
	count := strings.Count(oldContent, params.OldString)
	if count == 0 {
		return &tool.Output{
			Content: "Error: old_string not found in file. Make sure the string matches exactly, including whitespace and indentation.",
			IsError: true,
		}, nil
	}

	if count > 1 && !params.ReplaceAll {
		return &tool.Output{
			Content: fmt.Sprintf("Error: old_string found %d times in file. Either provide more surrounding context to make it unique, or use replace_all to change all occurrences.", count),
			IsError: true,
		}, nil
	}

	// Save to file history if available
	if e.FileHistory != nil {
		if err := e.FileHistory.Save(params.FilePath, oldContent); err != nil {
			// Log but don't fail
		}
	}

	// Perform replacement
	var newContent string
	if params.ReplaceAll {
		newContent = strings.ReplaceAll(oldContent, params.OldString, params.NewString)
	} else {
		newContent = strings.Replace(oldContent, params.OldString, params.NewString, 1)
	}

	// Write back
	if err := os.WriteFile(params.FilePath, []byte(newContent), 0644); err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Error writing file: %v", err),
			IsError: true,
		}, nil
	}

	replacements := 1
	if params.ReplaceAll {
		replacements = count
	}

	return &tool.Output{
		Content: fmt.Sprintf("Successfully edited %s (%d replacement(s) made)", params.FilePath, replacements),
		Metadata: map[string]interface{}{
			"replacements": replacements,
		},
	}, nil
}
