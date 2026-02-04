// Package hook implements lifecycle hooks for the agent
package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xinguang/agentic-coder/pkg/tool"
	"gopkg.in/yaml.v3"
)

// HookEvent represents the type of hook event
type HookEvent string

const (
	EventPreToolUse        HookEvent = "PreToolUse"
	EventPostToolUse       HookEvent = "PostToolUse"
	EventStop              HookEvent = "Stop"
	EventSubagentStop      HookEvent = "SubagentStop"
	EventSessionStart      HookEvent = "SessionStart"
	EventSessionEnd        HookEvent = "SessionEnd"
	EventUserPromptSubmit  HookEvent = "UserPromptSubmit"
	EventPreCompact        HookEvent = "PreCompact"
	EventNotification      HookEvent = "Notification"
)

// HookConfig represents a hook configuration
type HookConfig struct {
	// Matcher identifies when this hook should run
	Matcher HookMatcher `json:"matcher" yaml:"matcher"`

	// Hooks are the commands to execute
	Hooks []HookCommand `json:"hooks" yaml:"hooks"`
}

// HookMatcher defines conditions for when a hook should run
type HookMatcher struct {
	// Event type to match
	Event HookEvent `json:"event" yaml:"event"`

	// Tool name patterns (glob) - for tool-related events
	ToolName []string `json:"toolName,omitempty" yaml:"toolName,omitempty"`

	// Path patterns (glob) - for file-related operations
	Path []string `json:"path,omitempty" yaml:"path,omitempty"`

	// Command patterns (regex) - for Bash tool
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
}

// HookCommand represents a command to execute
type HookCommand struct {
	// Type: "command" or "prompt"
	Type string `json:"type" yaml:"type"`

	// Command is the shell command to execute (for type=command)
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// Prompt is the prompt to send to Claude (for type=prompt)
	Prompt string `json:"prompt,omitempty" yaml:"prompt,omitempty"`

	// Timeout in milliseconds
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// HookResult represents the result of running a hook
type HookResult struct {
	// Blocked indicates if the action should be blocked
	Blocked bool `json:"blocked"`

	// Message is an optional message to show
	Message string `json:"message,omitempty"`

	// ModifiedInput contains modified input parameters
	ModifiedInput map[string]interface{} `json:"modifiedInput,omitempty"`

	// Output from the hook command
	Output string `json:"output,omitempty"`
}

// Manager manages hooks
type Manager struct {
	hooks     []HookConfig
	configDir string

	mu sync.RWMutex
}

// NewManager creates a new hook manager
func NewManager(configDir string) *Manager {
	return &Manager{
		hooks:     make([]HookConfig, 0),
		configDir: configDir,
	}
}

// LoadHooks loads hooks from configuration
func (m *Manager) LoadHooks(configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No hooks configured
		}
		return err
	}

	// Try JSON first, then YAML
	var configs []HookConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		// Try YAML
		if err := yaml.Unmarshal(data, &configs); err != nil {
			return fmt.Errorf("failed to parse hooks config: %w", err)
		}
	}

	m.hooks = configs
	return nil
}

// RegisterHook registers a hook programmatically
func (m *Manager) RegisterHook(config HookConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, config)
}

// RunPreToolUse runs pre-tool-use hooks
func (m *Manager) RunPreToolUse(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
	m.mu.RLock()
	hooks := m.getMatchingHooks(EventPreToolUse, toolName, input)
	m.mu.RUnlock()

	for _, hook := range hooks {
		for _, cmd := range hook.Hooks {
			result := m.executeHook(ctx, cmd, map[string]interface{}{
				"tool_name": toolName,
				"input":     input,
			})

			if result.Blocked {
				return result
			}
		}
	}

	return &HookResult{Blocked: false}
}

// RunPostToolUse runs post-tool-use hooks
func (m *Manager) RunPostToolUse(ctx context.Context, toolName string, input map[string]interface{}, output *tool.Output) {
	m.mu.RLock()
	hooks := m.getMatchingHooks(EventPostToolUse, toolName, input)
	m.mu.RUnlock()

	for _, hook := range hooks {
		for _, cmd := range hook.Hooks {
			m.executeHook(ctx, cmd, map[string]interface{}{
				"tool_name": toolName,
				"input":     input,
				"output":    output,
			})
		}
	}
}

// RunStop runs stop hooks
func (m *Manager) RunStop(ctx context.Context, reason string) {
	m.mu.RLock()
	hooks := m.getMatchingHooks(EventStop, "", nil)
	m.mu.RUnlock()

	for _, hook := range hooks {
		for _, cmd := range hook.Hooks {
			m.executeHook(ctx, cmd, map[string]interface{}{
				"reason": reason,
			})
		}
	}
}

// RunSessionStart runs session start hooks
func (m *Manager) RunSessionStart(ctx context.Context, sessionID string) {
	m.mu.RLock()
	hooks := m.getMatchingHooks(EventSessionStart, "", nil)
	m.mu.RUnlock()

	for _, hook := range hooks {
		for _, cmd := range hook.Hooks {
			m.executeHook(ctx, cmd, map[string]interface{}{
				"session_id": sessionID,
			})
		}
	}
}

// RunSessionEnd runs session end hooks
func (m *Manager) RunSessionEnd(ctx context.Context, sessionID string) {
	m.mu.RLock()
	hooks := m.getMatchingHooks(EventSessionEnd, "", nil)
	m.mu.RUnlock()

	for _, hook := range hooks {
		for _, cmd := range hook.Hooks {
			m.executeHook(ctx, cmd, map[string]interface{}{
				"session_id": sessionID,
			})
		}
	}
}

// RunUserPromptSubmit runs user prompt submit hooks
func (m *Manager) RunUserPromptSubmit(ctx context.Context, prompt string) *HookResult {
	m.mu.RLock()
	hooks := m.getMatchingHooks(EventUserPromptSubmit, "", nil)
	m.mu.RUnlock()

	for _, hook := range hooks {
		for _, cmd := range hook.Hooks {
			result := m.executeHook(ctx, cmd, map[string]interface{}{
				"prompt": prompt,
			})

			if result.Blocked {
				return result
			}
		}
	}

	return &HookResult{Blocked: false}
}

// getMatchingHooks returns hooks that match the given event and context
func (m *Manager) getMatchingHooks(event HookEvent, toolName string, input map[string]interface{}) []HookConfig {
	var matching []HookConfig

	for _, hook := range m.hooks {
		if hook.Matcher.Event != event {
			continue
		}

		// Check tool name match
		if len(hook.Matcher.ToolName) > 0 && toolName != "" {
			if !matchAny(toolName, hook.Matcher.ToolName) {
				continue
			}
		}

		// Check path match (for file operations)
		if len(hook.Matcher.Path) > 0 && input != nil {
			if path, ok := input["file_path"].(string); ok {
				if !matchAny(path, hook.Matcher.Path) {
					continue
				}
			}
		}

		// Check command match (for Bash tool)
		if len(hook.Matcher.Command) > 0 && input != nil {
			if cmd, ok := input["command"].(string); ok {
				if !matchAny(cmd, hook.Matcher.Command) {
					continue
				}
			}
		}

		matching = append(matching, hook)
	}

	return matching
}

// executeHook executes a single hook command
func (m *Manager) executeHook(ctx context.Context, cmd HookCommand, data map[string]interface{}) *HookResult {
	switch cmd.Type {
	case "command":
		return m.executeCommand(ctx, cmd, data)
	case "prompt":
		return m.executePrompt(ctx, cmd, data)
	default:
		return &HookResult{
			Blocked: false,
			Message: fmt.Sprintf("unknown hook type: %s", cmd.Type),
		}
	}
}

// executeCommand executes a shell command hook
func (m *Manager) executeCommand(ctx context.Context, cmd HookCommand, data map[string]interface{}) *HookResult {
	// Expand environment variables and template
	command := expandTemplate(cmd.Command, data)

	// Set environment variables
	env := os.Environ()
	for k, v := range data {
		if s, ok := v.(string); ok {
			env = append(env, fmt.Sprintf("HOOK_%s=%s", strings.ToUpper(k), s))
		}
	}

	// Also pass as JSON
	jsonData, _ := json.Marshal(data)
	env = append(env, fmt.Sprintf("HOOK_DATA=%s", string(jsonData)))

	// Execute command
	shellCmd := exec.CommandContext(ctx, "sh", "-c", command)
	shellCmd.Env = env
	shellCmd.Dir = m.configDir

	output, err := shellCmd.CombinedOutput()

	result := &HookResult{
		Output: string(output),
	}

	if err != nil {
		// Non-zero exit means block
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				// Exit code 2 means block
				result.Blocked = true
				result.Message = strings.TrimSpace(string(output))
			}
		}
	}

	return result
}

// executePrompt executes a prompt-based hook (for AI validation)
func (m *Manager) executePrompt(ctx context.Context, cmd HookCommand, data map[string]interface{}) *HookResult {
	// This would call the AI provider to validate
	// For now, return non-blocking result
	return &HookResult{
		Blocked: false,
		Message: "prompt hooks not yet implemented",
	}
}

// matchAny checks if the value matches any of the patterns
func matchAny(value string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, value)
		if matched {
			return true
		}
		// Also try exact match
		if pattern == value {
			return true
		}
		// Try prefix match for patterns ending with *
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}

// expandTemplate expands template variables in a string
func expandTemplate(template string, data map[string]interface{}) string {
	result := template
	for k, v := range data {
		if s, ok := v.(string); ok {
			result = strings.ReplaceAll(result, "${"+k+"}", s)
			result = strings.ReplaceAll(result, "$"+k, s)
		}
	}
	return result
}
