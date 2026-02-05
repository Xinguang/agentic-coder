package review

import (
	"context"
	"errors"
	"testing"
)

func TestNewPipeline(t *testing.T) {
	p := NewPipeline(false)
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if p.parallel {
		t.Error("expected sequential pipeline")
	}
}

func TestPipeline_AddStage(t *testing.T) {
	p := NewPipeline(false)

	p.AddStage(PipelineStage{
		Name:      "test",
		CheckType: CheckTypeSyntax,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{Passed: true, Score: 100}, nil
		},
	})

	if len(p.stages) != 1 {
		t.Errorf("expected 1 stage, got %d", len(p.stages))
	}
}

func TestPipeline_RunSequential(t *testing.T) {
	p := NewPipeline(false)

	p.AddStage(PipelineStage{
		Name:      "stage1",
		CheckType: CheckTypeSyntax,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{CheckType: CheckTypeSyntax, Passed: true, Score: 100}, nil
		},
	})

	p.AddStage(PipelineStage{
		Name:      "stage2",
		CheckType: CheckTypeLogic,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{CheckType: CheckTypeLogic, Passed: true, Score: 80}, nil
		},
	})

	result, err := p.Run(context.Background(), "test code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Error("expected pipeline to pass")
	}

	if len(result.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(result.Checks))
	}

	// Average of 100 and 80
	if result.OverallScore != 90 {
		t.Errorf("expected score 90, got %d", result.OverallScore)
	}
}

func TestPipeline_RunParallel(t *testing.T) {
	p := NewPipeline(true)

	p.AddStage(PipelineStage{
		Name:      "stage1",
		CheckType: CheckTypeSyntax,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{CheckType: CheckTypeSyntax, Passed: true, Score: 100}, nil
		},
	})

	p.AddStage(PipelineStage{
		Name:      "stage2",
		CheckType: CheckTypeLogic,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{CheckType: CheckTypeLogic, Passed: true, Score: 80}, nil
		},
	})

	result, err := p.Run(context.Background(), "test code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Error("expected pipeline to pass")
	}

	if len(result.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(result.Checks))
	}
}

func TestPipeline_RequiredStageFailure(t *testing.T) {
	p := NewPipeline(false)

	p.AddStage(PipelineStage{
		Name:      "required",
		CheckType: CheckTypeSyntax,
		Required:  true,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{Passed: false, Score: 0}, nil
		},
	})

	p.AddStage(PipelineStage{
		Name:      "skipped",
		CheckType: CheckTypeLogic,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{Passed: true, Score: 100}, nil
		},
	})

	result, err := p.Run(context.Background(), "test code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected pipeline to fail")
	}

	// Should only have 1 check since required stage failed
	if len(result.Checks) != 1 {
		t.Errorf("expected 1 check (stopped early), got %d", len(result.Checks))
	}
}

func TestPipeline_StageError(t *testing.T) {
	p := NewPipeline(false)

	p.AddStage(PipelineStage{
		Name:      "error",
		CheckType: CheckTypeSyntax,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return nil, errors.New("stage failed")
		},
	})

	_, err := p.Run(context.Background(), "test code")
	if err == nil {
		t.Error("expected error from stage")
	}
}

func TestPipeline_ContextCancellation(t *testing.T) {
	p := NewPipeline(false)

	p.AddStage(PipelineStage{
		Name:      "slow",
		CheckType: CheckTypeSyntax,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			return &CheckResult{Passed: true, Score: 100}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Run(ctx, "test code")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestCreateDefaultPipeline(t *testing.T) {
	p := CreateDefaultPipeline(nil, false)
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}

	if len(p.stages) < 2 {
		t.Errorf("expected at least 2 default stages, got %d", len(p.stages))
	}
}
