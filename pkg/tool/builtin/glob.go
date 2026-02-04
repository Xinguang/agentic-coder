package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// GlobTool implements file pattern matching
type GlobTool struct{}

// GlobInput represents the input for Glob tool
type GlobInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// NewGlobTool creates a new Glob tool
func NewGlobTool() *GlobTool {
	return &GlobTool{}
}

func (g *GlobTool) Name() string {
	return "Glob"
}

func (g *GlobTool) Description() string {
	return `Fast file pattern matching tool that works with any codebase size.
- Supports glob patterns like "**/*.js" or "src/**/*.ts"
- Returns matching file paths sorted by modification time
- Use this tool when you need to find files by name patterns`
}

func (g *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "The glob pattern to match files against"
			},
			"path": {
				"type": "string",
				"description": "The directory to search in. If not specified, the current working directory will be used."
			}
		},
		"required": ["pattern"]
	}`)
}

func (g *GlobTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[GlobInput](input.Params)
	if err != nil {
		return err
	}

	if params.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}

	return nil
}

func (g *GlobTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[GlobInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Determine base path
	basePath := params.Path
	if basePath == "" && input.Context != nil {
		basePath = input.Context.CWD
	}
	if basePath == "" {
		basePath = "."
	}

	// Make absolute
	if !filepath.IsAbs(basePath) {
		if input.Context != nil && input.Context.CWD != "" {
			basePath = filepath.Join(input.Context.CWD, basePath)
		}
	}

	// Construct full pattern
	fullPattern := filepath.Join(basePath, params.Pattern)

	// Find matches
	matches, err := doublestar.FilepathGlob(fullPattern)
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Error matching pattern: %v", err),
			IsError: true,
		}, nil
	}

	if len(matches) == 0 {
		return &tool.Output{
			Content: "No files found",
		}, nil
	}

	// Get file info for sorting
	type fileWithTime struct {
		path    string
		modTime int64
	}

	filesWithTime := make([]fileWithTime, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		filesWithTime = append(filesWithTime, fileWithTime{
			path:    match,
			modTime: info.ModTime().Unix(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(filesWithTime, func(i, j int) bool {
		return filesWithTime[i].modTime > filesWithTime[j].modTime
	})

	// Build result
	var result strings.Builder
	for _, f := range filesWithTime {
		result.WriteString(f.path)
		result.WriteString("\n")
	}

	return &tool.Output{
		Content: strings.TrimSuffix(result.String(), "\n"),
		Metadata: map[string]interface{}{
			"numFiles": len(filesWithTime),
		},
	}, nil
}
