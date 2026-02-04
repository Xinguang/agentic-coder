// Package builtin provides built-in tools
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/mcp"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// MCPSearchTool searches for and selects MCP tools
type MCPSearchTool struct {
	manager       *mcp.Manager
	availableTools []mcp.MCPTool
}

// NewMCPSearchTool creates a new MCP search tool
func NewMCPSearchTool(manager *mcp.Manager) *MCPSearchTool {
	return &MCPSearchTool{
		manager: manager,
	}
}

// SetAvailableTools sets the list of available MCP tools for search
func (t *MCPSearchTool) SetAvailableTools(tools []mcp.MCPTool) {
	t.availableTools = tools
}

func (t *MCPSearchTool) Name() string {
	return "MCPSearch"
}

func (t *MCPSearchTool) Description() string {
	return `Search for or select MCP tools to make them available for use.

MANDATORY: You MUST use this tool to load MCP tools BEFORE calling them.

Query modes:
1. Direct selection - Use "select:<tool_name>" when you know which tool:
   - "select:mcp__slack__read_channel"
   - "select:mcp__filesystem__list_directory"

2. Keyword search - Use keywords when unsure:
   - "list directory" - find tools for listing directories
   - "slack message" - find slack messaging tools

Returns up to 5 matching tools ranked by relevance.`
}

func (t *MCPSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Query to find MCP tools. Use 'select:<tool_name>' for direct selection, or keywords to search."
			},
			"max_results": {
				"type": "number",
				"description": "Maximum number of results to return (default: 5)",
				"default": 5
			}
		},
		"required": ["query"]
	}`)
}

type mcpSearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func (t *MCPSearchTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[mcpSearchParams](input.Params)
	if err != nil {
		return err
	}

	if params.Query == "" {
		return fmt.Errorf("query is required")
	}

	return nil
}

func (t *MCPSearchTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[mcpSearchParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	// Handle direct selection
	if strings.HasPrefix(params.Query, "select:") {
		toolName := strings.TrimPrefix(params.Query, "select:")
		return t.selectTool(ctx, toolName)
	}

	// Keyword search
	results := t.searchTools(params.Query, maxResults)
	if len(results) == 0 {
		return &tool.Output{
			Content: fmt.Sprintf("No MCP tools found matching '%s'", params.Query),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d MCP tools matching '%s':\n\n", len(results), params.Query))
	for i, tool := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, tool.Name))
		if tool.Description != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", tool.Description))
		}
		sb.WriteString("\n")
	}

	return &tool.Output{
		Content: sb.String(),
		Metadata: map[string]interface{}{
			"query":        params.Query,
			"result_count": len(results),
			"tools":        results,
		},
	}, nil
}

func (t *MCPSearchTool) selectTool(ctx context.Context, toolName string) (*tool.Output, error) {
	// Find tool in available tools
	for _, mcpTool := range t.availableTools {
		if mcpTool.Name == toolName {
			return &tool.Output{
				Content: fmt.Sprintf("Selected MCP tool: %s\n\nDescription: %s\n\nThe tool is now available for use.", toolName, mcpTool.Description),
				Metadata: map[string]interface{}{
					"selected_tool": toolName,
					"tool_schema":   mcpTool.InputSchema,
				},
			}, nil
		}
	}

	return &tool.Output{
		Content: fmt.Sprintf("MCP tool '%s' not found", toolName),
		IsError: true,
	}, nil
}

func (t *MCPSearchTool) searchTools(query string, maxResults int) []mcp.MCPTool {
	query = strings.ToLower(query)
	keywords := strings.Fields(query)

	type scoredTool struct {
		tool  mcp.MCPTool
		score int
	}

	var scored []scoredTool

	for _, mcpTool := range t.availableTools {
		score := 0
		nameLower := strings.ToLower(mcpTool.Name)
		descLower := strings.ToLower(mcpTool.Description)

		for _, kw := range keywords {
			if strings.Contains(nameLower, kw) {
				score += 10
			}
			if strings.Contains(descLower, kw) {
				score += 5
			}
		}

		if score > 0 {
			scored = append(scored, scoredTool{tool: mcpTool, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Limit results
	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}

	results := make([]mcp.MCPTool, len(scored))
	for i, s := range scored {
		results[i] = s.tool
	}
	return results
}

// ListMCPResourcesTool lists available MCP resources
type ListMCPResourcesTool struct {
	manager *mcp.Manager
}

// NewListMCPResourcesTool creates a new list MCP resources tool
func NewListMCPResourcesTool(manager *mcp.Manager) *ListMCPResourcesTool {
	return &ListMCPResourcesTool{
		manager: manager,
	}
}

func (t *ListMCPResourcesTool) Name() string {
	return "ListMcpResourcesTool"
}

func (t *ListMCPResourcesTool) Description() string {
	return `List available resources from configured MCP servers.

Each returned resource includes:
- URI: Resource identifier
- Name: Human-readable name
- Description: What the resource contains
- Server: Which MCP server provides it

Parameters:
- server (optional): Filter resources by server name`
}

func (t *ListMCPResourcesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {
				"type": "string",
				"description": "Optional server name to filter resources by"
			}
		}
	}`)
}

type listMCPResourcesParams struct {
	Server string `json:"server"`
}

func (t *ListMCPResourcesTool) Validate(input *tool.Input) error {
	return nil
}

func (t *ListMCPResourcesTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[listMCPResourcesParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	if t.manager == nil {
		return &tool.Output{Content: "MCP manager not configured", IsError: true}, nil
	}

	resources, err := t.manager.ListResources(ctx, params.Server)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error listing resources: %v", err), IsError: true}, nil
	}

	if len(resources) == 0 {
		msg := "No MCP resources available"
		if params.Server != "" {
			msg = fmt.Sprintf("No resources found for server '%s'", params.Server)
		}
		return &tool.Output{Content: msg}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d MCP resources:\n\n", len(resources)))
	for i, res := range resources {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, res.Name))
		sb.WriteString(fmt.Sprintf("   URI: %s\n", res.URI))
		if res.Description != "" {
			sb.WriteString(fmt.Sprintf("   Description: %s\n", res.Description))
		}
		sb.WriteString(fmt.Sprintf("   Server: %s\n", res.Server))
		sb.WriteString("\n")
	}

	return &tool.Output{
		Content: sb.String(),
		Metadata: map[string]interface{}{
			"resource_count": len(resources),
			"resources":      resources,
		},
	}, nil
}

// ReadMCPResourceTool reads a specific MCP resource
type ReadMCPResourceTool struct {
	manager *mcp.Manager
}

// NewReadMCPResourceTool creates a new read MCP resource tool
func NewReadMCPResourceTool(manager *mcp.Manager) *ReadMCPResourceTool {
	return &ReadMCPResourceTool{
		manager: manager,
	}
}

func (t *ReadMCPResourceTool) Name() string {
	return "ReadMcpResourceTool"
}

func (t *ReadMCPResourceTool) Description() string {
	return `Read a specific resource from an MCP server.

Parameters:
- server (required): The name of the MCP server
- uri (required): The URI of the resource to read`
}

func (t *ReadMCPResourceTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {
				"type": "string",
				"description": "The MCP server name"
			},
			"uri": {
				"type": "string",
				"description": "The resource URI to read"
			}
		},
		"required": ["server", "uri"]
	}`)
}

type readMCPResourceParams struct {
	Server string `json:"server"`
	URI    string `json:"uri"`
}

func (t *ReadMCPResourceTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[readMCPResourceParams](input.Params)
	if err != nil {
		return err
	}

	if params.Server == "" {
		return fmt.Errorf("server is required")
	}
	if params.URI == "" {
		return fmt.Errorf("uri is required")
	}

	return nil
}

func (t *ReadMCPResourceTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[readMCPResourceParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	if t.manager == nil {
		return &tool.Output{Content: "MCP manager not configured", IsError: true}, nil
	}

	content, err := t.manager.ReadResource(ctx, params.Server, params.URI)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error reading resource: %v", err), IsError: true}, nil
	}

	return &tool.Output{
		Content: content,
		Metadata: map[string]interface{}{
			"server": params.Server,
			"uri":    params.URI,
		},
	}, nil
}
