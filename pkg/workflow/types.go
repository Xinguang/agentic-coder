// Package workflow implements multi-agent workflow with concurrent execution
package workflow

import (
	"time"
)

// Role represents an agent role in the workflow
type Role string

const (
	RoleManager   Role = "manager"
	RoleExecutor  Role = "executor"
	RoleReviewer  Role = "reviewer"
	RoleFixer     Role = "fixer"
	RoleEvaluator Role = "evaluator"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusReady     TaskStatus = "ready"     // Dependencies satisfied, waiting to execute
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusReviewing TaskStatus = "reviewing"
	TaskStatusFixing    TaskStatus = "fixing"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task represents a single task in the workflow
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"` // 1=highest, 5=lowest
	DependsOn   []string   `json:"depends_on"`
	Status      TaskStatus `json:"status"`

	// Execution tracking
	RetryCount int    `json:"retry_count"`
	AssignedTo string `json:"assigned_to"`

	// Results
	Execution *Execution `json:"execution,omitempty"`
	Reviews   []*Review  `json:"reviews,omitempty"`

	// Resource requirements (for locking)
	Resources []string `json:"resources,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TaskPlan represents the complete plan from Manager
type TaskPlan struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Requirement string    `json:"requirement"`
	Analysis    string    `json:"analysis"`
	Tasks       []*Task   `json:"tasks"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}

// ToolUsage records a single tool invocation
type ToolUsage struct {
	Name     string                 `json:"name"`
	Input    map[string]interface{} `json:"input"`
	Output   string                 `json:"output"`
	Success  bool                   `json:"success"`
	Duration time.Duration          `json:"duration"`
}

// Execution represents a task execution result
type Execution struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	ExecutorID string `json:"executor_id"`

	// What was done
	ToolsUsed    []ToolUsage `json:"tools_used"`
	FilesChanged []string    `json:"files_changed"`
	Output       string      `json:"output"`

	// Status
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	// Timestamps
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
}

// ReviewResult represents the result of a review
type ReviewResult string

const (
	ReviewPass   ReviewResult = "pass"
	ReviewFail   ReviewResult = "fail"
	ReviewReplan ReviewResult = "replan"
)

// Issue represents a problem found during review
type Issue struct {
	Severity    string `json:"severity"` // critical, major, minor
	Type        string `json:"type"`     // bug, security, style, performance
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// Review represents a review of an execution
type Review struct {
	ID          string `json:"id"`
	ExecutionID string `json:"execution_id"`
	ReviewerID  string `json:"reviewer_id"`

	Result   ReviewResult `json:"result"`
	Score    int          `json:"score,omitempty"` // 0-100
	Comments string       `json:"comments"`
	Issues   []Issue      `json:"issues,omitempty"`

	// For failed reviews
	CanAutoFix    bool   `json:"can_auto_fix"`
	FixSuggestion string `json:"fix_suggestion,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// Evaluation represents the final evaluation
type Evaluation struct {
	ID          string `json:"id"`
	PlanID      string `json:"plan_id"`
	EvaluatorID string `json:"evaluator_id"`

	MeetsRequirement bool `json:"meets_requirement"`
	QualityScore     int  `json:"quality_score"` // 0-100

	Strengths   []string `json:"strengths"`
	Weaknesses  []string `json:"weaknesses"`
	Suggestions []string `json:"suggestions"`

	CreatedAt time.Time `json:"created_at"`
}

// TaskSummary provides a brief summary of a task for reporting
type TaskSummary struct {
	TaskID     string        `json:"task_id"`
	Title      string        `json:"title"`
	Status     TaskStatus    `json:"status"`
	RetryCount int           `json:"retry_count"`
	Duration   time.Duration `json:"duration"`
}

// FinalReport represents the final report from Manager
type FinalReport struct {
	ID     string `json:"id"`
	PlanID string `json:"plan_id"`

	// Summary
	Requirement string `json:"requirement"`
	Status      string `json:"status"` // completed, partial, failed

	// Statistics
	TotalTasks   int `json:"total_tasks"`
	Completed    int `json:"completed"`
	Failed       int `json:"failed"`
	TotalRetries int `json:"total_retries"`

	// Results
	TaskSummaries []TaskSummary `json:"task_summaries"`
	Evaluation    *Evaluation   `json:"evaluation"`

	// Conclusion
	Conclusion string   `json:"conclusion"`
	NextSteps  []string `json:"next_steps"`

	// Metrics
	TotalDuration time.Duration `json:"total_duration"`
	TotalTokens   int           `json:"total_tokens"`
	TotalCost     float64       `json:"total_cost"`

	CreatedAt time.Time `json:"created_at"`
}

// ProgressEvent represents a workflow progress update
type ProgressEvent struct {
	Type      string     `json:"type"` // analyzing, plan_created, task_started, reviewing, fixing, task_completed, evaluating, completed
	TaskID    string     `json:"task_id,omitempty"`
	TaskTitle string     `json:"task_title,omitempty"`
	Status    TaskStatus `json:"status,omitempty"`
	Message   string     `json:"message,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}
