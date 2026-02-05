package review

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// CheckType represents a type of review check
type CheckType string

const (
	CheckTypeSyntax      CheckType = "syntax"
	CheckTypeLogic       CheckType = "logic"
	CheckTypeSecurity    CheckType = "security"
	CheckTypePerformance CheckType = "performance"
	CheckTypeStyle       CheckType = "style"
	CheckTypeTests       CheckType = "tests"
)

// CheckResult represents the result of a single check
type CheckResult struct {
	CheckType   CheckType `json:"check_type"`
	Passed      bool      `json:"passed"`
	Score       int       `json:"score"` // 0-100
	Issues      []string  `json:"issues"`
	Suggestions []string  `json:"suggestions"`
}

// PipelineResult contains results from all pipeline stages
type PipelineResult struct {
	Checks       []CheckResult `json:"checks"`
	OverallScore int           `json:"overall_score"`
	Passed       bool          `json:"passed"`
	Summary      string        `json:"summary"`
}

// Pipeline represents a review pipeline with multiple stages
type Pipeline struct {
	stages   []PipelineStage
	parallel bool
	mu       sync.Mutex
}

// PipelineStage represents a single stage in the review pipeline
type PipelineStage struct {
	Name      string
	CheckType CheckType
	Check     func(ctx context.Context, code string) (*CheckResult, error)
	Required  bool // If true, pipeline stops on failure
}

// NewPipeline creates a new review pipeline
func NewPipeline(parallel bool) *Pipeline {
	return &Pipeline{
		stages:   make([]PipelineStage, 0),
		parallel: parallel,
	}
}

// AddStage adds a stage to the pipeline
func (p *Pipeline) AddStage(stage PipelineStage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stages = append(p.stages, stage)
}

// Run executes the pipeline
func (p *Pipeline) Run(ctx context.Context, code string) (*PipelineResult, error) {
	if p.parallel {
		return p.runParallel(ctx, code)
	}
	return p.runSequential(ctx, code)
}

// runSequential runs stages one by one
func (p *Pipeline) runSequential(ctx context.Context, code string) (*PipelineResult, error) {
	result := &PipelineResult{
		Checks: make([]CheckResult, 0, len(p.stages)),
		Passed: true,
	}

	totalScore := 0
	for _, stage := range p.stages {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		checkResult, err := stage.Check(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("stage %s failed: %w", stage.Name, err)
		}

		result.Checks = append(result.Checks, *checkResult)
		totalScore += checkResult.Score

		if !checkResult.Passed {
			result.Passed = false
			if stage.Required {
				// Stop pipeline on required stage failure
				break
			}
		}
	}

	if len(result.Checks) > 0 {
		result.OverallScore = totalScore / len(result.Checks)
	}

	result.Summary = p.generateSummary(result)
	return result, nil
}

// runParallel runs all stages in parallel
func (p *Pipeline) runParallel(ctx context.Context, code string) (*PipelineResult, error) {
	result := &PipelineResult{
		Checks: make([]CheckResult, len(p.stages)),
		Passed: true,
	}

	// Initialize all checks with default failed state to prevent nil/zero-value issues
	for i, stage := range p.stages {
		result.Checks[i] = CheckResult{
			CheckType: stage.CheckType,
			Passed:    false,
			Score:     0,
			Issues:    []string{"check not completed"},
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, stage := range p.stages {
		wg.Add(1)
		go func(idx int, s PipelineStage) {
			defer wg.Done()

			checkResult, err := s.Check(ctx, code)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("stage %s failed: %w", s.Name, err)
				}
				// Update the check with error info instead of leaving default
				result.Checks[idx] = CheckResult{
					CheckType: s.CheckType,
					Passed:    false,
					Score:     0,
					Issues:    []string{fmt.Sprintf("error: %v", err)},
				}
				result.Passed = false
				return
			}

			result.Checks[idx] = *checkResult
			if !checkResult.Passed {
				result.Passed = false
			}
		}(i, stage)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Calculate overall score
	totalScore := 0
	for _, check := range result.Checks {
		totalScore += check.Score
	}
	if len(result.Checks) > 0 {
		result.OverallScore = totalScore / len(result.Checks)
	}

	result.Summary = p.generateSummary(result)
	return result, nil
}

// generateSummary creates a human-readable summary
func (p *Pipeline) generateSummary(result *PipelineResult) string {
	var sb strings.Builder

	passedCount := 0
	for _, check := range result.Checks {
		if check.Passed {
			passedCount++
		}
	}

	sb.WriteString(fmt.Sprintf("Review complete: %d/%d checks passed (score: %d/100)\n",
		passedCount, len(result.Checks), result.OverallScore))

	for _, check := range result.Checks {
		status := "✓"
		if !check.Passed {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %d/100\n", status, check.CheckType, check.Score))

		for _, issue := range check.Issues {
			sb.WriteString(fmt.Sprintf("    - %s\n", issue))
		}
	}

	return sb.String()
}

// CreateDefaultPipeline creates a pipeline with default checks
func CreateDefaultPipeline(reviewer *Reviewer, parallel bool) *Pipeline {
	p := NewPipeline(parallel)

	// Use real syntax checker
	p.AddStage(CreateSyntaxCheckStage())

	// Logic validation - uses heuristics
	p.AddStage(PipelineStage{
		Name:      "Logic Validation",
		CheckType: CheckTypeLogic,
		Required:  false,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			issues := []string{}
			suggestions := []string{}

			// Extract code blocks for analysis
			blocks := ExtractCodeBlocks(code)

			for _, block := range blocks {
				// Check for common logic issues
				content := block.Content

				// Empty function bodies
				if strings.Contains(content, "{}") && !strings.Contains(content, "interface{}") {
					suggestions = append(suggestions, "Consider adding implementation for empty code blocks")
				}

				// TODO/FIXME comments
				if strings.Contains(strings.ToUpper(content), "TODO") || strings.Contains(strings.ToUpper(content), "FIXME") {
					issues = append(issues, "Code contains TODO/FIXME comments that need attention")
				}

				// Panic calls
				if strings.Contains(content, "panic(") {
					suggestions = append(suggestions, "Consider proper error handling instead of panic")
				}

				// Hardcoded credentials patterns
				if strings.Contains(strings.ToLower(content), "password") && strings.Contains(content, "=") {
					issues = append(issues, "Possible hardcoded password detected")
				}
			}

			score := 100
			if len(issues) > 0 {
				score = 60
			} else if len(suggestions) > 0 {
				score = 85
			}

			return &CheckResult{
				CheckType:   CheckTypeLogic,
				Passed:      len(issues) == 0,
				Score:       score,
				Issues:      issues,
				Suggestions: suggestions,
			}, nil
		},
	})

	return p
}
