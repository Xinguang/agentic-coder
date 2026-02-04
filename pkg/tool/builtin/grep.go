package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// GrepTool implements content searching using ripgrep
type GrepTool struct {
	RipgrepPath string
}

// GrepInput represents the input for Grep tool
type GrepInput struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	OutputMode string `json:"output_mode,omitempty"` // content, files_with_matches, count
	Before     int    `json:"-B,omitempty"`
	After      int    `json:"-A,omitempty"`
	Context    int    `json:"-C,omitempty"`
	LineNum    bool   `json:"-n,omitempty"`
	IgnoreCase bool   `json:"-i,omitempty"`
	Type       string `json:"type,omitempty"`
	HeadLimit  int    `json:"head_limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Multiline  bool   `json:"multiline,omitempty"`
}

// NewGrepTool creates a new Grep tool
func NewGrepTool() *GrepTool {
	// Try to find ripgrep
	path, err := exec.LookPath("rg")
	if err != nil {
		path = "rg" // fallback, may fail at runtime
	}

	return &GrepTool{
		RipgrepPath: path,
	}
}

func (g *GrepTool) Name() string {
	return "Grep"
}

func (g *GrepTool) Description() string {
	return `A powerful search tool built on ripgrep.
- Supports full regex syntax (e.g., "log.*Error", "function\\s+\\w+")
- Filter files with glob parameter (e.g., "*.js", "**/*.tsx") or type parameter (e.g., "js", "py")
- Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts`
}

func (g *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "The regular expression pattern to search for in file contents"
			},
			"path": {
				"type": "string",
				"description": "File or directory to search in. Defaults to current working directory."
			},
			"glob": {
				"type": "string",
				"description": "Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\")"
			},
			"output_mode": {
				"type": "string",
				"enum": ["content", "files_with_matches", "count"],
				"description": "Output mode. Defaults to files_with_matches."
			},
			"-B": {
				"type": "number",
				"description": "Number of lines to show before each match"
			},
			"-A": {
				"type": "number",
				"description": "Number of lines to show after each match"
			},
			"-C": {
				"type": "number",
				"description": "Number of lines to show before and after each match"
			},
			"-i": {
				"type": "boolean",
				"description": "Case insensitive search"
			},
			"type": {
				"type": "string",
				"description": "File type to search (e.g. js, py, rust, go)"
			},
			"head_limit": {
				"type": "number",
				"description": "Limit output to first N lines/entries"
			},
			"multiline": {
				"type": "boolean",
				"description": "Enable multiline mode where patterns can span lines"
			}
		},
		"required": ["pattern"]
	}`)
}

func (g *GrepTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[GrepInput](input.Params)
	if err != nil {
		return err
	}

	if params.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}

	return nil
}

func (g *GrepTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[GrepInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Build arguments
	args := []string{}

	// Output mode
	switch params.OutputMode {
	case "content":
		// Default shows content
		args = append(args, "-n") // Always show line numbers
	case "count":
		args = append(args, "-c")
	case "files_with_matches", "":
		args = append(args, "-l")
	}

	// Options
	if params.IgnoreCase {
		args = append(args, "-i")
	}

	if params.Glob != "" {
		args = append(args, "--glob", params.Glob)
	}

	if params.Type != "" {
		args = append(args, "--type", params.Type)
	}

	if params.Before > 0 {
		args = append(args, "-B", strconv.Itoa(params.Before))
	}

	if params.After > 0 {
		args = append(args, "-A", strconv.Itoa(params.After))
	}

	if params.Context > 0 {
		args = append(args, "-C", strconv.Itoa(params.Context))
	}

	if params.Multiline {
		args = append(args, "-U", "--multiline-dotall")
	}

	// Add pattern
	args = append(args, params.Pattern)

	// Add path
	searchPath := params.Path
	if searchPath == "" && input.Context != nil {
		searchPath = input.Context.CWD
	}
	if searchPath == "" {
		searchPath = "."
	}
	args = append(args, searchPath)

	// Execute ripgrep
	cmd := exec.CommandContext(ctx, g.RipgrepPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// ripgrep returns exit code 1 for no matches, which is not an error
	output := stdout.String()
	if output == "" && stderr.Len() > 0 {
		output = stderr.String()
	}

	if output == "" {
		output = "No matches found"
	}

	// Apply head_limit if specified
	if params.HeadLimit > 0 {
		lines := strings.Split(output, "\n")
		start := params.Offset
		if start >= len(lines) {
			output = "No matches found (offset exceeds results)"
		} else {
			end := start + params.HeadLimit
			if end > len(lines) {
				end = len(lines)
			}
			output = strings.Join(lines[start:end], "\n")
		}
	}

	// Count results
	numResults := 0
	if output != "No matches found" {
		numResults = len(strings.Split(strings.TrimSpace(output), "\n"))
	}

	return &tool.Output{
		Content: output,
		Metadata: map[string]interface{}{
			"numResults": numResults,
		},
	}, nil
}
