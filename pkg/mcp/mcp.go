// Package mcp provides Model Context Protocol (MCP) integration
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ServerType represents the type of MCP server
type ServerType string

const (
	ServerTypeStdio ServerType = "stdio"
	ServerTypeSSE   ServerType = "sse"
	ServerTypeHTTP  ServerType = "http"
)

// ServerConfig represents MCP server configuration
type ServerConfig struct {
	Name      string            `json:"name"`
	Type      ServerType        `json:"type"`
	Command   string            `json:"command,omitempty"`   // For stdio
	Args      []string          `json:"args,omitempty"`      // For stdio
	URL       string            `json:"url,omitempty"`       // For sse/http
	Env       map[string]string `json:"env,omitempty"`       // Environment variables
	AutoStart bool              `json:"auto_start,omitempty"`
}

// Server represents an MCP server connection
type Server struct {
	config   ServerConfig
	client   *Client
	tools    []Tool
	running  bool
	mu       sync.RWMutex

	// For stdio servers
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// Tool represents an MCP tool
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	ServerName  string          `json:"server_name"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	Server      string `json:"server"`
}

// MCPTool is an alias for Tool for external use
type MCPTool = Tool

// Client handles MCP protocol communication
type Client struct {
	server *Server
	reqID  int
	mu     sync.Mutex

	// Response channels
	responses map[int]chan *Response
	respMu    sync.Mutex
}

// Request represents an MCP JSON-RPC request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// Manager manages multiple MCP server connections
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*Server
	tools   map[string]*Tool
}

// NewManager creates a new MCP manager
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*Server),
		tools:   make(map[string]*Tool),
	}
}

// AddServer adds and starts an MCP server
func (m *Manager) AddServer(config ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[config.Name]; exists {
		return fmt.Errorf("server already exists: %s", config.Name)
	}

	server := &Server{
		config: config,
	}

	m.servers[config.Name] = server

	if config.AutoStart {
		if err := m.startServer(server); err != nil {
			delete(m.servers, config.Name)
			return err
		}
	}

	return nil
}

// RemoveServer stops and removes an MCP server
func (m *Manager) RemoveServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, exists := m.servers[name]
	if !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	if server.running {
		if err := m.stopServer(server); err != nil {
			return err
		}
	}

	// Remove tools from this server
	for toolName, tool := range m.tools {
		if tool.ServerName == name {
			delete(m.tools, toolName)
		}
	}

	delete(m.servers, name)
	return nil
}

// StartServer starts an MCP server
func (m *Manager) StartServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, exists := m.servers[name]
	if !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	return m.startServer(server)
}

// startServer starts an MCP server (internal, must hold lock)
func (m *Manager) startServer(server *Server) error {
	server.mu.Lock()
	defer server.mu.Unlock()

	if server.running {
		return nil
	}

	switch server.config.Type {
	case ServerTypeStdio:
		return m.startStdioServer(server)
	case ServerTypeSSE:
		return m.startSSEServer(server)
	case ServerTypeHTTP:
		return m.startHTTPServer(server)
	default:
		return fmt.Errorf("unsupported server type: %s", server.config.Type)
	}
}

// startStdioServer starts a stdio MCP server
func (m *Manager) startStdioServer(server *Server) error {
	cmd := exec.Command(server.config.Command, server.config.Args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range server.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	server.cmd = cmd
	server.stdin = stdin
	server.stdout = stdout
	server.running = true

	// Create client
	server.client = &Client{
		server:    server,
		responses: make(map[int]chan *Response),
	}

	// Start reading responses
	go server.client.readResponses()

	// Initialize connection
	if err := m.initializeServer(server); err != nil {
		m.stopServer(server)
		return err
	}

	// Discover tools
	if err := m.discoverTools(server); err != nil {
		m.stopServer(server)
		return err
	}

	return nil
}

// startSSEServer starts an SSE MCP server
func (m *Manager) startSSEServer(server *Server) error {
	server.client = &Client{
		server:    server,
		responses: make(map[int]chan *Response),
	}
	server.running = true

	// Initialize and discover tools
	if err := m.initializeServer(server); err != nil {
		return err
	}

	return m.discoverTools(server)
}

// startHTTPServer starts an HTTP MCP server
func (m *Manager) startHTTPServer(server *Server) error {
	server.client = &Client{
		server:    server,
		responses: make(map[int]chan *Response),
	}
	server.running = true

	// Initialize and discover tools
	if err := m.initializeServer(server); err != nil {
		return err
	}

	return m.discoverTools(server)
}

// stopServer stops an MCP server
func (m *Manager) stopServer(server *Server) error {
	server.mu.Lock()
	defer server.mu.Unlock()

	if !server.running {
		return nil
	}

	if server.stdin != nil {
		server.stdin.Close()
	}

	if server.cmd != nil {
		server.cmd.Process.Kill()
		server.cmd.Wait()
	}

	server.running = false
	return nil
}

// initializeServer initializes an MCP server connection
func (m *Manager) initializeServer(server *Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := server.client.Call(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "agentic-coder",
			"version": "0.1.0",
		},
	})

	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Send initialized notification
	server.client.Notify("notifications/initialized", nil)

	return nil
}

// discoverTools discovers tools from an MCP server
func (m *Manager) discoverTools(server *Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := server.client.Call(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	var toolsResult struct {
		Tools []Tool `json:"tools"`
	}

	if err := json.Unmarshal(result, &toolsResult); err != nil {
		return fmt.Errorf("failed to parse tools: %w", err)
	}

	// Register tools
	for _, tool := range toolsResult.Tools {
		tool.ServerName = server.config.Name
		fullName := fmt.Sprintf("mcp__%s__%s", server.config.Name, tool.Name)
		m.tools[fullName] = &tool
		server.tools = append(server.tools, tool)
	}

	return nil
}

// GetTools returns all available MCP tools
func (m *Manager) GetTools() []*Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]*Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetTool retrieves a tool by name
func (m *Manager) GetTool(name string) (*Tool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if tool, ok := m.tools[name]; ok {
		return tool, nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

// CallTool calls an MCP tool
func (m *Manager) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error) {
	m.mu.RLock()
	tool, ok := m.tools[toolName]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	server, ok := m.servers[tool.ServerName]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("server not found: %s", tool.ServerName)
	}
	m.mu.RUnlock()

	result, err := server.client.Call(ctx, "tools/call", map[string]interface{}{
		"name":      tool.Name,
		"arguments": arguments,
	})

	if err != nil {
		return nil, err
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}

	if err := json.Unmarshal(result, &callResult); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	// Extract text content
	var text string
	for _, content := range callResult.Content {
		if content.Type == "text" {
			text += content.Text
		}
	}

	return map[string]interface{}{
		"content": text,
		"isError": callResult.IsError,
	}, nil
}

// ListResources lists resources from all servers or a specific server
func (m *Manager) ListResources(ctx context.Context, serverName string) ([]*Resource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var resources []*Resource

	for name, server := range m.servers {
		// Filter by server name if specified
		if serverName != "" && name != serverName {
			continue
		}

		if !server.running {
			continue
		}

		result, err := server.client.Call(ctx, "resources/list", nil)
		if err != nil {
			continue
		}

		var resourcesResult struct {
			Resources []Resource `json:"resources"`
		}

		if err := json.Unmarshal(result, &resourcesResult); err != nil {
			continue
		}

		for _, res := range resourcesResult.Resources {
			res.Server = server.config.Name
			resources = append(resources, &res)
		}
	}

	return resources, nil
}

// ReadResource reads a resource from a server
func (m *Manager) ReadResource(ctx context.Context, serverName, uri string) (string, error) {
	m.mu.RLock()
	server, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("server not found: %s", serverName)
	}

	result, err := server.client.Call(ctx, "resources/read", map[string]interface{}{
		"uri": uri,
	})

	if err != nil {
		return "", err
	}

	var readResult struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType,omitempty"`
			Text     string `json:"text,omitempty"`
		} `json:"contents"`
	}

	if err := json.Unmarshal(result, &readResult); err != nil {
		return "", fmt.Errorf("failed to parse result: %w", err)
	}

	if len(readResult.Contents) > 0 {
		return readResult.Contents[0].Text, nil
	}

	return "", nil
}

// Client methods

// Call makes a JSON-RPC call
func (c *Client) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.reqID++
	id := c.reqID
	c.mu.Unlock()

	req := &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	respChan := make(chan *Response, 1)
	c.respMu.Lock()
	c.responses[id] = respChan
	c.respMu.Unlock()

	defer func() {
		c.respMu.Lock()
		delete(c.responses, id)
		c.respMu.Unlock()
	}()

	// Send request
	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// Notify sends a notification (no response expected)
func (c *Client) Notify(method string, params interface{}) error {
	req := &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.sendRequest(req)
}

// sendRequest sends a request to the server
func (c *Client) sendRequest(req *Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	switch c.server.config.Type {
	case ServerTypeStdio:
		_, err = fmt.Fprintf(c.server.stdin, "%s\n", data)
		return err
	case ServerTypeSSE, ServerTypeHTTP:
		return c.sendHTTPRequest(data)
	default:
		return fmt.Errorf("unsupported server type")
	}
}

// sendHTTPRequest sends a request via HTTP
func (c *Client) sendHTTPRequest(data []byte) error {
	resp, err := http.Post(c.server.config.URL, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	return nil
}

// readResponses reads responses from stdio
func (c *Client) readResponses() {
	scanner := bufio.NewScanner(c.server.stdout)
	for scanner.Scan() {
		line := scanner.Text()

		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}

		c.respMu.Lock()
		if ch, ok := c.responses[resp.ID]; ok {
			ch <- &resp
		}
		c.respMu.Unlock()
	}
}

// LoadConfigFromFile loads MCP configuration from .mcp.json
func LoadConfigFromFile(path string) ([]ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	var servers []ServerConfig
	for name, srv := range config.MCPServers {
		servers = append(servers, ServerConfig{
			Name:      name,
			Type:      ServerTypeStdio,
			Command:   srv.Command,
			Args:      srv.Args,
			Env:       srv.Env,
			AutoStart: true,
		})
	}

	return servers, nil
}
