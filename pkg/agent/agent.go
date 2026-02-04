// Package agent provides subagent management for specialized tasks
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Agent represents a specialized subagent
type Agent struct {
	// Metadata from YAML frontmatter
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Model       string   `yaml:"model,omitempty" json:"model,omitempty"` // sonnet, opus, haiku
	Tools       []string `yaml:"tools,omitempty" json:"tools,omitempty"` // Allowed tools
	Color       string   `yaml:"color,omitempty" json:"color,omitempty"` // Display color

	// System prompt content
	SystemPrompt string `yaml:"-" json:"system_prompt"`
	SourceFile   string `yaml:"-" json:"source_file"`
	PluginName   string `yaml:"-" json:"plugin_name"`
	Location     string `yaml:"-" json:"location"` // builtin, plugin, file
}

// Task represents a running agent task
type Task struct {
	ID          string
	AgentType   string
	Description string
	Prompt      string
	Status      TaskStatus
	Result      string
	Error       error
	StartTime   time.Time
	EndTime     time.Time
	Background  bool
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Registry manages agent registration and lookup
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// Register adds an agent to the registry
func (r *Registry) Register(agent *Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	r.agents[agent.Name] = agent
	return nil
}

// Get retrieves an agent by name
func (r *Registry) Get(name string) (*Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if agent, ok := r.agents[name]; ok {
		return agent, nil
	}

	return nil, fmt.Errorf("agent not found: %s", name)
}

// List returns all registered agents
func (r *Registry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

// LoadFromDirectory loads agents from a directory
func (r *Registry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".md" && ext != ".txt" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		agent, err := LoadAgentFromFile(path)
		if err != nil {
			continue
		}

		agent.Location = "file"
		if err := r.Register(agent); err != nil {
			continue
		}
	}

	return nil
}

// LoadAgentFromFile loads an agent from a file
func LoadAgentFromFile(path string) (*Agent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	agent, err := ParseAgent(string(content))
	if err != nil {
		return nil, err
	}

	agent.SourceFile = path

	if agent.Name == "" {
		baseName := filepath.Base(path)
		agent.Name = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	return agent, nil
}

// ParseAgent parses an agent from content with YAML frontmatter
func ParseAgent(content string) (*Agent, error) {
	agent := &Agent{}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			if err := yaml.Unmarshal([]byte(parts[1]), agent); err != nil {
				return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
			}
			agent.SystemPrompt = strings.TrimSpace(parts[2])
		} else {
			agent.SystemPrompt = content
		}
	} else {
		agent.SystemPrompt = content
	}

	return agent, nil
}

// Manager manages running agent tasks
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task

	registry *Registry

	// Callback for running agents
	runCallback func(ctx context.Context, task *Task, agent *Agent) (string, error)
}

// NewManager creates a new agent manager
func NewManager(registry *Registry) *Manager {
	return &Manager{
		tasks:    make(map[string]*Task),
		registry: registry,
	}
}

// SetRunCallback sets the callback for running agents
func (m *Manager) SetRunCallback(cb func(ctx context.Context, task *Task, agent *Agent) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runCallback = cb
}

// Launch launches a new agent task
func (m *Manager) Launch(ctx context.Context, agentType, description, prompt string, background bool) (*Task, error) {
	agent, err := m.registry.Get(agentType)
	if err != nil {
		return nil, err
	}

	task := &Task{
		ID:          generateTaskID(),
		AgentType:   agentType,
		Description: description,
		Prompt:      prompt,
		Status:      TaskStatusPending,
		StartTime:   time.Now(),
		Background:  background,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	if background {
		go m.runTask(ctx, task, agent)
	} else {
		m.runTask(ctx, task, agent)
	}

	return task, nil
}

// runTask runs an agent task
func (m *Manager) runTask(ctx context.Context, task *Task, agent *Agent) {
	m.mu.Lock()
	task.Status = TaskStatusRunning
	m.mu.Unlock()

	var result string
	var err error

	if m.runCallback != nil {
		result, err = m.runCallback(ctx, task, agent)
	} else {
		err = fmt.Errorf("no run callback configured")
	}

	m.mu.Lock()
	task.EndTime = time.Now()
	if ctx.Err() != nil {
		task.Status = TaskStatusCancelled
		task.Error = ctx.Err()
	} else if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err
	} else {
		task.Status = TaskStatusCompleted
		task.Result = result
	}
	m.mu.Unlock()
}

// GetTask retrieves a task by ID
func (m *Manager) GetTask(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if task, ok := m.tasks[id]; ok {
		return task, nil
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

// GetTaskOutput waits for and returns task output
func (m *Manager) GetTaskOutput(ctx context.Context, id string, block bool, timeout time.Duration) (*Task, error) {
	task, err := m.GetTask(id)
	if err != nil {
		return nil, err
	}

	if !block {
		return task, nil
	}

	// Wait for completion
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return task, ctx.Err()
		case <-ticker.C:
			task, _ = m.GetTask(id)
			if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
				return task, nil
			}
			if time.Now().After(deadline) {
				return task, fmt.Errorf("timeout waiting for task")
			}
		}
	}
}

// ListTasks returns all tasks
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// BuiltinAgents returns the built-in agents
func BuiltinAgents() []*Agent {
	return []*Agent{
		{
			Name:        "Explore",
			Description: "Fast agent for exploring codebases - finds files, searches code, answers questions",
			Model:       "haiku",
			Tools:       []string{"Read", "Glob", "Grep", "LSP"},
			Color:       "cyan",
			Location:    "builtin",
			SystemPrompt: `You are a fast codebase exploration agent. Your job is to quickly find information in the codebase.

When exploring:
1. Use Glob to find files by pattern
2. Use Grep to search for code patterns
3. Use Read to examine specific files
4. Use LSP for code navigation (definitions, references)

Be thorough but efficient. Return concise, actionable information.`,
		},
		{
			Name:        "Plan",
			Description: "Software architect agent for designing implementation plans",
			Model:       "sonnet",
			Tools:       []string{"Read", "Glob", "Grep", "LSP"},
			Color:       "yellow",
			Location:    "builtin",
			SystemPrompt: `You are a software architect agent. Your job is to design implementation plans.

When planning:
1. Understand the requirements fully
2. Explore existing code patterns
3. Identify key files and components
4. Design a step-by-step implementation plan
5. Consider edge cases and potential issues
6. Note any architectural trade-offs

Output a clear, actionable plan that can be executed step by step.`,
		},
		{
			Name:        "general-purpose",
			Description: "General-purpose agent for complex, multi-step tasks",
			Model:       "sonnet",
			Tools:       []string{"*"},
			Color:       "green",
			Location:    "builtin",
			SystemPrompt: `You are a general-purpose agent capable of handling complex tasks.

You have access to all tools and can:
- Read and write files
- Execute commands
- Search and explore code
- Make edits and changes

Work autonomously to complete the task. Be thorough and verify your work.`,
		},
		{
			Name:        "code-reviewer",
			Description: "Code review agent for reviewing changes",
			Model:       "sonnet",
			Tools:       []string{"Read", "Glob", "Grep", "Bash"},
			Color:       "magenta",
			Location:    "builtin",
			SystemPrompt: `You are a code review agent. Review code changes thoroughly.

Focus on:
1. Code correctness and logic errors
2. Potential bugs and edge cases
3. Security vulnerabilities
4. Performance issues
5. Code style and readability
6. Test coverage

Provide specific, actionable feedback with line references.`,
		},
		{
			Name:        "test-runner",
			Description: "Agent for running and fixing tests",
			Model:       "sonnet",
			Tools:       []string{"Read", "Edit", "Bash", "Glob", "Grep"},
			Color:       "blue",
			Location:    "builtin",
			SystemPrompt: `You are a test runner agent. Your job is to run tests and fix failures.

Process:
1. Detect the test framework
2. Run the test suite
3. Analyze any failures
4. Fix issues in the code
5. Re-run tests until they pass

Be systematic and fix root causes, not symptoms.`,
		},
	}
}
