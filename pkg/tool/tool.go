// Package tool defines the tool interface and registry
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

// Tool is the interface that all tools must implement
type Tool interface {
	// Name returns the tool name (unique identifier)
	Name() string

	// Description returns the tool description for the AI
	Description() string

	// InputSchema returns the JSON Schema for the tool input
	InputSchema() json.RawMessage

	// Execute executes the tool with the given input
	Execute(ctx context.Context, input *Input) (*Output, error)

	// Validate validates the input (optional, can return nil)
	Validate(input *Input) error
}

// Input represents tool input
type Input struct {
	ID     string                 // tool_use_id
	Name   string                 // tool name
	Params map[string]interface{} // input parameters

	// Execution context
	Context *ExecutionContext
}

// ExecutionContext provides context for tool execution
type ExecutionContext struct {
	SessionID      string
	CWD            string
	ProjectPath    string
	PermissionMode PermissionMode

	// Callbacks
	RequestPermission func(req *PermissionRequest) (bool, error)
	Output            func(content string)
}

// PermissionMode represents the permission mode
type PermissionMode string

const (
	PermissionModeDefault           PermissionMode = "default"
	PermissionModePlan              PermissionMode = "plan"
	PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
	PermissionModeDontAsk           PermissionMode = "dontAsk"
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

// PermissionRequest represents a permission request
type PermissionRequest struct {
	Tool     string
	Action   string
	Resource string
	Message  string
}

// Output represents tool output
type Output struct {
	Content  string      `json:"content"`
	IsError  bool        `json:"is_error,omitempty"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// Registry is the tool registry
type Registry struct {
	tools    map[string]Tool
	aliases  map[string]string
	disabled map[string]bool

	mu sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]Tool),
		aliases:  make(map[string]string),
		disabled: make(map[string]bool),
	}
}

// Register registers a tool
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// RegisterAlias registers an alias for a tool
func (r *Registry) RegisterAlias(alias, toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[alias] = toolName
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check alias
	if actual, ok := r.aliases[name]; ok {
		name = actual
	}

	// Check if disabled
	if r.disabled[name] {
		return nil, fmt.Errorf("tool %s is disabled", name)
	}

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// Disable disables a tool
func (r *Registry) Disable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disabled[name] = true
}

// Enable enables a tool
func (r *Registry) Enable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.disabled, name)
}

// List returns all enabled tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for name, tool := range r.tools {
		if !r.disabled[name] {
			tools = append(tools, tool)
		}
	}
	return tools
}

// Names returns all tool names
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		if !r.disabled[name] {
			names = append(names, name)
		}
	}
	return names
}

// ToAPITools converts tools to API format
func (r *Registry) ToAPITools() []provider.Tool {
	tools := r.List()
	apiTools := make([]provider.Tool, len(tools))

	for i, t := range tools {
		apiTools[i] = provider.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		}
	}

	return apiTools
}

// FilteredRegistry returns a new registry with only the specified tools
func (r *Registry) FilteredRegistry(allowed []string, disallowed []string) *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := NewRegistry()

	allowSet := make(map[string]bool)
	for _, name := range allowed {
		allowSet[name] = true
	}

	disallowSet := make(map[string]bool)
	for _, name := range disallowed {
		disallowSet[name] = true
	}

	for name, tool := range r.tools {
		// Skip if in disallow list
		if disallowSet[name] {
			continue
		}

		// If allow list is specified, only include those
		if len(allowed) > 0 && !allowSet[name] {
			continue
		}

		filtered.tools[name] = tool
	}

	return filtered
}

// ParamsTo converts params map to a struct
func ParamsTo[T any](params map[string]interface{}) (*T, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
