// Package builtin provides built-in tools
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// NotebookEditTool edits Jupyter notebook cells
type NotebookEditTool struct{}

// NewNotebookEditTool creates a new notebook edit tool
func NewNotebookEditTool() *NotebookEditTool {
	return &NotebookEditTool{}
}

func (t *NotebookEditTool) Name() string {
	return "NotebookEdit"
}

func (t *NotebookEditTool) Description() string {
	return `Edit Jupyter notebook (.ipynb) cells.

Supports three edit modes:
- replace: Replace the contents of a specific cell
- insert: Insert a new cell at a position
- delete: Delete a cell

Parameters:
- notebook_path: Absolute path to the notebook file
- cell_id: The ID of the cell to edit (optional, can use index)
- cell_type: Type of cell (code or markdown) - required for insert
- new_source: The new content for the cell
- edit_mode: replace, insert, or delete (default: replace)`
}

func (t *NotebookEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"notebook_path": {
				"type": "string",
				"description": "Absolute path to the Jupyter notebook file"
			},
			"cell_id": {
				"type": "string",
				"description": "The ID of the cell to edit. For insert, the new cell will be inserted after this cell."
			},
			"cell_type": {
				"type": "string",
				"enum": ["code", "markdown"],
				"description": "The type of the cell (code or markdown). Required for insert mode."
			},
			"new_source": {
				"type": "string",
				"description": "The new source content for the cell"
			},
			"edit_mode": {
				"type": "string",
				"enum": ["replace", "insert", "delete"],
				"description": "The type of edit to make (default: replace)"
			}
		},
		"required": ["notebook_path", "new_source"]
	}`)
}

type notebookEditParams struct {
	NotebookPath string `json:"notebook_path"`
	CellID       string `json:"cell_id"`
	CellType     string `json:"cell_type"`
	NewSource    string `json:"new_source"`
	EditMode     string `json:"edit_mode"`
}

func (t *NotebookEditTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[notebookEditParams](input.Params)
	if err != nil {
		return err
	}

	if params.NotebookPath == "" {
		return fmt.Errorf("notebook_path is required")
	}

	if !filepath.IsAbs(params.NotebookPath) {
		return fmt.Errorf("notebook_path must be an absolute path")
	}

	if !strings.HasSuffix(params.NotebookPath, ".ipynb") {
		return fmt.Errorf("file must be a Jupyter notebook (.ipynb)")
	}

	editMode := params.EditMode
	if editMode == "" {
		editMode = "replace"
	}

	if editMode == "insert" && params.CellType == "" {
		return fmt.Errorf("cell_type is required for insert mode")
	}

	if editMode == "insert" && params.CellType != "code" && params.CellType != "markdown" {
		return fmt.Errorf("cell_type must be 'code' or 'markdown'")
	}

	return nil
}

func (t *NotebookEditTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[notebookEditParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	editMode := params.EditMode
	if editMode == "" {
		editMode = "replace"
	}

	// Read notebook
	notebook, err := t.readNotebook(params.NotebookPath)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error reading notebook: %v", err), IsError: true}, nil
	}

	// Find cell index
	cellIndex := -1
	if params.CellID != "" {
		cellIndex = t.findCellByID(notebook, params.CellID)
		if cellIndex == -1 && editMode != "insert" {
			return &tool.Output{Content: fmt.Sprintf("Cell not found: %s", params.CellID), IsError: true}, nil
		}
	}

	// Perform edit
	switch editMode {
	case "replace":
		if cellIndex == -1 {
			// If no cell ID, replace first cell
			cellIndex = 0
		}
		if cellIndex >= len(notebook.Cells) {
			return &tool.Output{Content: "Cell index out of range", IsError: true}, nil
		}
		notebook.Cells[cellIndex].Source = t.splitSource(params.NewSource)
		if params.CellType != "" {
			notebook.Cells[cellIndex].CellType = params.CellType
		}

	case "insert":
		newCell := &NotebookCell{
			CellType: params.CellType,
			Source:   t.splitSource(params.NewSource),
			Metadata: map[string]interface{}{},
		}
		if params.CellType == "code" {
			newCell.Outputs = []interface{}{}
			newCell.ExecutionCount = nil
		}

		insertIndex := cellIndex + 1
		if cellIndex == -1 {
			insertIndex = 0 // Insert at beginning
		}

		// Insert cell
		notebook.Cells = append(notebook.Cells[:insertIndex], append([]*NotebookCell{newCell}, notebook.Cells[insertIndex:]...)...)

	case "delete":
		if cellIndex == -1 {
			return &tool.Output{Content: "cell_id is required for delete mode", IsError: true}, nil
		}
		notebook.Cells = append(notebook.Cells[:cellIndex], notebook.Cells[cellIndex+1:]...)

	default:
		return &tool.Output{Content: fmt.Sprintf("Invalid edit_mode: %s", editMode), IsError: true}, nil
	}

	// Write notebook back
	if err := t.writeNotebook(params.NotebookPath, notebook); err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error writing notebook: %v", err), IsError: true}, nil
	}

	// Build result message
	var msg string
	switch editMode {
	case "replace":
		msg = fmt.Sprintf("Replaced cell %d in %s", cellIndex, filepath.Base(params.NotebookPath))
	case "insert":
		msg = fmt.Sprintf("Inserted new %s cell in %s", params.CellType, filepath.Base(params.NotebookPath))
	case "delete":
		msg = fmt.Sprintf("Deleted cell %d from %s", cellIndex, filepath.Base(params.NotebookPath))
	}

	return &tool.Output{
		Content: msg,
		Metadata: map[string]interface{}{
			"notebook_path": params.NotebookPath,
			"edit_mode":     editMode,
			"cell_index":    cellIndex,
			"cell_count":    len(notebook.Cells),
		},
	}, nil
}

// Notebook represents a Jupyter notebook
type Notebook struct {
	Cells         []*NotebookCell        `json:"cells"`
	Metadata      map[string]interface{} `json:"metadata"`
	NBFormat      int                    `json:"nbformat"`
	NBFormatMinor int                    `json:"nbformat_minor"`
}

// NotebookCell represents a cell in a Jupyter notebook
type NotebookCell struct {
	CellType       string                 `json:"cell_type"`
	Source         []string               `json:"source"`
	Metadata       map[string]interface{} `json:"metadata"`
	ID             string                 `json:"id,omitempty"`
	Outputs        []interface{}          `json:"outputs,omitempty"`
	ExecutionCount *int                   `json:"execution_count,omitempty"`
}

// readNotebook reads a Jupyter notebook from disk
func (t *NotebookEditTool) readNotebook(path string) (*Notebook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var notebook Notebook
	if err := json.Unmarshal(data, &notebook); err != nil {
		return nil, err
	}

	return &notebook, nil
}

// writeNotebook writes a Jupyter notebook to disk
func (t *NotebookEditTool) writeNotebook(path string, notebook *Notebook) error {
	data, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// findCellByID finds a cell by its ID
func (t *NotebookEditTool) findCellByID(notebook *Notebook, cellID string) int {
	for i, cell := range notebook.Cells {
		if cell.ID == cellID {
			return i
		}
	}
	return -1
}

// splitSource splits source into lines for notebook format
func (t *NotebookEditTool) splitSource(source string) []string {
	if source == "" {
		return []string{}
	}

	lines := strings.Split(source, "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		if i < len(lines)-1 {
			result[i] = line + "\n"
		} else {
			result[i] = line
		}
	}

	return result
}
