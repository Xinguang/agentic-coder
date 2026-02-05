// Package config provides configuration management for agentic-coder
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config represents the application configuration
type Config struct {
	mu sync.RWMutex

	// API Keys
	APIKeys map[string]string `json:"api_keys,omitempty"`

	// Model settings
	DefaultModel  string  `json:"default_model,omitempty"`
	MaxTokens     int     `json:"max_tokens,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	ThinkingLevel string  `json:"thinking_level,omitempty"` // high, medium, low, none

	// Session settings
	AutoSave        bool   `json:"auto_save,omitempty"`
	SessionDir      string `json:"session_dir,omitempty"`
	MaxIterations   int    `json:"max_iterations,omitempty"`
	CompactPercent  float64 `json:"compact_percent,omitempty"`

	// Permission settings
	PermissionMode  string   `json:"permission_mode,omitempty"` // default, plan, accept_edits, dont_ask, bypass
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// Hook settings
	Hooks []HookConfig `json:"hooks,omitempty"`

	// MCP settings
	MCPServers []MCPServerConfig `json:"mcp_servers,omitempty"`

	// Plugin settings
	PluginPaths []string `json:"plugin_paths,omitempty"`

	// Editor settings
	Editor       string `json:"editor,omitempty"`
	EditorArgs   string `json:"editor_args,omitempty"`

	// Logging
	Verbose  bool   `json:"verbose,omitempty"`
	LogLevel string `json:"log_level,omitempty"` // debug, info, warn, error
	LogFile  string `json:"log_file,omitempty"`

	// UI settings
	Theme        string `json:"theme,omitempty"` // dark, light
	StatusLine   bool   `json:"status_line,omitempty"`
	ShowThinking bool   `json:"show_thinking,omitempty"`

	// Git settings
	GitAutoCommit bool `json:"git_auto_commit,omitempty"`
	GitSignCommit bool `json:"git_sign_commit,omitempty"`

	// Extra custom settings
	Extra map[string]interface{} `json:"extra,omitempty"`

	// Internal
	configPath string
}

// HookConfig represents a hook configuration
type HookConfig struct {
	Event   string            `json:"event"`   // PreToolUse, PostToolUse, Stop, etc.
	Matcher string            `json:"matcher"` // Tool name pattern or "*"
	Command string            `json:"command"` // Shell command to run
	Timeout int               `json:"timeout"` // Timeout in seconds
	Env     map[string]string `json:"env,omitempty"`
}

// MCPServerConfig represents an MCP server configuration
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"` // stdio, sse, http
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	AutoStart bool              `json:"auto_start,omitempty"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultModel:   "sonnet",
		MaxTokens:      200000,
		Temperature:    0.0,
		ThinkingLevel:  "medium",
		AutoSave:       true,
		MaxIterations:  100,
		CompactPercent: 0.95,
		PermissionMode: "default",
		Verbose:        false,
		LogLevel:       "info",
		Theme:          "dark",
		StatusLine:     true,
		ShowThinking:   false,
		APIKeys:        make(map[string]string),
		Extra:          make(map[string]interface{}),
	}
}

// ConfigManager manages configuration loading and saving
type ConfigManager struct {
	globalConfig  *Config
	projectConfig *Config
	mergedConfig  *Config

	globalPath  string
	projectPath string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() (*ConfigManager, error) {
	cm := &ConfigManager{}

	// Determine config paths
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	cm.globalPath = configPath

	return cm, nil
}

// Load loads configuration from disk
func (cm *ConfigManager) Load(projectPath string) error {
	// Load global config
	cm.globalConfig = DefaultConfig()
	if err := cm.loadFromFile(cm.globalPath, cm.globalConfig); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load global config: %w", err)
	}
	cm.globalConfig.configPath = cm.globalPath

	// Load project config if provided
	if projectPath != "" {
		cm.projectPath = GetProjectConfigPath(projectPath)
		cm.projectConfig = DefaultConfig()
		if err := cm.loadFromFile(cm.projectPath, cm.projectConfig); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to load project config: %w", err)
		}
		cm.projectConfig.configPath = cm.projectPath
	}

	// Merge configs (project overrides global)
	cm.mergedConfig = cm.merge()

	return nil
}

// loadFromFile loads configuration from a file
func (cm *ConfigManager) loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	return nil
}

// merge merges global and project configs
func (cm *ConfigManager) merge() *Config {
	merged := DefaultConfig()

	// Start with global config
	if cm.globalConfig != nil {
		copyConfig(cm.globalConfig, merged)
	}

	// Override with project config
	if cm.projectConfig != nil {
		mergeConfig(cm.projectConfig, merged)
	}

	return merged
}

// copyConfig copies non-zero values from src to dst
func copyConfig(src, dst *Config) {
	if src.DefaultModel != "" {
		dst.DefaultModel = src.DefaultModel
	}
	if src.MaxTokens > 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if src.Temperature > 0 {
		dst.Temperature = src.Temperature
	}
	if src.ThinkingLevel != "" {
		dst.ThinkingLevel = src.ThinkingLevel
	}
	if src.SessionDir != "" {
		dst.SessionDir = src.SessionDir
	}
	if src.MaxIterations > 0 {
		dst.MaxIterations = src.MaxIterations
	}
	if src.CompactPercent > 0 {
		dst.CompactPercent = src.CompactPercent
	}
	if src.PermissionMode != "" {
		dst.PermissionMode = src.PermissionMode
	}
	if len(src.AllowedTools) > 0 {
		dst.AllowedTools = src.AllowedTools
	}
	if len(src.DisallowedTools) > 0 {
		dst.DisallowedTools = src.DisallowedTools
	}
	if len(src.Hooks) > 0 {
		dst.Hooks = src.Hooks
	}
	if len(src.MCPServers) > 0 {
		dst.MCPServers = src.MCPServers
	}
	if len(src.PluginPaths) > 0 {
		dst.PluginPaths = src.PluginPaths
	}
	if src.Editor != "" {
		dst.Editor = src.Editor
	}
	if src.LogLevel != "" {
		dst.LogLevel = src.LogLevel
	}
	if src.LogFile != "" {
		dst.LogFile = src.LogFile
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}

	// Boolean fields
	dst.AutoSave = src.AutoSave
	dst.Verbose = src.Verbose
	dst.StatusLine = src.StatusLine
	dst.ShowThinking = src.ShowThinking
	dst.GitAutoCommit = src.GitAutoCommit
	dst.GitSignCommit = src.GitSignCommit

	// Maps
	for k, v := range src.APIKeys {
		dst.APIKeys[k] = v
	}
	for k, v := range src.Extra {
		dst.Extra[k] = v
	}
}

// mergeConfig merges src into dst (only non-zero values)
func mergeConfig(src, dst *Config) {
	copyConfig(src, dst)
}

// Get returns the merged configuration
func (cm *ConfigManager) Get() *Config {
	if cm.mergedConfig == nil {
		return DefaultConfig()
	}
	return cm.mergedConfig
}

// Global returns the global configuration
func (cm *ConfigManager) Global() *Config {
	return cm.globalConfig
}

// Project returns the project configuration
func (cm *ConfigManager) Project() *Config {
	return cm.projectConfig
}

// Save saves the configuration to disk
func (cm *ConfigManager) Save(scope string) error {
	var cfg *Config
	var path string

	switch scope {
	case "global":
		cfg = cm.globalConfig
		path = cm.globalPath
	case "project":
		cfg = cm.projectConfig
		path = cm.projectPath
	default:
		return fmt.Errorf("invalid scope: %s", scope)
	}

	if cfg == nil {
		return fmt.Errorf("no %s config loaded", scope)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal with indentation
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Set sets a configuration value
func (c *Config) Set(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch key {
	case "default_model":
		c.DefaultModel = value.(string)
	case "max_tokens":
		c.MaxTokens = toInt(value)
	case "temperature":
		c.Temperature = toFloat(value)
	case "thinking_level":
		c.ThinkingLevel = value.(string)
	case "auto_save":
		c.AutoSave = value.(bool)
	case "max_iterations":
		c.MaxIterations = toInt(value)
	case "permission_mode":
		c.PermissionMode = value.(string)
	case "verbose":
		c.Verbose = value.(bool)
	case "log_level":
		c.LogLevel = value.(string)
	case "theme":
		c.Theme = value.(string)
	case "status_line":
		c.StatusLine = value.(bool)
	case "show_thinking":
		c.ShowThinking = value.(bool)
	default:
		// Store in extra
		c.Extra[key] = value
	}

	return nil
}

// GetString returns a string configuration value
func (c *Config) GetString(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch key {
	case "default_model":
		return c.DefaultModel
	case "thinking_level":
		return c.ThinkingLevel
	case "permission_mode":
		return c.PermissionMode
	case "log_level":
		return c.LogLevel
	case "theme":
		return c.Theme
	case "editor":
		return c.Editor
	default:
		if v, ok := c.Extra[key].(string); ok {
			return v
		}
		return ""
	}
}

// GetInt returns an integer configuration value
func (c *Config) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch key {
	case "max_tokens":
		return c.MaxTokens
	case "max_iterations":
		return c.MaxIterations
	default:
		if v, ok := c.Extra[key].(int); ok {
			return v
		}
		return 0
	}
}

// GetBool returns a boolean configuration value
func (c *Config) GetBool(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch key {
	case "auto_save":
		return c.AutoSave
	case "verbose":
		return c.Verbose
	case "status_line":
		return c.StatusLine
	case "show_thinking":
		return c.ShowThinking
	case "git_auto_commit":
		return c.GitAutoCommit
	case "git_sign_commit":
		return c.GitSignCommit
	default:
		if v, ok := c.Extra[key].(bool); ok {
			return v
		}
		return false
	}
}

// GetAPIKey returns an API key for a provider
func (c *Config) GetAPIKey(provider string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if key, ok := c.APIKeys[provider]; ok {
		return key
	}

	// Try environment variables
	envVars := map[string]string{
		"claude":   "ANTHROPIC_API_KEY",
		"openai":   "OPENAI_API_KEY",
		"gemini":   "GOOGLE_API_KEY",
		"deepseek": "DEEPSEEK_API_KEY",
	}

	if envVar, ok := envVars[provider]; ok {
		return os.Getenv(envVar)
	}

	return ""
}

// SetAPIKey sets an API key for a provider
func (c *Config) SetAPIKey(provider, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.APIKeys == nil {
		c.APIKeys = make(map[string]string)
	}
	c.APIKeys[provider] = key
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation error: %s - %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

// IsValid returns true if there are no errors
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// HasWarnings returns true if there are warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// Validate validates the configuration and returns validation results
func (c *Config) Validate() *ValidationResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := &ValidationResult{
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationError, 0),
	}

	// Validate model
	validModels := map[string]bool{
		"sonnet": true, "opus": true, "haiku": true,
		"gpt-4o": true, "gpt-4o-mini": true, "o1": true, "o1-mini": true, "o3-mini": true,
		"gemini": true, "gemini-2.0-flash": true, "gemini-1.5-pro": true,
		"deepseek": true, "deepseek-chat": true, "deepseek-reasoner": true,
		"llama": true, "qwen": true, "codex": true,
		"geminicli": true, "claudecli": true, "codexcli": true,
	}
	if c.DefaultModel != "" && !validModels[c.DefaultModel] {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "default_model",
			Value:   c.DefaultModel,
			Message: "unknown model, may not be supported",
		})
	}

	// Validate max_tokens
	if c.MaxTokens < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "max_tokens",
			Value:   c.MaxTokens,
			Message: "must be non-negative",
		})
	}
	if c.MaxTokens > 1000000 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "max_tokens",
			Value:   c.MaxTokens,
			Message: "very large value, may exceed model limits",
		})
	}

	// Validate temperature
	if c.Temperature < 0 || c.Temperature > 2 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "temperature",
			Value:   c.Temperature,
			Message: "must be between 0 and 2",
		})
	}

	// Validate thinking_level
	validThinkingLevels := map[string]bool{
		"high": true, "medium": true, "low": true, "none": true, "": true,
	}
	if !validThinkingLevels[c.ThinkingLevel] {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "thinking_level",
			Value:   c.ThinkingLevel,
			Message: "must be one of: high, medium, low, none",
		})
	}

	// Validate max_iterations
	if c.MaxIterations < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "max_iterations",
			Value:   c.MaxIterations,
			Message: "must be non-negative",
		})
	}
	if c.MaxIterations > 1000 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "max_iterations",
			Value:   c.MaxIterations,
			Message: "very large value, may cause long-running sessions",
		})
	}

	// Validate compact_percent
	if c.CompactPercent < 0 || c.CompactPercent > 1 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "compact_percent",
			Value:   c.CompactPercent,
			Message: "must be between 0 and 1",
		})
	}

	// Validate permission_mode
	validPermissionModes := map[string]bool{
		"default": true, "plan": true, "accept_edits": true, "dont_ask": true, "bypass": true, "": true,
	}
	if !validPermissionModes[c.PermissionMode] {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "permission_mode",
			Value:   c.PermissionMode,
			Message: "must be one of: default, plan, accept_edits, dont_ask, bypass",
		})
	}

	// Validate log_level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "": true,
	}
	if !validLogLevels[c.LogLevel] {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "log_level",
			Value:   c.LogLevel,
			Message: "must be one of: debug, info, warn, error",
		})
	}

	// Validate theme
	validThemes := map[string]bool{
		"dark": true, "light": true, "": true,
	}
	if !validThemes[c.Theme] {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "theme",
			Value:   c.Theme,
			Message: "unknown theme, using default",
		})
	}

	// Validate hooks
	validHookEvents := map[string]bool{
		"PreToolUse": true, "PostToolUse": true, "Stop": true,
		"SubagentStop": true, "SessionStart": true, "SessionEnd": true,
		"UserPromptSubmit": true, "PreCompact": true, "Notification": true,
	}
	for i, hook := range c.Hooks {
		if hook.Event == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("hooks[%d].event", i),
				Value:   hook.Event,
				Message: "event is required",
			})
		} else if !validHookEvents[hook.Event] {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   fmt.Sprintf("hooks[%d].event", i),
				Value:   hook.Event,
				Message: "unknown hook event",
			})
		}
		if hook.Command == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("hooks[%d].command", i),
				Value:   hook.Command,
				Message: "command is required",
			})
		}
	}

	// Validate MCP servers
	validMCPTypes := map[string]bool{
		"stdio": true, "sse": true, "http": true, "": true,
	}
	for i, server := range c.MCPServers {
		if server.Name == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp_servers[%d].name", i),
				Value:   server.Name,
				Message: "name is required",
			})
		}
		if !validMCPTypes[server.Type] {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp_servers[%d].type", i),
				Value:   server.Type,
				Message: "must be one of: stdio, sse, http",
			})
		}
		if server.Type == "stdio" && server.Command == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp_servers[%d].command", i),
				Value:   server.Command,
				Message: "command is required for stdio type",
			})
		}
		if (server.Type == "sse" || server.Type == "http") && server.URL == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp_servers[%d].url", i),
				Value:   server.URL,
				Message: "url is required for sse/http type",
			})
		}
	}

	// Validate plugin paths exist
	for i, path := range c.PluginPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   fmt.Sprintf("plugin_paths[%d]", i),
				Value:   path,
				Message: "path does not exist",
			})
		}
	}

	return result
}

// ValidateAndPrint validates and prints errors/warnings
func (c *Config) ValidateAndPrint() bool {
	result := c.Validate()

	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "Configuration errors:\n")
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %s (value: %v)\n", err.Field, err.Message, err.Value)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Configuration warnings:\n")
		for _, warn := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  ⚠ %s: %s (value: %v)\n", warn.Field, warn.Message, warn.Value)
		}
	}

	return result.IsValid()
}

// Helper functions
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// LoadClaudeMD loads CLAUDE.md content from project and home directories
func LoadClaudeMD(projectPath string) string {
	var content string

	// Load from home directory first
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		homeMD := filepath.Join(homeDir, ".claude", "CLAUDE.md")
		if data, err := os.ReadFile(homeMD); err == nil {
			content = string(data)
		}
	}

	// Load from project directory (overrides)
	if projectPath != "" {
		projectMD := filepath.Join(projectPath, "CLAUDE.md")
		if data, err := os.ReadFile(projectMD); err == nil {
			if content != "" {
				content += "\n\n"
			}
			content += string(data)
		}

		// Also check .claude directory
		projectClaudeMD := filepath.Join(projectPath, ".claude", "CLAUDE.md")
		if data, err := os.ReadFile(projectClaudeMD); err == nil {
			if content != "" {
				content += "\n\n"
			}
			content += string(data)
		}
	}

	return content
}
