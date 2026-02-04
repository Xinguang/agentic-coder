// Package builtin provides built-in tools
package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// PlanModeCallback is called when plan mode state changes
type PlanModeCallback func(entering bool) error

// EnterPlanModeTool transitions into plan mode
type EnterPlanModeTool struct {
	callback PlanModeCallback
	inPlan   *bool
}

// NewEnterPlanModeTool creates a new enter plan mode tool
func NewEnterPlanModeTool(inPlan *bool, callback PlanModeCallback) *EnterPlanModeTool {
	return &EnterPlanModeTool{
		callback: callback,
		inPlan:   inPlan,
	}
}

func (t *EnterPlanModeTool) Name() string {
	return "EnterPlanMode"
}

func (t *EnterPlanModeTool) Description() string {
	return `Enter plan mode for implementation planning.

Use this tool proactively when:
1. Starting non-trivial implementation tasks
2. New feature implementation
3. Multiple valid approaches exist
4. Code modifications affect existing behavior
5. Architectural decisions needed
6. Multi-file changes expected
7. Unclear requirements need exploration

In plan mode, you can:
- Explore the codebase using Glob, Grep, Read tools
- Understand existing patterns and architecture
- Design an implementation approach
- Present the plan to user for approval
- Use AskUserQuestion for clarifications

Do NOT use for:
- Single-line or few-line fixes
- Simple, obvious implementations
- Pure research/exploration tasks`
}

func (t *EnterPlanModeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)
}

func (t *EnterPlanModeTool) Validate(input *tool.Input) error {
	return nil
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	if t.inPlan != nil && *t.inPlan {
		return &tool.Output{Content: "Already in plan mode", IsError: true}, nil
	}

	if t.callback != nil {
		if err := t.callback(true); err != nil {
			return &tool.Output{Content: fmt.Sprintf("Failed to enter plan mode: %v", err), IsError: true}, nil
		}
	}

	if t.inPlan != nil {
		*t.inPlan = true
	}

	return &tool.Output{
		Content: "Entered plan mode. You can now explore the codebase and design an implementation approach. Use ExitPlanMode when ready to begin implementation.",
		Metadata: map[string]interface{}{
			"mode": "plan",
		},
	}, nil
}

// ExitPlanModeTool transitions out of plan mode
type ExitPlanModeTool struct {
	callback PlanModeCallback
	inPlan   *bool
}

// NewExitPlanModeTool creates a new exit plan mode tool
func NewExitPlanModeTool(inPlan *bool, callback PlanModeCallback) *ExitPlanModeTool {
	return &ExitPlanModeTool{
		callback: callback,
		inPlan:   inPlan,
	}
}

func (t *ExitPlanModeTool) Name() string {
	return "ExitPlanMode"
}

func (t *ExitPlanModeTool) Description() string {
	return `Exit plan mode after finishing your implementation plan.

Use this tool when:
1. You have written your plan to the plan file
2. You are ready for user approval
3. The plan is clear and unambiguous

Before using this tool:
- Ensure your plan is clear and unambiguous
- If multiple valid approaches exist, use AskUserQuestion first
- Ask about specific implementation choices
- Clarify any assumptions
- Edit your plan file to incorporate user feedback

This tool requires user approval - they must consent to starting implementation.`
}

func (t *ExitPlanModeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)
}

func (t *ExitPlanModeTool) Validate(input *tool.Input) error {
	return nil
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	if t.inPlan != nil && !*t.inPlan {
		return &tool.Output{Content: "Not in plan mode", IsError: true}, nil
	}

	if t.callback != nil {
		if err := t.callback(false); err != nil {
			return &tool.Output{Content: fmt.Sprintf("Failed to exit plan mode: %v", err), IsError: true}, nil
		}
	}

	if t.inPlan != nil {
		*t.inPlan = false
	}

	return &tool.Output{
		Content: "Exited plan mode. Ready to begin implementation.",
		Metadata: map[string]interface{}{
			"mode": "implement",
		},
	}, nil
}
