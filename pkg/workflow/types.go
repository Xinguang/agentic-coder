// Package workflow implements multi-agent workflow with concurrent execution
package workflow

import (
	"github.com/xinguang/agentic-coder/pkg/workflow/agent"
)

// Re-export types from agent package
type (
	Role          = agent.Role
	TaskStatus    = agent.TaskStatus
	ReviewResult  = agent.ReviewResult
	Task          = agent.Task
	TaskPlan      = agent.TaskPlan
	ToolUsage     = agent.ToolUsage
	Execution     = agent.Execution
	Issue         = agent.Issue
	Review        = agent.Review
	Evaluation    = agent.Evaluation
	TaskSummary   = agent.TaskSummary
	FinalReport   = agent.FinalReport
	ProgressEvent = agent.ProgressEvent
)

// Re-export constants
const (
	RoleManager   = agent.RoleManager
	RoleExecutor  = agent.RoleExecutor
	RoleReviewer  = agent.RoleReviewer
	RoleFixer     = agent.RoleFixer
	RoleEvaluator = agent.RoleEvaluator

	TaskStatusPending   = agent.TaskStatusPending
	TaskStatusReady     = agent.TaskStatusReady
	TaskStatusRunning   = agent.TaskStatusRunning
	TaskStatusReviewing = agent.TaskStatusReviewing
	TaskStatusFixing    = agent.TaskStatusFixing
	TaskStatusCompleted = agent.TaskStatusCompleted
	TaskStatusFailed    = agent.TaskStatusFailed
	TaskStatusCancelled = agent.TaskStatusCancelled

	ReviewPass   = agent.ReviewPass
	ReviewFail   = agent.ReviewFail
	ReviewReplan = agent.ReviewReplan
)
