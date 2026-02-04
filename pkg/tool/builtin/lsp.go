package builtin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// LSPTool provides language server protocol integration
type LSPTool struct {
	servers map[string]*LSPServer // language -> server
	mu      sync.RWMutex
}

// LSPServer represents a running LSP server
type LSPServer struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	reader   *bufio.Reader
	msgID    int
	pending  map[int]chan json.RawMessage
	mu       sync.Mutex
}

// LSPInput represents the input for LSP tool
type LSPInput struct {
	Operation string `json:"operation"` // goToDefinition, findReferences, hover, etc.
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`      // 1-based
	Character int    `json:"character"` // 1-based
}

// LSP message types
type lspRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type lspResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *lspError       `json:"error,omitempty"`
}

type lspError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type lspNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// LSP parameter types
type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

type location struct {
	URI   string `json:"uri"`
	Range lspRange `json:"range"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type hover struct {
	Contents interface{} `json:"contents"`
	Range    *lspRange   `json:"range,omitempty"`
}

type symbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// NewLSPTool creates a new LSP tool
func NewLSPTool() *LSPTool {
	return &LSPTool{
		servers: make(map[string]*LSPServer),
	}
}

func (l *LSPTool) Name() string {
	return "LSP"
}

func (l *LSPTool) Description() string {
	return `Interact with Language Server Protocol servers for code intelligence.

Supported operations:
- goToDefinition: Find where a symbol is defined
- findReferences: Find all references to a symbol
- hover: Get hover information (documentation, type info)
- documentSymbol: Get all symbols in a document
- workspaceSymbol: Search for symbols across the workspace
- goToImplementation: Find implementations of an interface
- prepareCallHierarchy: Get call hierarchy item at position
- incomingCalls: Find functions that call the function at position
- outgoingCalls: Find functions called by the function at position

All operations require filePath, line (1-based), and character (1-based).`
}

func (l *LSPTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"operation": {
				"type": "string",
				"description": "The LSP operation to perform",
				"enum": ["goToDefinition", "findReferences", "hover", "documentSymbol", "workspaceSymbol", "goToImplementation", "prepareCallHierarchy", "incomingCalls", "outgoingCalls"]
			},
			"filePath": {
				"type": "string",
				"description": "The absolute or relative path to the file"
			},
			"line": {
				"type": "integer",
				"description": "The line number (1-based)",
				"exclusiveMinimum": 0
			},
			"character": {
				"type": "integer",
				"description": "The character offset (1-based)",
				"exclusiveMinimum": 0
			}
		},
		"required": ["operation", "filePath", "line", "character"]
	}`)
}

func (l *LSPTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[LSPInput](input.Params)
	if err != nil {
		return err
	}

	if params.Operation == "" {
		return fmt.Errorf("operation is required")
	}

	validOps := map[string]bool{
		"goToDefinition":       true,
		"findReferences":       true,
		"hover":                true,
		"documentSymbol":       true,
		"workspaceSymbol":      true,
		"goToImplementation":   true,
		"prepareCallHierarchy": true,
		"incomingCalls":        true,
		"outgoingCalls":        true,
	}
	if !validOps[params.Operation] {
		return fmt.Errorf("invalid operation: %s", params.Operation)
	}

	if params.FilePath == "" {
		return fmt.Errorf("filePath is required")
	}

	if params.Line < 1 {
		return fmt.Errorf("line must be >= 1")
	}

	if params.Character < 1 {
		return fmt.Errorf("character must be >= 1")
	}

	return nil
}

func (l *LSPTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[LSPInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Make path absolute
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		if input.Context != nil && input.Context.CWD != "" {
			filePath = filepath.Join(input.Context.CWD, filePath)
		}
	}

	// Detect language from file extension
	lang := detectLanguage(filePath)
	if lang == "" {
		return &tool.Output{
			Content: fmt.Sprintf("Unsupported file type: %s", filepath.Ext(filePath)),
			IsError: true,
		}, nil
	}

	// Get or start LSP server
	server, err := l.getServer(lang)
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Failed to start LSP server for %s: %v", lang, err),
			IsError: true,
		}, nil
	}

	// Convert to 0-based indexing for LSP
	pos := position{
		Line:      params.Line - 1,
		Character: params.Character - 1,
	}

	// Execute operation
	var result interface{}
	switch params.Operation {
	case "goToDefinition":
		result, err = server.goToDefinition(ctx, filePath, pos)
	case "findReferences":
		result, err = server.findReferences(ctx, filePath, pos)
	case "hover":
		result, err = server.hover(ctx, filePath, pos)
	case "documentSymbol":
		result, err = server.documentSymbol(ctx, filePath)
	case "workspaceSymbol":
		result, err = server.workspaceSymbol(ctx, "")
	case "goToImplementation":
		result, err = server.goToImplementation(ctx, filePath, pos)
	default:
		return &tool.Output{
			Content: fmt.Sprintf("Operation not yet implemented: %s", params.Operation),
			IsError: true,
		}, nil
	}

	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("LSP error: %v", err),
			IsError: true,
		}, nil
	}

	// Format result
	content := formatLSPResult(params.Operation, result)

	return &tool.Output{
		Content: content,
		Metadata: map[string]interface{}{
			"operation": params.Operation,
			"language":  lang,
		},
	}, nil
}

// getServer returns or creates an LSP server for a language
func (l *LSPTool) getServer(lang string) (*LSPServer, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if server, ok := l.servers[lang]; ok {
		return server, nil
	}

	server, err := startLSPServer(lang)
	if err != nil {
		return nil, err
	}

	l.servers[lang] = server
	return server, nil
}

// startLSPServer starts an LSP server for a language
func startLSPServer(lang string) (*LSPServer, error) {
	var cmd *exec.Cmd

	switch lang {
	case "go":
		cmd = exec.Command("gopls")
	case "typescript", "javascript":
		cmd = exec.Command("typescript-language-server", "--stdio")
	case "python":
		cmd = exec.Command("pylsp")
	case "rust":
		cmd = exec.Command("rust-analyzer")
	case "c", "cpp":
		cmd = exec.Command("clangd")
	default:
		return nil, fmt.Errorf("no LSP server configured for %s", lang)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	server := &LSPServer{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		reader:  bufio.NewReader(stdout),
		pending: make(map[int]chan json.RawMessage),
	}

	// Start response reader
	go server.readResponses()

	// Initialize the server
	if err := server.initialize(); err != nil {
		server.Close()
		return nil, err
	}

	return server, nil
}

// sendRequest sends a request and waits for response
func (s *LSPServer) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	s.mu.Lock()
	s.msgID++
	id := s.msgID
	responseCh := make(chan json.RawMessage, 1)
	s.pending[id] = responseCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	req := lspRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := s.writeMessage(req); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-responseCh:
		return result, nil
	}
}

// sendNotification sends a notification (no response expected)
func (s *LSPServer) sendNotification(method string, params interface{}) error {
	notif := lspNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return s.writeMessage(notif)
}

// writeMessage writes a JSON-RPC message
func (s *LSPServer) writeMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	_, err = s.stdin.Write([]byte(header))
	if err != nil {
		return err
	}

	_, err = s.stdin.Write(data)
	return err
}

// readResponses reads responses from the server
func (s *LSPServer) readResponses() {
	for {
		// Read header
		var contentLength int
		for {
			line, err := s.reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Read content
		content := make([]byte, contentLength)
		_, err := io.ReadFull(s.reader, content)
		if err != nil {
			return
		}

		// Parse response
		var resp lspResponse
		if err := json.Unmarshal(content, &resp); err != nil {
			continue
		}

		// Deliver to waiting request
		s.mu.Lock()
		if ch, ok := s.pending[resp.ID]; ok {
			ch <- resp.Result
		}
		s.mu.Unlock()
	}
}

// initialize sends the initialize request
func (s *LSPServer) initialize() error {
	ctx := context.Background()

	cwd, _ := os.Getwd()
	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   "file://" + cwd,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition":     map[string]interface{}{"dynamicRegistration": true},
				"references":     map[string]interface{}{"dynamicRegistration": true},
				"hover":          map[string]interface{}{"dynamicRegistration": true},
				"documentSymbol": map[string]interface{}{"dynamicRegistration": true},
			},
			"workspace": map[string]interface{}{
				"symbol": map[string]interface{}{"dynamicRegistration": true},
			},
		},
	}

	_, err := s.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	// Send initialized notification
	return s.sendNotification("initialized", struct{}{})
}

// Close shuts down the LSP server
func (s *LSPServer) Close() error {
	s.sendNotification("shutdown", nil)
	s.sendNotification("exit", nil)
	return s.cmd.Wait()
}

// LSP operations
func (s *LSPServer) goToDefinition(ctx context.Context, filePath string, pos position) (interface{}, error) {
	params := textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file://" + filePath},
		Position:     pos,
	}

	result, err := s.sendRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	var locations []location
	if err := json.Unmarshal(result, &locations); err != nil {
		// Try single location
		var loc location
		if err := json.Unmarshal(result, &loc); err == nil {
			return []location{loc}, nil
		}
		return nil, err
	}
	return locations, nil
}

func (s *LSPServer) findReferences(ctx context.Context, filePath string, pos position) (interface{}, error) {
	params := map[string]interface{}{
		"textDocument": textDocumentIdentifier{URI: "file://" + filePath},
		"position":     pos,
		"context":      map[string]bool{"includeDeclaration": true},
	}

	result, err := s.sendRequest(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locations []location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, err
	}
	return locations, nil
}

func (s *LSPServer) hover(ctx context.Context, filePath string, pos position) (interface{}, error) {
	params := textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file://" + filePath},
		Position:     pos,
	}

	result, err := s.sendRequest(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	var h hover
	if err := json.Unmarshal(result, &h); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *LSPServer) documentSymbol(ctx context.Context, filePath string) (interface{}, error) {
	params := map[string]interface{}{
		"textDocument": textDocumentIdentifier{URI: "file://" + filePath},
	}

	result, err := s.sendRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []symbolInformation
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}

func (s *LSPServer) workspaceSymbol(ctx context.Context, query string) (interface{}, error) {
	params := map[string]interface{}{
		"query": query,
	}

	result, err := s.sendRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []symbolInformation
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}

func (s *LSPServer) goToImplementation(ctx context.Context, filePath string, pos position) (interface{}, error) {
	params := textDocumentPositionParams{
		TextDocument: textDocumentIdentifier{URI: "file://" + filePath},
		Position:     pos,
	}

	result, err := s.sendRequest(ctx, "textDocument/implementation", params)
	if err != nil {
		return nil, err
	}

	var locations []location
	if err := json.Unmarshal(result, &locations); err != nil {
		var loc location
		if err := json.Unmarshal(result, &loc); err == nil {
			return []location{loc}, nil
		}
		return nil, err
	}
	return locations, nil
}

// detectLanguage detects the programming language from file extension
func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	default:
		return ""
	}
}

// formatLSPResult formats the LSP result for display
func formatLSPResult(operation string, result interface{}) string {
	switch operation {
	case "goToDefinition", "findReferences", "goToImplementation":
		locations, ok := result.([]location)
		if !ok || len(locations) == 0 {
			return "No results found"
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d result(s):\n", len(locations)))
		for _, loc := range locations {
			path := strings.TrimPrefix(loc.URI, "file://")
			sb.WriteString(fmt.Sprintf("  %s:%d:%d\n",
				path,
				loc.Range.Start.Line+1,
				loc.Range.Start.Character+1))
		}
		return sb.String()

	case "hover":
		h, ok := result.(hover)
		if !ok {
			return "No hover information"
		}

		switch contents := h.Contents.(type) {
		case string:
			return contents
		case map[string]interface{}:
			if value, ok := contents["value"].(string); ok {
				return value
			}
		case []interface{}:
			var parts []string
			for _, part := range contents {
				if s, ok := part.(string); ok {
					parts = append(parts, s)
				} else if m, ok := part.(map[string]interface{}); ok {
					if v, ok := m["value"].(string); ok {
						parts = append(parts, v)
					}
				}
			}
			return strings.Join(parts, "\n\n")
		}
		return fmt.Sprintf("%v", h.Contents)

	case "documentSymbol", "workspaceSymbol":
		symbols, ok := result.([]symbolInformation)
		if !ok || len(symbols) == 0 {
			return "No symbols found"
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d symbol(s):\n", len(symbols)))
		for _, sym := range symbols {
			path := strings.TrimPrefix(sym.Location.URI, "file://")
			sb.WriteString(fmt.Sprintf("  %s (%s) - %s:%d\n",
				sym.Name,
				symbolKindName(sym.Kind),
				path,
				sym.Location.Range.Start.Line+1))
		}
		return sb.String()

	default:
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data)
	}
}

// symbolKindName returns the name of a symbol kind
func symbolKindName(kind int) string {
	names := map[int]string{
		1:  "File",
		2:  "Module",
		3:  "Namespace",
		4:  "Package",
		5:  "Class",
		6:  "Method",
		7:  "Property",
		8:  "Field",
		9:  "Constructor",
		10: "Enum",
		11: "Interface",
		12: "Function",
		13: "Variable",
		14: "Constant",
		15: "String",
		16: "Number",
		17: "Boolean",
		18: "Array",
		19: "Object",
		20: "Key",
		21: "Null",
		22: "EnumMember",
		23: "Struct",
		24: "Event",
		25: "Operator",
		26: "TypeParameter",
	}
	if name, ok := names[kind]; ok {
		return name
	}
	return fmt.Sprintf("Kind(%d)", kind)
}
