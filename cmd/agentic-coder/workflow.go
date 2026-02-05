package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/xinguang/agentic-coder/pkg/auth"
	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/provider/claude"
	"github.com/xinguang/agentic-coder/pkg/tool"
	"github.com/xinguang/agentic-coder/pkg/ui"
	"github.com/xinguang/agentic-coder/pkg/workflow"
	"github.com/xinguang/agentic-coder/pkg/workflow/agent"
)

func workflowCmd() *cobra.Command {
	var (
		maxExecutors  int
		maxReviewers  int
		maxRetries    int
		enableAutoFix bool
		managerModel  string
		executorModel string
		reviewerModel string
		fixerModel    string
		evalModel     string
		defaultModel  string
	)

	cmd := &cobra.Command{
		Use:   "workflow <requirement>",
		Short: "Run multi-agent workflow for complex tasks",
		Long: `Execute a multi-agent workflow that automatically plans, executes,
reviews, and evaluates tasks to fulfill your requirement.

The workflow uses multiple AI agents:
  - Manager: Analyzes requirements and creates task plans
  - Executors: Execute individual tasks concurrently
  - Reviewers: Review task execution quality
  - Fixers: Auto-fix issues found during review
  - Evaluator: Evaluate overall result quality

Example:
  agentic-coder workflow "Add user authentication with JWT"
  agentic-coder workflow --max-executors 10 "Refactor the codebase"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requirement := strings.Join(args, " ")
			printer := ui.NewPrinter()

			// Build config
			config := &workflow.WorkflowConfig{
				MaxExecutors:  maxExecutors,
				MaxReviewers:  maxReviewers,
				MaxFixers:     maxReviewers, // Same as reviewers
				MaxRetries:    maxRetries,
				EnableAutoFix: enableAutoFix,
				Models: workflow.RoleModels{
					Default:   defaultModel,
					Manager:   managerModel,
					Executor:  executorModel,
					Reviewer:  reviewerModel,
					Fixer:     fixerModel,
					Evaluator: evalModel,
				},
			}
			config.Validate()

			return runWorkflow(cmd.Context(), requirement, config, printer)
		},
	}

	// Concurrency flags
	cmd.Flags().IntVar(&maxExecutors, "max-executors", 5, "Maximum concurrent executors")
	cmd.Flags().IntVar(&maxReviewers, "max-reviewers", 2, "Maximum concurrent reviewers")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 3, "Maximum retries per task")
	cmd.Flags().BoolVar(&enableAutoFix, "auto-fix", true, "Enable auto-fix for minor issues")

	// Model flags
	cmd.Flags().StringVar(&defaultModel, "model", "sonnet", "Default model for all roles")
	cmd.Flags().StringVar(&managerModel, "manager-model", "", "Model for manager (default: use --model)")
	cmd.Flags().StringVar(&executorModel, "executor-model", "", "Model for executors (default: use --model)")
	cmd.Flags().StringVar(&reviewerModel, "reviewer-model", "", "Model for reviewers (default: use --model)")
	cmd.Flags().StringVar(&fixerModel, "fixer-model", "", "Model for fixers (default: use --model)")
	cmd.Flags().StringVar(&evalModel, "evaluator-model", "", "Model for evaluator (default: use --model)")

	return cmd
}

func runWorkflow(ctx context.Context, requirement string, config *workflow.WorkflowConfig, printer *ui.Printer) error {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		printer.Warning("Interrupted, cancelling workflow...")
		cancel()
	}()

	// Print workflow start
	printer.Info("🚀 Starting Multi-Agent Workflow")
	printer.Dim("Requirement: %s", requirement)
	printer.Dim("Max executors: %d, Max retries: %d", config.MaxExecutors, config.MaxRetries)
	fmt.Println()

	// Create provider factory
	provFactory, err := createWorkflowProviderFactory(printer)
	if err != nil {
		return err
	}

	// Create engine factory
	cwd, _ := os.Getwd()
	registry := tool.NewRegistry()
	registerBuiltinTools(registry)

	engFactory := func() *engine.Engine {
		prov, _ := provFactory(config.Models.Default)
		return engine.NewEngine(&engine.EngineOptions{
			Provider:      prov,
			Registry:      registry,
			MaxIterations: 50,
			MaxTokens:     8192,
			SystemPrompt:  getSystemPrompt(),
		})
	}

	// Adapt provider factory to use workflow's expected signature
	agentProvFactory := func(model string) provider.AIProvider {
		prov, _ := provFactory(model)
		return prov
	}

	// Create workflow
	wf := workflow.NewWorkflow(config, agentProvFactory, engFactory)

	// Setup progress callback
	wf.SetProgressCallback(func(event *workflow.ProgressEvent) {
		printProgressEvent(event, printer)
	})

	// Run workflow
	startTime := time.Now()
	report, err := wf.Run(ctx, requirement)
	duration := time.Since(startTime)

	if err != nil {
		printer.Error("Workflow failed: %v", err)
		return err
	}

	// Print final report
	fmt.Println()
	printFinalReport(report, duration, printer, cwd)

	return nil
}

func createWorkflowProviderFactory(printer *ui.Printer) (func(model string) (provider.AIProvider, error), error) {
	authMgr := auth.NewManager("")

	return func(model string) (provider.AIProvider, error) {
		// Detect provider type from model
		providerType := provider.DetectProviderFromModel(model)

		switch providerType {
		case provider.ProviderTypeClaude:
			// Try auth manager first
			if creds, err := authMgr.GetCredentials(auth.ProviderClaude); err == nil && creds.APIKey != "" {
				return claude.New(creds.APIKey, claude.WithBeta("interleaved-thinking-2025-05-14")), nil
			}

			// Try env var
			key := os.Getenv("ANTHROPIC_API_KEY")
			if key == "" {
				return nil, fmt.Errorf("no API key for Claude. Set ANTHROPIC_API_KEY or run 'agentic-coder auth login claude'")
			}
			return claude.New(key, claude.WithBeta("interleaved-thinking-2025-05-14")), nil

		default:
			return nil, fmt.Errorf("unsupported model for workflow: %s", model)
		}
	}, nil
}

func printProgressEvent(event *workflow.ProgressEvent, printer *ui.Printer) {
	switch event.Type {
	case "analyzing":
		printer.Info("📋 %s", event.Message)
	case "plan_created":
		printer.Success("✅ %s", event.Message)
	case "task_started":
		printer.Info("▶️  [%s] %s - %s", event.TaskID, event.TaskTitle, event.Message)
	case "reviewing":
		printer.Dim("🔍 [%s] Reviewing...", event.TaskID)
	case "fixing":
		printer.Dim("🔧 [%s] Auto-fixing: %s", event.TaskID, event.Message)
	case "task_completed":
		printer.Success("✅ [%s] %s - %s", event.TaskID, event.TaskTitle, event.Message)
	case "task_failed":
		printer.Error("❌ [%s] %s - %s", event.TaskID, event.TaskTitle, event.Message)
	case "review_failed":
		printer.Warning("⚠️  [%s] Review failed: %s", event.TaskID, event.Message)
	case "evaluating":
		printer.Info("📊 %s", event.Message)
	case "reporting":
		printer.Info("📝 %s", event.Message)
	case "completed":
		printer.Success("🎉 %s", event.Message)
	default:
		printer.Dim("   %s: %s", event.Type, event.Message)
	}
}

func printFinalReport(report *agent.FinalReport, duration time.Duration, printer *ui.Printer, cwd string) {
	// Header
	printer.Info("═══════════════════════════════════════════════════════════")
	printer.Info("                    WORKFLOW REPORT                        ")
	printer.Info("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// Status
	statusIcon := "✅"
	if report.Status == "partial" {
		statusIcon = "⚠️"
	} else if report.Status == "failed" {
		statusIcon = "❌"
	}
	printer.Info("%s Status: %s", statusIcon, strings.ToUpper(report.Status))
	fmt.Println()

	// Statistics
	printer.Info("📊 Statistics")
	printer.Dim("   Total tasks:    %d", report.TotalTasks)
	printer.Dim("   Completed:      %d", report.Completed)
	printer.Dim("   Failed:         %d", report.Failed)
	printer.Dim("   Total retries:  %d", report.TotalRetries)
	printer.Dim("   Duration:       %s", duration.Round(time.Second))
	fmt.Println()

	// Task summaries
	printer.Info("📋 Task Summaries")
	for _, ts := range report.TaskSummaries {
		icon := "⬜"
		switch ts.Status {
		case agent.TaskStatusCompleted:
			icon = "✅"
		case agent.TaskStatusFailed:
			icon = "❌"
		case agent.TaskStatusCancelled:
			icon = "⏹️"
		}
		printer.Dim("   %s [%s] %s", icon, ts.TaskID, ts.Title)
	}
	fmt.Println()

	// Evaluation
	if report.Evaluation != nil {
		eval := report.Evaluation
		printer.Info("📊 Evaluation")
		meetsReq := "No"
		if eval.MeetsRequirement {
			meetsReq = "Yes"
		}
		printer.Dim("   Meets requirement: %s", meetsReq)
		printer.Dim("   Quality score:     %d/100", eval.QualityScore)

		if len(eval.Strengths) > 0 {
			printer.Dim("   Strengths:")
			for _, s := range eval.Strengths {
				printer.Dim("     + %s", s)
			}
		}

		if len(eval.Weaknesses) > 0 {
			printer.Dim("   Weaknesses:")
			for _, w := range eval.Weaknesses {
				printer.Dim("     - %s", w)
			}
		}

		if len(eval.Suggestions) > 0 {
			printer.Dim("   Suggestions:")
			for _, s := range eval.Suggestions {
				printer.Dim("     → %s", s)
			}
		}
		fmt.Println()
	}

	// Conclusion
	printer.Info("📝 Conclusion")
	// Wrap conclusion text
	words := strings.Fields(report.Conclusion)
	var line string
	for _, word := range words {
		if len(line)+len(word)+1 > 60 {
			printer.Dim("   %s", line)
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		printer.Dim("   %s", line)
	}
	fmt.Println()

	// Next steps
	if len(report.NextSteps) > 0 {
		printer.Info("🔜 Next Steps")
		for i, step := range report.NextSteps {
			printer.Dim("   %d. %s", i+1, step)
		}
		fmt.Println()
	}

	printer.Info("═══════════════════════════════════════════════════════════")
}
