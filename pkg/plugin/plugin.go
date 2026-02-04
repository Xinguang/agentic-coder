// Package plugin implements the plugin system
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Plugin represents a loaded plugin
type Plugin struct {
	// Manifest contains plugin metadata
	Manifest Manifest

	// Root is the plugin root directory
	Root string

	// Commands are loaded slash commands
	Commands []Command

	// Skills are loaded skills
	Skills []Skill

	// Agents are loaded subagents
	Agents []Agent

	// Hooks are loaded hooks
	Hooks []Hook
}

// Manifest represents plugin.json
type Manifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	License     string   `json:"license,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`

	// Component paths
	Commands string `json:"commands,omitempty"` // default: commands/
	Skills   string `json:"skills,omitempty"`   // default: skills/
	Agents   string `json:"agents,omitempty"`   // default: agents/
	Hooks    string `json:"hooks,omitempty"`    // default: hooks/

	// MCP server configuration
	MCP []MCPConfig `json:"mcp,omitempty"`

	// Settings schema
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"` // stdio, sse, http
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Transport string            `json:"transport,omitempty"`
}

// Command represents a slash command
type Command struct {
	// Name is the command name (without /)
	Name string

	// Description is a short description
	Description string

	// Content is the command template content
	Content string

	// Args are command arguments
	Args []CommandArg

	// Plugin is the source plugin name
	Plugin string
}

// CommandArg represents a command argument
type CommandArg struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
	Type        string `yaml:"type,omitempty"` // string, file, boolean
}

// Skill represents a skill
type Skill struct {
	// Name is the skill identifier
	Name string

	// Description is when to use this skill
	Description string

	// Content is the skill content
	Content string

	// Plugin is the source plugin name
	Plugin string
}

// Agent represents a subagent
type Agent struct {
	// Name is the agent identifier
	Name string

	// Description is when to spawn this agent
	Description string

	// SystemPrompt is the agent's system prompt
	SystemPrompt string

	// AllowedTools lists tools this agent can use
	AllowedTools []string

	// DisallowedTools lists tools this agent cannot use
	DisallowedTools []string

	// Model override
	Model string

	// Plugin is the source plugin name
	Plugin string
}

// Hook represents a hook definition
type Hook struct {
	// Event is the hook event type
	Event string

	// Matcher defines when the hook fires
	Matcher HookMatcher

	// Commands to execute
	Commands []HookCommand

	// Plugin is the source plugin name
	Plugin string
}

// HookMatcher defines hook matching criteria
type HookMatcher struct {
	ToolName []string `yaml:"toolName,omitempty"`
	Path     []string `yaml:"path,omitempty"`
	Command  []string `yaml:"command,omitempty"`
}

// HookCommand represents a hook command
type HookCommand struct {
	Type    string `yaml:"type"` // command, prompt
	Command string `yaml:"command,omitempty"`
	Prompt  string `yaml:"prompt,omitempty"`
	Timeout int    `yaml:"timeout,omitempty"`
}

// Manager manages loaded plugins
type Manager struct {
	plugins     map[string]*Plugin
	searchPaths []string

	mu sync.RWMutex
}

// NewManager creates a new plugin manager
func NewManager(searchPaths []string) *Manager {
	return &Manager{
		plugins:     make(map[string]*Plugin),
		searchPaths: searchPaths,
	}
}

// LoadAll loads all plugins from search paths
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, searchPath := range m.searchPaths {
		if err := m.loadFromPath(searchPath); err != nil {
			// Log but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to load plugins from %s: %v\n", searchPath, err)
		}
	}

	return nil
}

// loadFromPath loads plugins from a directory
func (m *Manager) loadFromPath(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(path, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.json")

		if _, err := os.Stat(manifestPath); err != nil {
			continue // No plugin.json, skip
		}

		plugin, err := m.loadPlugin(pluginDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load plugin %s: %v\n", entry.Name(), err)
			continue
		}

		m.plugins[plugin.Manifest.Name] = plugin
	}

	return nil
}

// loadPlugin loads a single plugin
func (m *Manager) loadPlugin(dir string) (*Plugin, error) {
	// Read manifest
	manifestPath := filepath.Join(dir, "plugin.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	plugin := &Plugin{
		Manifest: manifest,
		Root:     dir,
	}

	// Set defaults
	if manifest.Commands == "" {
		manifest.Commands = "commands"
	}
	if manifest.Skills == "" {
		manifest.Skills = "skills"
	}
	if manifest.Agents == "" {
		manifest.Agents = "agents"
	}
	if manifest.Hooks == "" {
		manifest.Hooks = "hooks"
	}

	// Load components
	plugin.Commands = m.loadCommands(dir, manifest.Commands, manifest.Name)
	plugin.Skills = m.loadSkills(dir, manifest.Skills, manifest.Name)
	plugin.Agents = m.loadAgents(dir, manifest.Agents, manifest.Name)
	plugin.Hooks = m.loadHooks(dir, manifest.Hooks, manifest.Name)

	return plugin, nil
}

// loadCommands loads slash commands from directory
func (m *Manager) loadCommands(pluginDir, commandsDir, pluginName string) []Command {
	dir := filepath.Join(pluginDir, commandsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var commands []Command
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		cmd := parseCommand(string(content))
		cmd.Name = strings.TrimSuffix(name, ".md")
		cmd.Plugin = pluginName
		commands = append(commands, cmd)
	}

	return commands
}

// loadSkills loads skills from directory
func (m *Manager) loadSkills(pluginDir, skillsDir, pluginName string) []Skill {
	dir := filepath.Join(pluginDir, skillsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		skill := parseSkill(string(content))
		skill.Name = strings.TrimSuffix(name, ".md")
		skill.Plugin = pluginName
		skills = append(skills, skill)
	}

	return skills
}

// loadAgents loads subagents from directory
func (m *Manager) loadAgents(pluginDir, agentsDir, pluginName string) []Agent {
	dir := filepath.Join(pluginDir, agentsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var agents []Agent
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		agent := parseAgent(string(content))
		agent.Name = strings.TrimSuffix(name, ".md")
		agent.Plugin = pluginName
		agents = append(agents, agent)
	}

	return agents
}

// loadHooks loads hooks from directory
func (m *Manager) loadHooks(pluginDir, hooksDir, pluginName string) []Hook {
	dir := filepath.Join(pluginDir, hooksDir)

	// Try hooks.json first
	hooksFile := filepath.Join(dir, "hooks.json")
	if data, err := os.ReadFile(hooksFile); err == nil {
		var hooks []Hook
		if err := json.Unmarshal(data, &hooks); err == nil {
			for i := range hooks {
				hooks[i].Plugin = pluginName
			}
			return hooks
		}
	}

	// Try hooks.yaml
	hooksFile = filepath.Join(dir, "hooks.yaml")
	if data, err := os.ReadFile(hooksFile); err == nil {
		var hooks []Hook
		if err := yaml.Unmarshal(data, &hooks); err == nil {
			for i := range hooks {
				hooks[i].Plugin = pluginName
			}
			return hooks
		}
	}

	return nil
}

// Get returns a plugin by name
func (m *Manager) Get(name string) *Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins[name]
}

// List returns all loaded plugins
func (m *Manager) List() []*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// GetAllCommands returns all commands from all plugins
func (m *Manager) GetAllCommands() []Command {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var commands []Command
	for _, p := range m.plugins {
		commands = append(commands, p.Commands...)
	}
	return commands
}

// GetAllSkills returns all skills from all plugins
func (m *Manager) GetAllSkills() []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var skills []Skill
	for _, p := range m.plugins {
		skills = append(skills, p.Skills...)
	}
	return skills
}

// GetAllAgents returns all agents from all plugins
func (m *Manager) GetAllAgents() []Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var agents []Agent
	for _, p := range m.plugins {
		agents = append(agents, p.Agents...)
	}
	return agents
}

// parseCommand parses command content with YAML frontmatter
func parseCommand(content string) Command {
	cmd := Command{}

	// Split frontmatter and content
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		// Parse YAML frontmatter
		var frontmatter struct {
			Description string       `yaml:"description"`
			Args        []CommandArg `yaml:"args"`
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err == nil {
			cmd.Description = frontmatter.Description
			cmd.Args = frontmatter.Args
		}
		cmd.Content = strings.TrimSpace(parts[2])
	} else {
		cmd.Content = content
	}

	return cmd
}

// parseSkill parses skill content with YAML frontmatter
func parseSkill(content string) Skill {
	skill := Skill{}

	// Split frontmatter and content
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		// Parse YAML frontmatter
		var frontmatter struct {
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err == nil {
			skill.Description = frontmatter.Description
		}
		skill.Content = strings.TrimSpace(parts[2])
	} else {
		skill.Content = content
	}

	return skill
}

// parseAgent parses agent content with YAML frontmatter
func parseAgent(content string) Agent {
	agent := Agent{}

	// Split frontmatter and content
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		// Parse YAML frontmatter
		var frontmatter struct {
			Description     string   `yaml:"description"`
			AllowedTools    []string `yaml:"allowedTools"`
			DisallowedTools []string `yaml:"disallowedTools"`
			Model           string   `yaml:"model"`
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err == nil {
			agent.Description = frontmatter.Description
			agent.AllowedTools = frontmatter.AllowedTools
			agent.DisallowedTools = frontmatter.DisallowedTools
			agent.Model = frontmatter.Model
		}
		agent.SystemPrompt = strings.TrimSpace(parts[2])
	} else {
		agent.SystemPrompt = content
	}

	return agent
}
