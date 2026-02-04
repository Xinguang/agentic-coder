// Package builtin provides built-in tools
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/skill"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// SkillTool executes skills (slash commands)
type SkillTool struct {
	manager *skill.Manager
}

// NewSkillTool creates a new skill execution tool
func NewSkillTool(manager *skill.Manager) *SkillTool {
	return &SkillTool{
		manager: manager,
	}
}

func (t *SkillTool) Name() string {
	return "Skill"
}

func (t *SkillTool) Description() string {
	skills := t.getAvailableSkills()
	return fmt.Sprintf(`Execute a skill (slash command) within the conversation.

Usage:
- Use skill name and optional arguments to invoke a skill
- Skills provide specialized capabilities for common tasks

Available skills:
%s

Example:
- skill: "commit" - invoke the commit skill
- skill: "review-pr", args: "123" - invoke with arguments`, skills)
}

func (t *SkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"skill": {
				"type": "string",
				"description": "The skill name to execute (e.g., 'commit', 'review-pr')"
			},
			"args": {
				"type": "string",
				"description": "Optional arguments for the skill"
			}
		},
		"required": ["skill"]
	}`)
}

type skillToolParams struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

func (t *SkillTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[skillToolParams](input.Params)
	if err != nil {
		return err
	}

	if params.Skill == "" {
		return fmt.Errorf("skill name is required")
	}

	return nil
}

func (t *SkillTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[skillToolParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	if t.manager == nil {
		return &tool.Output{Content: "Skill manager not configured", IsError: true}, nil
	}

	// Find the skill
	sk := t.manager.Get(params.Skill)
	if sk == nil {
		// Try with plugin prefix
		available := t.manager.List()
		for _, s := range available {
			if strings.HasSuffix(s.Name, ":"+params.Skill) {
				sk = s
				break
			}
		}
	}

	if sk == nil {
		return &tool.Output{
			Content: fmt.Sprintf("Skill '%s' not found. Use /help to see available skills.", params.Skill),
			IsError: true,
		}, nil
	}

	// Execute the skill
	result, err := t.manager.Execute(ctx, sk.Name, params.Args)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Skill execution error: %v", err), IsError: true}, nil
	}

	return &tool.Output{
		Content: result,
		Metadata: map[string]interface{}{
			"skill":       params.Skill,
			"args":        params.Args,
			"description": sk.Description,
		},
	}, nil
}

// getAvailableSkills returns formatted list of available skills
func (t *SkillTool) getAvailableSkills() string {
	if t.manager == nil {
		return "- No skills configured"
	}

	skills := t.manager.List()
	if len(skills) == 0 {
		return "- No skills available"
	}

	var sb strings.Builder
	for _, sk := range skills {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", sk.Name, sk.Description))
	}
	return sb.String()
}
