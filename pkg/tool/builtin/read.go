// Package builtin provides built-in tool implementations
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// ReadTool implements file reading
type ReadTool struct {
	MaxLines   int
	MaxLineLen int
}

// ReadInput represents the input for Read tool
type ReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// NewReadTool creates a new Read tool
func NewReadTool() *ReadTool {
	return &ReadTool{
		MaxLines:   2000,
		MaxLineLen: 2000,
	}
}

func (r *ReadTool) Name() string {
	return "Read"
}

func (r *ReadTool) Description() string {
	return `Reads a file from the local filesystem. You can access any file directly by using this tool.
- The file_path parameter must be an absolute path, not a relative path
- By default, it reads up to 2000 lines starting from the beginning of the file
- You can optionally specify a line offset and limit
- Results are returned with line numbers starting at 1
- This tool can read images, PDFs, and Jupyter notebooks`
}

func (r *ReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The absolute path to the file to read"
			},
			"offset": {
				"type": "number",
				"description": "The line number to start reading from. Only provide if the file is too large to read at once"
			},
			"limit": {
				"type": "number",
				"description": "The number of lines to read. Only provide if the file is too large to read at once"
			}
		},
		"required": ["file_path"]
	}`)
}

func (r *ReadTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[ReadInput](input.Params)
	if err != nil {
		return err
	}

	if params.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}

	if !strings.HasPrefix(params.FilePath, "/") {
		return fmt.Errorf("file_path must be an absolute path")
	}

	return nil
}

func (r *ReadTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[ReadInput](input.Params)
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

	// Handle binary files (simple check)
	if isBinary(content) {
		return &tool.Output{
			Content: fmt.Sprintf("Binary file: %s (%d bytes)", params.FilePath, len(content)),
		}, nil
	}

	// Split into lines
	lines := strings.Split(string(content), "\n")

	// Apply offset and limit
	offset := params.Offset
	if offset > 0 {
		offset-- // Convert to 0-based
	}
	if offset >= len(lines) {
		return &tool.Output{
			Content: "Offset exceeds file length",
			IsError: true,
		}, nil
	}

	limit := params.Limit
	if limit == 0 {
		limit = r.MaxLines
	}

	endLine := offset + limit
	if endLine > len(lines) {
		endLine = len(lines)
	}

	lines = lines[offset:endLine]

	// Format with line numbers
	result := r.formatWithLineNumbers(lines, offset+1)

	return &tool.Output{
		Content: result,
		Metadata: map[string]interface{}{
			"lines_read": len(lines),
			"total_lines": len(strings.Split(string(content), "\n")),
		},
	}, nil
}

func (r *ReadTool) formatWithLineNumbers(lines []string, startLine int) string {
	var buf strings.Builder

	for i, line := range lines {
		lineNum := startLine + i

		// Truncate long lines
		if len(line) > r.MaxLineLen {
			line = line[:r.MaxLineLen] + "..."
		}

		fmt.Fprintf(&buf, "%6d\t%s\n", lineNum, line)
	}

	return buf.String()
}

// isBinary checks if content appears to be binary
func isBinary(content []byte) bool {
	// Check first 512 bytes for null bytes
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}

	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}

	return false
}
