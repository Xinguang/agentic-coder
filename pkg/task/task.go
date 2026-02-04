// Package task implements the subagent/task system
package task

import (
	"context"
	"sync"
	"time"

	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/session"
	"github.com/xinguang/agentic-coder/pkg/tool"
	"github.com/google/uuid"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task represents a subagent task
type Task struct {
	// ID is the unique task identifier
	ID string

	// Description is a short task description
	Description string

	// Prompt is the task prompt
	Prompt string

	// SubagentType is the type of subagent
	SubagentType string

	// Status is the current task status
	Status TaskStatus

	// Result contains the task output
	Result *TaskResult

	// ParentID is the parent task ID (for nested tasks)
	ParentID string

	// Engine is the task's agent engine
	Engine *engine.Engine

	// Session is the task's session
	Session *session.Session

	// Created is the creation time
	Created time.Time

	// Started is when execution started
	Started *time.Time

	// Completed is when execution completed
	Completed *time.Time

	// Error contains any error message
	Error string

	// RunInBackground indicates if task runs async
	RunInBackground bool

	// Cancel function
	cancel context.CancelFunc

	mu sync.RWMutex
}

// TaskResult contains the result of a task
type TaskResult struct {
	// Output is the text output from the task
	Output string

	// Metadata contains additional result data
	Metadata map[string]interface{}

	// Usage contains token usage info
	Usage *provider.Usage
}

// Manager manages tasks and subagents
type Manager struct {
	tasks       map[string]*Task
	tasksByType map[string][]*Task

	// Provider for creating subagent engines
	providerFactory func(model string) provider.AIProvider

	// Registry for creating filtered tool sets
	baseRegistry *tool.Registry

	// Subagent configurations
	agentConfigs map[string]*AgentConfig

	mu sync.RWMutex
}

// AgentConfig defines a subagent type configuration
type AgentConfig struct {
	// Name is the agent type identifier
	Name string

	// Description explains when to use this agent
	Description string

	// SystemPrompt is the agent's system prompt
	SystemPrompt string

	// AllowedTools restricts available tools
	AllowedTools []string

	// DisallowedTools removes specific tools
	DisallowedTools []string

	// Model override (defaults to parent's model)
	Model string

	// MaxIterations limits iterations
	MaxIterations int

	// Color for UI display
	Color string
}

// NewManager creates a new task manager
func NewManager(providerFactory func(string) provider.AIProvider, registry *tool.Registry) *Manager {
	m := &Manager{
		tasks:           make(map[string]*Task),
		tasksByType:     make(map[string][]*Task),
		providerFactory: providerFactory,
		baseRegistry:    registry,
		agentConfigs:    make(map[string]*AgentConfig),
	}

	// Register built-in agents
	m.registerBuiltinAgents()

	return m
}

// registerBuiltinAgents registers default subagent types
func (m *Manager) registerBuiltinAgents() {
	// Explore agent - for codebase exploration
	m.RegisterAgent(&AgentConfig{
		Name:        "Explore",
		Description: "Fast agent for exploring codebases. Use for finding files, searching code, or answering questions about the codebase.",
		SystemPrompt: `You are a fast exploration agent. Your task is to quickly search and analyze codebases.

Focus on:
- Finding relevant files and code patterns
- Understanding code structure
- Providing concise answers

Be efficient and direct. Use search tools strategically to minimize iterations.`,
		AllowedTools:  []string{"Glob", "Grep", "Read", "Bash"},
		MaxIterations: 20,
		Color:         "#3B82F6",
	})

	// Plan agent - for implementation planning
	m.RegisterAgent(&AgentConfig{
		Name:        "Plan",
		Description: "Software architect agent for designing implementation plans. Use when planning implementation strategy for a task.",
		SystemPrompt: `You are a software architect agent. Your task is to create detailed implementation plans.

Focus on:
- Understanding requirements thoroughly
- Identifying affected files and components
- Designing step-by-step implementation approach
- Considering edge cases and risks

Return a clear, actionable plan.`,
		AllowedTools:  []string{"Glob", "Grep", "Read"},
		MaxIterations: 30,
		Color:         "#8B5CF6",
	})

	// General purpose agent
	m.RegisterAgent(&AgentConfig{
		Name:        "general-purpose",
		Description: "General-purpose agent for complex multi-step tasks. Use when you need autonomous handling of research or implementation.",
		SystemPrompt: `You are a general-purpose assistant agent. Handle the task autonomously and return results.`,
		MaxIterations: 50,
		Color:         "#10B981",
	})
}

// RegisterAgent registers a new agent configuration
func (m *Manager) RegisterAgent(config *AgentConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentConfigs[config.Name] = config
}

// GetAgentConfig returns an agent configuration by name
func (m *Manager) GetAgentConfig(name string) *AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agentConfigs[name]
}

// ListAgents returns all registered agent configurations
func (m *Manager) ListAgents() []*AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*AgentConfig, 0, len(m.agentConfigs))
	for _, c := range m.agentConfigs {
		configs = append(configs, c)
	}
	return configs
}

// CreateTask creates a new task
func (m *Manager) CreateTask(opts *TaskOptions) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &Task{
		ID:              uuid.New().String(),
		Description:     opts.Description,
		Prompt:          opts.Prompt,
		SubagentType:    opts.SubagentType,
		Status:          TaskStatusPending,
		ParentID:        opts.ParentID,
		Created:         time.Now(),
		RunInBackground: opts.RunInBackground,
	}

	m.tasks[task.ID] = task
	m.tasksByType[opts.SubagentType] = append(m.tasksByType[opts.SubagentType], task)

	return task, nil
}

// TaskOptions holds options for creating a task
type TaskOptions struct {
	Description     string
	Prompt          string
	SubagentType    string
	ParentID        string
	Model           string
	RunInBackground bool
	CWD             string
}

// RunTask runs a task
func (m *Manager) RunTask(ctx context.Context, taskID string) error {
	task := m.GetTask(taskID)
	if task == nil {
		return ErrTaskNotFound
	}

	// Get agent config
	config := m.GetAgentConfig(task.SubagentType)
	if config == nil {
		config = &AgentConfig{
			Name:          task.SubagentType,
			MaxIterations: 50,
		}
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	task.cancel = cancel

	// Update status
	task.mu.Lock()
	task.Status = TaskStatusRunning
	now := time.Now()
	task.Started = &now
	task.mu.Unlock()

	// Create provider
	model := config.Model
	if model == "" {
		model = "sonnet"
	}
	prov := m.providerFactory(model)

	// Create filtered registry
	registry := m.baseRegistry
	if len(config.AllowedTools) > 0 || len(config.DisallowedTools) > 0 {
		registry = m.baseRegistry.FilteredRegistry(config.AllowedTools, config.DisallowedTools)
	}

	// Create session for this task
	sess := session.NewSession(&session.SessionOptions{
		Model:   provider.ResolveModel(model),
		Version: "1.0",
	})
	sess.IsSidechain = true
	sess.AgentID = task.SubagentType
	task.Session = sess

	// Create engine
	eng := engine.NewEngine(&engine.EngineOptions{
		Provider:      prov,
		Registry:      registry,
		Session:       sess,
		MaxIterations: config.MaxIterations,
		SystemPrompt:  config.SystemPrompt,
	})
	task.Engine = eng

	// Collect output
	var output string
	eng.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			output += text
		},
	})

	// Run engine
	err := eng.Run(ctx, task.Prompt)

	// Update status
	task.mu.Lock()
	completedAt := time.Now()
	task.Completed = &completedAt

	if ctx.Err() == context.Canceled {
		task.Status = TaskStatusCancelled
	} else if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
	} else {
		task.Status = TaskStatusCompleted
		task.Result = &TaskResult{
			Output: output,
		}
	}
	task.mu.Unlock()

	return err
}

// RunTaskAsync runs a task in the background
func (m *Manager) RunTaskAsync(ctx context.Context, taskID string) {
	go func() {
		_ = m.RunTask(ctx, taskID)
	}()
}

// GetTask returns a task by ID
func (m *Manager) GetTask(id string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}

// CancelTask cancels a running task
func (m *Manager) CancelTask(id string) error {
	task := m.GetTask(id)
	if task == nil {
		return ErrTaskNotFound
	}

	if task.cancel != nil {
		task.cancel()
	}

	return nil
}

// ListTasks returns all tasks
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// WaitForTask waits for a task to complete
func (m *Manager) WaitForTask(ctx context.Context, id string, timeout time.Duration) (*TaskResult, error) {
	task := m.GetTask(id)
	if task == nil {
		return nil, ErrTaskNotFound
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-timeoutCh:
			return nil, ErrTimeout

		case <-ticker.C:
			task.mu.RLock()
			status := task.Status
			result := task.Result
			errMsg := task.Error
			task.mu.RUnlock()

			switch status {
			case TaskStatusCompleted:
				return result, nil
			case TaskStatusFailed:
				return nil, &TaskError{Message: errMsg}
			case TaskStatusCancelled:
				return nil, ErrCancelled
			}
		}
	}
}

// Errors
var (
	ErrTaskNotFound = &TaskError{Message: "task not found"}
	ErrTimeout      = &TaskError{Message: "task timed out"}
	ErrCancelled    = &TaskError{Message: "task cancelled"}
)

// TaskError represents a task error
type TaskError struct {
	Message string
}

func (e *TaskError) Error() string {
	return e.Message
}
