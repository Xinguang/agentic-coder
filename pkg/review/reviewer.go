// Package review provides automatic review and self-correction for AI responses
package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

// ReviewResult contains the result of a review
type ReviewResult struct {
	Passed       bool   `json:"passed"`
	Issues       string `json:"issues"`
	Feedback     string `json:"feedback"`
	InputTokens  int    `json:"-"` // Token usage tracking
	OutputTokens int    `json:"-"`
}

// ReviewConfig defines what aspects to check during review
type ReviewConfig struct {
	CheckCompilation bool     // Check if code compiles/runs
	CheckSecurity    bool     // Check for security issues
	CheckPerformance bool     // Check for performance issues
	CheckStyle       bool     // Check code style
	CheckTests       bool     // Check if tests are included
	CustomCriteria   []string // Custom review criteria
	StrictMode       bool     // Be more strict in evaluation
}

// DefaultReviewConfig returns a balanced default configuration
func DefaultReviewConfig() *ReviewConfig {
	return &ReviewConfig{
		CheckCompilation: true,
		CheckSecurity:    false,
		CheckPerformance: false,
		CheckStyle:       false,
		CheckTests:       false,
		StrictMode:       false,
	}
}

// StrictReviewConfig returns a strict review configuration
func StrictReviewConfig() *ReviewConfig {
	return &ReviewConfig{
		CheckCompilation: true,
		CheckSecurity:    true,
		CheckPerformance: true,
		CheckStyle:       true,
		CheckTests:       true,
		StrictMode:       true,
	}
}

// Reviewer checks if AI responses meet user requirements
type Reviewer struct {
	provider provider.AIProvider
	model    string
	config   *ReviewConfig
}

// NewReviewer creates a new reviewer with default config
func NewReviewer(prov provider.AIProvider) *Reviewer {
	return &Reviewer{
		provider: prov,
		config:   DefaultReviewConfig(),
	}
}

// NewReviewerWithConfig creates a new reviewer with custom config
func NewReviewerWithConfig(prov provider.AIProvider, cfg *ReviewConfig) *Reviewer {
	if cfg == nil {
		cfg = DefaultReviewConfig()
	}
	return &Reviewer{
		provider: prov,
		config:   cfg,
	}
}

// SetConfig updates the review configuration
func (r *Reviewer) SetConfig(cfg *ReviewConfig) {
	if cfg != nil {
		r.config = cfg
	}
}

// buildReviewCriteria generates review criteria based on config
func (r *Reviewer) buildReviewCriteria() string {
	var criteria []string

	if r.config.CheckCompilation {
		criteria = append(criteria, "- Code compiles/runs without syntax errors")
	}
	if r.config.CheckSecurity {
		criteria = append(criteria, "- No security vulnerabilities (injection, XSS, etc.)")
	}
	if r.config.CheckPerformance {
		criteria = append(criteria, "- No obvious performance issues or inefficiencies")
	}
	if r.config.CheckStyle {
		criteria = append(criteria, "- Code follows consistent style and best practices")
	}
	if r.config.CheckTests {
		criteria = append(criteria, "- Tests are included or test strategy is mentioned")
	}

	// Always include these basic criteria
	criteria = append(criteria,
		"- All requested features are implemented",
		"- The implementation matches the user's intent",
	)

	// Add custom criteria
	for _, c := range r.config.CustomCriteria {
		criteria = append(criteria, "- "+c)
	}

	return strings.Join(criteria, "\n")
}

// getStrictnessNote returns guidance based on strictness level
func (r *Reviewer) getStrictnessNote() string {
	if r.config.StrictMode {
		return "Be STRICT in evaluation. Only mark as 'passed' if ALL criteria are fully met with no compromises."
	}
	return "Be strict but fair. Minor issues that don't affect functionality can be noted but shouldn't cause failure."
}

// Review checks if the AI response meets the user's requirements
func (r *Reviewer) Review(ctx context.Context, userRequest, aiResponse string) (*ReviewResult, error) {
	// Input validation
	userRequest = strings.TrimSpace(userRequest)
	aiResponse = strings.TrimSpace(aiResponse)

	if userRequest == "" {
		return &ReviewResult{
			Passed:   true, // Can't evaluate without request
			Issues:   "",
			Feedback: "No user request provided, skipping review",
		}, nil
	}

	if aiResponse == "" {
		return &ReviewResult{
			Passed:   false,
			Issues:   "Empty response",
			Feedback: "The AI response is empty, please provide a substantive response",
		}, nil
	}

	// Skip review for very short responses that are likely acknowledgments
	if len(aiResponse) < 20 {
		return &ReviewResult{
			Passed:   true,
			Issues:   "",
			Feedback: "Response too short for meaningful review",
		}, nil
	}

	// Build dynamic review criteria based on config
	criteria := r.buildReviewCriteria()

	prompt := fmt.Sprintf(`You are a code review assistant. Analyze if the AI's response adequately addresses the user's request.

## User's Request:
%s

## AI's Response:
%s

## Review Criteria:
%s

## Your Task:
1. Check if the AI's response fully addresses what the user asked for
2. Evaluate against each review criterion listed above
3. Identify any issues, bugs, or missing functionality
4. Determine if further modifications are needed

Respond in JSON format:
{
  "passed": true/false,
  "issues": "description of issues found, or empty if passed",
  "feedback": "specific instructions for fixing the issues, or empty if passed"
}

%s

Respond ONLY with the JSON, no other text.`, userRequest, truncateResponse(aiResponse, 8000), criteria, r.getStrictnessNote())

	// Create a simple message for the review
	messages := []provider.Message{
		{
			Role: "user",
			Content: []provider.ContentBlock{
				&provider.TextBlock{Text: prompt},
			},
		},
	}

	// Use streaming to get the response
	stream, err := r.provider.CreateMessageStream(ctx, &provider.Request{
		Messages:  messages,
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create review stream: %w", err)
	}
	defer stream.Close()

	var response strings.Builder
	var inputTokens, outputTokens int
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}

		switch e := event.(type) {
		case *provider.ContentBlockDeltaEvent:
			if delta, ok := e.Delta.(*provider.TextDelta); ok {
				response.WriteString(delta.Text)
			}
		case *provider.MessageDeltaEvent:
			if e.Usage != nil {
				outputTokens = e.Usage.OutputTokens
			}
		case *provider.MessageStartEvent:
			if e.Message != nil {
				inputTokens = e.Message.Usage.InputTokens
			}
		case *provider.MessageStopEvent:
			break
		}
	}

	// Parse the JSON response
	result := &ReviewResult{}
	responseText := response.String()

	// Try to extract JSON from the response
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonText := responseText[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonText), result); err != nil {
			// If parsing fails, return error with warning
			return &ReviewResult{
				Passed:       false,
				Issues:       "Review response parsing failed",
				Feedback:     fmt.Sprintf("Could not parse review response: %v. Raw response: %s", err, truncateResponse(responseText, 200)),
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
			}, nil
		}
	} else {
		// No JSON found, return warning
		return &ReviewResult{
			Passed:       false,
			Issues:       "Review response format invalid",
			Feedback:     fmt.Sprintf("Expected JSON response but got: %s", truncateResponse(responseText, 200)),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}, nil
	}

	// Add token usage to result
	result.InputTokens = inputTokens
	result.OutputTokens = outputTokens

	return result, nil
}

// GenerateCorrectionPrompt creates a prompt for the AI to fix issues
func (r *Reviewer) GenerateCorrectionPrompt(issues, feedback string) string {
	return fmt.Sprintf(`The previous response has issues that need to be fixed:

## Issues Found:
%s

## Required Fixes:
%s

Please fix these issues and provide the corrected implementation. Focus only on addressing the specific problems mentioned above.`, issues, feedback)
}

// truncateResponse limits the response length for review with smart summarization
func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Try to preserve code blocks and key information
	var result strings.Builder
	lines := strings.Split(s, "\n")
	inCodeBlock := false
	codeBlockContent := strings.Builder{}

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block - add if fits
				codeBlockContent.WriteString(line + "\n")
				if result.Len()+codeBlockContent.Len() < maxLen {
					result.WriteString(codeBlockContent.String())
				}
				codeBlockContent.Reset()
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
				codeBlockContent.WriteString(line + "\n")
			}
			continue
		}

		if inCodeBlock {
			codeBlockContent.WriteString(line + "\n")
		} else {
			// For non-code content, add if there's space
			if result.Len()+len(line)+1 < maxLen {
				result.WriteString(line + "\n")
			}
		}

		// Stop if we're at limit
		if result.Len() >= maxLen {
			break
		}
	}

	// Add truncation notice
	if result.Len() < len(s) {
		result.WriteString("\n... (truncated, original: ")
		result.WriteString(fmt.Sprintf("%d chars)", len(s)))
	}

	return result.String()
}
