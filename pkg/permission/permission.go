// Package permission provides permission management for tool execution
package permission

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Mode represents the permission mode
type Mode string

const (
	ModeDefault           Mode = "default"            // Ask for each tool use
	ModePlan              Mode = "plan"               // Planning mode, read-only
	ModeAcceptEdits       Mode = "accept_edits"       // Auto-accept file edits
	ModeDontAsk           Mode = "dont_ask"           // Don't ask, just execute
	ModeBypassPermissions Mode = "bypass_permissions" // Bypass all permissions
)

// Decision represents a permission decision
type Decision string

const (
	DecisionAllow      Decision = "allow"
	DecisionDeny       Decision = "deny"
	DecisionAsk        Decision = "ask"
	DecisionAllowOnce  Decision = "allow_once"
	DecisionAllowAll   Decision = "allow_all"
)

// Rule represents a permission rule
type Rule struct {
	Tool     string   `json:"tool"`               // Tool name pattern (supports wildcards)
	Action   Decision `json:"action"`             // allow, deny, ask
	Paths    []string `json:"paths,omitempty"`    // Path patterns (for file tools)
	Commands []string `json:"commands,omitempty"` // Command patterns (for bash)
	Scope    string   `json:"scope,omitempty"`    // global, project, session
}

// Request represents a permission request
type Request struct {
	Tool    string                 // Tool name
	Params  map[string]interface{} // Tool parameters
	Context *Context               // Execution context
}

// Context provides context for permission decisions
type Context struct {
	SessionID   string
	ProjectPath string
	CWD         string
	Mode        Mode
}

// Result represents a permission check result
type Result struct {
	Allowed bool
	Reason  string
	Rule    *Rule // The rule that matched, if any
}

// Manager manages permissions for tool execution
type Manager struct {
	mu sync.RWMutex

	mode  Mode
	rules []Rule

	// Cached decisions for the session
	sessionDecisions map[string]Decision

	// Allowed tools list
	allowedTools    map[string]bool
	disallowedTools map[string]bool

	// Path-based permissions
	allowedPaths    []string
	disallowedPaths []string

	// Callback for asking user
	askCallback func(req *Request) Decision
}

// NewManager creates a new permission manager
func NewManager(mode Mode) *Manager {
	return &Manager{
		mode:             mode,
		rules:            make([]Rule, 0),
		sessionDecisions: make(map[string]Decision),
		allowedTools:     make(map[string]bool),
		disallowedTools:  make(map[string]bool),
		allowedPaths:     make([]string, 0),
		disallowedPaths:  make([]string, 0),
	}
}

// SetMode sets the permission mode
func (m *Manager) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

// GetMode returns the current permission mode
func (m *Manager) GetMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

// SetAskCallback sets the callback function for asking user
func (m *Manager) SetAskCallback(cb func(req *Request) Decision) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.askCallback = cb
}

// AddRule adds a permission rule
func (m *Manager) AddRule(rule Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// RemoveRule removes a permission rule by tool pattern
func (m *Manager) RemoveRule(toolPattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newRules := make([]Rule, 0, len(m.rules))
	for _, r := range m.rules {
		if r.Tool != toolPattern {
			newRules = append(newRules, r)
		}
	}
	m.rules = newRules
}

// AllowTool adds a tool to the allowed list
func (m *Manager) AllowTool(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowedTools[toolName] = true
	delete(m.disallowedTools, toolName)
}

// DisallowTool adds a tool to the disallowed list
func (m *Manager) DisallowTool(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disallowedTools[toolName] = true
	delete(m.allowedTools, toolName)
}

// AllowPath adds a path pattern to the allowed list
func (m *Manager) AllowPath(pathPattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowedPaths = append(m.allowedPaths, pathPattern)
}

// DisallowPath adds a path pattern to the disallowed list
func (m *Manager) DisallowPath(pathPattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disallowedPaths = append(m.disallowedPaths, pathPattern)
}

// Check checks if a tool operation is permitted
func (m *Manager) Check(req *Request) *Result {
	m.mu.RLock()

	// Bypass mode allows everything
	if m.mode == ModeBypassPermissions {
		m.mu.RUnlock()
		return &Result{Allowed: true, Reason: "bypass mode"}
	}

	// Check disallowed tools first
	if m.disallowedTools[req.Tool] {
		m.mu.RUnlock()
		return &Result{Allowed: false, Reason: fmt.Sprintf("tool '%s' is disallowed", req.Tool)}
	}

	// Check explicit rules
	for _, rule := range m.rules {
		if m.matchRule(&rule, req) {
			switch rule.Action {
			case DecisionAllow:
				m.mu.RUnlock()
				return &Result{Allowed: true, Reason: "matched allow rule", Rule: &rule}
			case DecisionDeny:
				m.mu.RUnlock()
				return &Result{Allowed: false, Reason: "matched deny rule", Rule: &rule}
			case DecisionAsk:
				ruleCopy := rule // Copy rule before releasing lock
				m.mu.RUnlock()
				return m.handleAsk(req, &ruleCopy)
			}
		}
	}

	// Check allowed tools list
	if len(m.allowedTools) > 0 {
		if m.allowedTools[req.Tool] || m.matchToolPattern(req.Tool) {
			m.mu.RUnlock()
			return &Result{Allowed: true, Reason: "tool in allowed list"}
		}
	}

	// Mode-based decisions
	mode := m.mode
	m.mu.RUnlock()

	switch mode {
	case ModeDontAsk:
		return &Result{Allowed: true, Reason: "dont_ask mode"}

	case ModePlan:
		// In plan mode, only allow read-only operations
		if m.isReadOnlyTool(req.Tool) {
			return &Result{Allowed: true, Reason: "read-only tool in plan mode"}
		}
		return &Result{Allowed: false, Reason: "write operation not allowed in plan mode"}

	case ModeAcceptEdits:
		// Auto-accept file edit operations
		if m.isEditTool(req.Tool) {
			return &Result{Allowed: true, Reason: "auto-accept edits mode"}
		}
		return m.handleAsk(req, nil)

	default: // ModeDefault
		return m.handleAsk(req, nil)
	}
}

// matchRule checks if a rule matches the request
func (m *Manager) matchRule(rule *Rule, req *Request) bool {
	// Check tool pattern
	if !m.matchPattern(rule.Tool, req.Tool) {
		return false
	}

	// Check path patterns for file tools
	if len(rule.Paths) > 0 && m.isFileTool(req.Tool) {
		path := m.extractPath(req)
		if path == "" {
			return false
		}
		matched := false
		for _, pattern := range rule.Paths {
			if m.matchPath(pattern, path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check command patterns for bash
	if len(rule.Commands) > 0 && req.Tool == "Bash" {
		cmd := m.extractCommand(req)
		if cmd == "" {
			return false
		}
		matched := false
		for _, pattern := range rule.Commands {
			if m.matchPattern(pattern, cmd) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// matchPattern matches a pattern against a string (supports wildcards)
func (m *Manager) matchPattern(pattern, str string) bool {
	if pattern == "*" {
		return true
	}

	// Convert glob pattern to regex
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
	regexPattern = strings.ReplaceAll(regexPattern, `\?`, ".")

	matched, _ := regexp.MatchString(regexPattern, str)
	return matched
}

// matchPath matches a path against a pattern
func (m *Manager) matchPath(pattern, path string) bool {
	// Normalize paths
	pattern = filepath.Clean(pattern)
	path = filepath.Clean(path)

	// Direct match
	if pattern == path {
		return true
	}

	// Glob match
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Check if path is under pattern directory
	if strings.HasSuffix(pattern, "/**") {
		dir := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(path, dir+"/") || path == dir {
			return true
		}
	}

	return false
}

// matchToolPattern checks if a tool matches any allowed pattern
func (m *Manager) matchToolPattern(toolName string) bool {
	for name := range m.allowedTools {
		if m.matchPattern(name, toolName) {
			return true
		}
	}
	return false
}

// handleAsk handles the ask decision (must NOT be called while holding mu lock)
func (m *Manager) handleAsk(req *Request, rule *Rule) *Result {
	cacheKey := m.getCacheKey(req)

	// Check session cache with lock
	m.mu.RLock()
	if decision, ok := m.sessionDecisions[cacheKey]; ok {
		m.mu.RUnlock()
		if decision == DecisionAllowAll || decision == DecisionAllow {
			return &Result{Allowed: true, Reason: "cached allow decision"}
		}
		if decision == DecisionDeny {
			return &Result{Allowed: false, Reason: "cached deny decision"}
		}
	} else {
		m.mu.RUnlock()
	}

	// If no callback, deny by default
	m.mu.RLock()
	callback := m.askCallback
	m.mu.RUnlock()

	if callback == nil {
		return &Result{Allowed: false, Reason: "no permission callback configured"}
	}

	// Ask user (outside of lock)
	decision := callback(req)

	// Cache decision if applicable (with write lock)
	if decision == DecisionAllowAll {
		m.mu.Lock()
		m.sessionDecisions[cacheKey] = DecisionAllowAll
		m.mu.Unlock()
	}

	switch decision {
	case DecisionAllow, DecisionAllowOnce, DecisionAllowAll:
		return &Result{Allowed: true, Reason: "user approved"}
	default:
		return &Result{Allowed: false, Reason: "user denied"}
	}
}

// getCacheKey generates a cache key for a request
func (m *Manager) getCacheKey(req *Request) string {
	// For most tools, cache by tool name
	key := req.Tool

	// For file tools, include the path
	if m.isFileTool(req.Tool) {
		if path := m.extractPath(req); path != "" {
			key = fmt.Sprintf("%s:%s", req.Tool, path)
		}
	}

	// For bash, include the command prefix
	if req.Tool == "Bash" {
		if cmd := m.extractCommand(req); cmd != "" {
			// Use first word of command as key
			parts := strings.Fields(cmd)
			if len(parts) > 0 {
				key = fmt.Sprintf("Bash:%s", parts[0])
			}
		}
	}

	return key
}

// isReadOnlyTool checks if a tool is read-only
func (m *Manager) isReadOnlyTool(toolName string) bool {
	readOnlyTools := map[string]bool{
		"Read":      true,
		"Glob":      true,
		"Grep":      true,
		"LSP":       true,
		"WebFetch":  true,
		"WebSearch": true,
	}
	return readOnlyTools[toolName]
}

// isEditTool checks if a tool is for editing files
func (m *Manager) isEditTool(toolName string) bool {
	editTools := map[string]bool{
		"Edit":         true,
		"Write":        true,
		"NotebookEdit": true,
	}
	return editTools[toolName]
}

// isFileTool checks if a tool operates on files
func (m *Manager) isFileTool(toolName string) bool {
	fileTools := map[string]bool{
		"Read":         true,
		"Write":        true,
		"Edit":         true,
		"Glob":         true,
		"NotebookEdit": true,
	}
	return fileTools[toolName]
}

// extractPath extracts the file path from a request
func (m *Manager) extractPath(req *Request) string {
	if req.Params == nil {
		return ""
	}

	// Common parameter names for file paths
	pathKeys := []string{"file_path", "path", "notebook_path"}
	for _, key := range pathKeys {
		if path, ok := req.Params[key].(string); ok {
			return path
		}
	}

	return ""
}

// extractCommand extracts the command from a bash request
func (m *Manager) extractCommand(req *Request) string {
	if req.Params == nil {
		return ""
	}

	if cmd, ok := req.Params["command"].(string); ok {
		return cmd
	}

	return ""
}

// ClearSessionCache clears the session decision cache
func (m *Manager) ClearSessionCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionDecisions = make(map[string]Decision)
}

// FormatRequest formats a permission request for display
func FormatRequest(req *Request) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tool: %s\n", req.Tool))

	if path := (&Manager{}).extractPath(req); path != "" {
		sb.WriteString(fmt.Sprintf("Path: %s\n", path))
	}

	if cmd := (&Manager{}).extractCommand(req); cmd != "" {
		sb.WriteString(fmt.Sprintf("Command: %s\n", cmd))
	}

	return sb.String()
}

// DefaultRules returns the default permission rules
func DefaultRules() []Rule {
	return []Rule{
		// Allow read operations by default
		{Tool: "Read", Action: DecisionAllow},
		{Tool: "Glob", Action: DecisionAllow},
		{Tool: "Grep", Action: DecisionAllow},
		{Tool: "LSP", Action: DecisionAllow},

		// Ask for write operations
		{Tool: "Write", Action: DecisionAsk},
		{Tool: "Edit", Action: DecisionAsk},
		{Tool: "Bash", Action: DecisionAsk},

		// Deny dangerous patterns
		{Tool: "Bash", Action: DecisionDeny, Commands: []string{"rm -rf /*", "sudo rm -rf *"}},
	}
}
