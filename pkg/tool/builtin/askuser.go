package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// AskUserQuestionTool asks the user questions
type AskUserQuestionTool struct {
	askFunc func(questions []Question) (map[string]string, error)
}

// Question represents a question to ask the user
type Question struct {
	Question    string         `json:"question"`
	Header      string         `json:"header"`
	Options     []QuestionOption `json:"options"`
	MultiSelect bool           `json:"multiSelect"`
}

// QuestionOption represents an option for a question
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// AskUserQuestionInput represents the input for AskUserQuestion tool
type AskUserQuestionInput struct {
	Questions []Question `json:"questions"`
}

// NewAskUserQuestionTool creates a new AskUserQuestion tool
func NewAskUserQuestionTool(askFunc func([]Question) (map[string]string, error)) *AskUserQuestionTool {
	return &AskUserQuestionTool{askFunc: askFunc}
}

func (a *AskUserQuestionTool) Name() string {
	return "AskUserQuestion"
}

func (a *AskUserQuestionTool) Description() string {
	return `Ask the user questions to gather preferences, clarify instructions, or get decisions.

Use when you need to:
1. Gather user preferences
2. Clarify ambiguous instructions
3. Get decisions on implementation choices

Usage notes:
- Users can always select "Other" for custom input
- Use multiSelect: true for multiple answers
- Max 4 questions, each with 2-4 options`
}

func (a *AskUserQuestionTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"questions": {
				"type": "array",
				"description": "Questions to ask (1-4)",
				"minItems": 1,
				"maxItems": 4,
				"items": {
					"type": "object",
					"properties": {
						"question": {
							"type": "string",
							"description": "The question to ask"
						},
						"header": {
							"type": "string",
							"description": "Short label (max 12 chars)"
						},
						"multiSelect": {
							"type": "boolean",
							"description": "Allow multiple selections"
						},
						"options": {
							"type": "array",
							"minItems": 2,
							"maxItems": 4,
							"items": {
								"type": "object",
								"properties": {
									"label": {
										"type": "string",
										"description": "Option display text"
									},
									"description": {
										"type": "string",
										"description": "Option explanation"
									}
								},
								"required": ["label", "description"]
							}
						}
					},
					"required": ["question", "header", "options", "multiSelect"]
				}
			}
		},
		"required": ["questions"]
	}`)
}

func (a *AskUserQuestionTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[AskUserQuestionInput](input.Params)
	if err != nil {
		return err
	}

	if len(params.Questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}

	if len(params.Questions) > 4 {
		return fmt.Errorf("maximum 4 questions allowed")
	}

	for i, q := range params.Questions {
		if q.Question == "" {
			return fmt.Errorf("questions[%d].question is required", i)
		}
		if q.Header == "" {
			return fmt.Errorf("questions[%d].header is required", i)
		}
		if len(q.Header) > 12 {
			return fmt.Errorf("questions[%d].header must be max 12 characters", i)
		}
		if len(q.Options) < 2 {
			return fmt.Errorf("questions[%d] must have at least 2 options", i)
		}
		if len(q.Options) > 4 {
			return fmt.Errorf("questions[%d] must have at most 4 options", i)
		}
	}

	return nil
}

func (a *AskUserQuestionTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[AskUserQuestionInput](input.Params)
	if err != nil {
		return nil, err
	}

	// If no ask function, format as text prompt
	if a.askFunc == nil {
		return a.formatAsTextPrompt(params.Questions)
	}

	// Call the ask function
	answers, err := a.askFunc(params.Questions)
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Failed to get user input: %v", err),
			IsError: true,
		}, nil
	}

	// Format answers
	var sb strings.Builder
	sb.WriteString("User answers:\n")
	for header, answer := range answers {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", header, answer))
	}

	return &tool.Output{
		Content: sb.String(),
		Metadata: map[string]interface{}{
			"answers": answers,
		},
	}, nil
}

// formatAsTextPrompt formats questions as a text prompt when no UI is available
func (a *AskUserQuestionTool) formatAsTextPrompt(questions []Question) (*tool.Output, error) {
	var sb strings.Builder
	sb.WriteString("Questions for user:\n\n")

	for i, q := range questions {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, q.Header, q.Question))
		for j, opt := range q.Options {
			sb.WriteString(fmt.Sprintf("   %c. %s - %s\n", 'a'+rune(j), opt.Label, opt.Description))
		}
		if q.MultiSelect {
			sb.WriteString("   (Multiple selections allowed)\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Please respond with your choices.")

	return &tool.Output{
		Content: sb.String(),
		Metadata: map[string]interface{}{
			"awaiting_response": true,
			"questions":         questions,
		},
	}, nil
}
