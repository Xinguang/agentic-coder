// Package agent implements workflow agents
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

// Agent is the interface for all workflow agents
type Agent interface {
	Role() Role
	Model() string
}

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	role     Role
	model    string
	provider provider.AIProvider
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(role Role, model string, prov provider.AIProvider) *BaseAgent {
	return &BaseAgent{
		role:     role,
		model:    model,
		provider: prov,
	}
}

// Role returns the agent's role
func (a *BaseAgent) Role() Role {
	return a.role
}

// Model returns the agent's model
func (a *BaseAgent) Model() string {
	return a.model
}

// Provider returns the agent's AI provider
func (a *BaseAgent) Provider() provider.AIProvider {
	return a.provider
}

// Chat sends a message to the AI and returns the response
func (a *BaseAgent) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	req := &provider.Request{
		Model:     a.model,
		MaxTokens: 8192,
		Messages: []provider.Message{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentBlock{
					&provider.TextBlock{Text: userMessage},
				},
			},
		},
		System: []provider.ContentBlock{
			&provider.TextBlock{Text: systemPrompt},
		},
	}

	resp, err := a.provider.CreateMessage(ctx, req)
	if err != nil {
		return "", fmt.Errorf("chat failed: %w", err)
	}

	var result strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.(*provider.TextBlock); ok {
			result.WriteString(tb.Text)
		}
	}

	return result.String(), nil
}

// ChatJSON sends a message and parses JSON response
func (a *BaseAgent) ChatJSON(ctx context.Context, systemPrompt, userMessage string, result interface{}) error {
	resp, err := a.Chat(ctx, systemPrompt, userMessage)
	if err != nil {
		return err
	}

	// Extract JSON from response (may be wrapped in markdown code blocks)
	jsonStr := extractJSON(resp)

	if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w\nResponse: %s", err, resp)
	}

	return nil
}

// extractJSON extracts JSON from a response that may contain markdown
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Try to find JSON in code blocks
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(s[start:], "```")
		if end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}

	// Try to find JSON in generic code blocks
	if idx := strings.Index(s, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier if present
		if newline := strings.Index(s[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		end := strings.Index(s[start:], "```")
		if end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}

	// Try to find raw JSON object or array
	if idx := strings.IndexAny(s, "{["); idx != -1 {
		// Find matching closing bracket
		depth := 0
		start := idx
		inString := false
		escape := false

		for i := start; i < len(s); i++ {
			c := s[i]

			if escape {
				escape = false
				continue
			}

			if c == '\\' && inString {
				escape = true
				continue
			}

			if c == '"' {
				inString = !inString
				continue
			}

			if inString {
				continue
			}

			if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	return s
}

// ProviderFactory creates a provider for a given model
type ProviderFactory func(model string) provider.AIProvider
