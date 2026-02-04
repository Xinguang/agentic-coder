// agentic-coder - An AI-powered coding assistant
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/xinguang/agentic-coder/pkg/auth"
	"github.com/xinguang/agentic-coder/pkg/engine"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/provider/claude"
	"github.com/xinguang/agentic-coder/pkg/provider/claudecli"
	"github.com/xinguang/agentic-coder/pkg/provider/codexcli"
	"github.com/xinguang/agentic-coder/pkg/provider/deepseek"
	"github.com/xinguang/agentic-coder/pkg/provider/gemini"
	"github.com/xinguang/agentic-coder/pkg/provider/geminicli"
	"github.com/xinguang/agentic-coder/pkg/provider/ollama"
	"github.com/xinguang/agentic-coder/pkg/provider/openai"
	"github.com/xinguang/agentic-coder/pkg/session"
	"github.com/xinguang/agentic-coder/pkg/tool"
	"github.com/xinguang/agentic-coder/pkg/tool/builtin"
	"github.com/xinguang/agentic-coder/pkg/tui"
	"github.com/xinguang/agentic-coder/pkg/ui"
	"github.com/xinguang/agentic-coder/pkg/workctx"
)

var (
	version = "0.1.0"
	model   string
	apiKey  string
	verbose bool
	useTUI  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "agentic-coder",
		Short: "AI-powered coding assistant",
		Long: `agentic-coder is an AI-powered coding assistant that helps you
write, edit, and understand code using natural language.`,
		RunE: runChat,
	}

	// Flags
	rootCmd.PersistentFlags().StringVarP(&model, "model", "m", "sonnet", "Model: sonnet/opus/haiku, gemini, gpt4o, llama3.2/qwen (Ollama)")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key (defaults to ANTHROPIC_API_KEY env var)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&useTUI, "tui", "t", true, "Enable interactive TUI mode (default: true)")
	rootCmd.PersistentFlags().Bool("no-tui", false, "Disable TUI mode, use classic mode")

	// Subcommands
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(authCmd())
	rootCmd.AddCommand(workCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("agentic-coder version %s\n", version)
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Configuration management")
		},
	}
}

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	// Login subcommand
	loginCmd := &cobra.Command{
		Use:   "login [provider]",
		Short: "Authenticate with a provider (claude, gemini, openai)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := "claude"
			if len(args) > 0 {
				providerName = args[0]
			}

			authMgr := auth.NewManager("")
			ctx := context.Background()

			var provider auth.Provider
			switch strings.ToLower(providerName) {
			case "claude", "anthropic":
				provider = auth.ProviderClaude
			case "gemini", "google":
				provider = auth.ProviderGemini
			case "openai":
				provider = auth.ProviderOpenAI
			default:
				return fmt.Errorf("unknown provider: %s", providerName)
			}

			_, err := authMgr.Authenticate(ctx, provider)
			return err
		},
	}

	// Logout subcommand
	logoutCmd := &cobra.Command{
		Use:   "logout [provider]",
		Short: "Remove authentication for a provider",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := "claude"
			if len(args) > 0 {
				providerName = args[0]
			}

			authMgr := auth.NewManager("")

			var provider auth.Provider
			switch strings.ToLower(providerName) {
			case "claude", "anthropic":
				provider = auth.ProviderClaude
			case "gemini", "google":
				provider = auth.ProviderGemini
			case "openai":
				provider = auth.ProviderOpenAI
			default:
				return fmt.Errorf("unknown provider: %s", providerName)
			}

			authMgr.Logout(provider)
			fmt.Printf("✅ Logged out from %s\n", provider)
			return nil
		},
	}

	// Status subcommand
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Run: func(cmd *cobra.Command, args []string) {
			authMgr := auth.NewManager("")
			providers := authMgr.ListProviders()

			fmt.Println("\n🔐 Authentication Status")
			fmt.Println("========================")

			if len(providers) == 0 {
				fmt.Println("No authenticated providers.")
				fmt.Println("\nRun 'agentic-coder auth login <provider>' to authenticate.")
				return
			}

			for _, p := range providers {
				creds, err := authMgr.GetCredentials(p)
				if err != nil {
					continue
				}

				status := "✅"
				authType := string(creds.AuthType)
				extra := ""

				if creds.IsExpired() {
					status = "⚠️ (expired)"
				}

				if creds.Email != "" {
					extra = fmt.Sprintf(" (%s)", creds.Email)
				}

				fmt.Printf("  %s %s: %s%s\n", status, p, authType, extra)
			}
		},
	}

	cmd.AddCommand(loginCmd)
	cmd.AddCommand(logoutCmd)
	cmd.AddCommand(statusCmd)

	return cmd
}

func workCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage work context for task continuity",
	}

	// New work context
	newCmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new work context",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			goal, _ := cmd.Flags().GetString("goal")

			mgr := workctx.NewManager("")
			ctx := mgr.New(title, goal)

			if err := mgr.Save(ctx); err != nil {
				return err
			}

			fmt.Printf("✅ Created work context: %s\n", ctx.ID)
			fmt.Printf("   Title: %s\n", ctx.Title)
			if ctx.Goal != "" {
				fmt.Printf("   Goal: %s\n", ctx.Goal)
			}
			fmt.Println("\nUse 'agentic-coder work update' to add progress, pending items, and notes.")
			return nil
		},
	}
	newCmd.Flags().StringP("goal", "g", "", "Goal/objective for this work")

	// List work contexts
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all saved work contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := workctx.NewManager("")
			contexts, err := mgr.List()
			if err != nil {
				return err
			}

			if len(contexts) == 0 {
				fmt.Println("No work contexts found.")
				fmt.Println("Use 'agentic-coder work new <title>' to create one.")
				return nil
			}

			fmt.Println("\n📋 Work Contexts")
			fmt.Println("================")
			for _, ctx := range contexts {
				fmt.Printf("  %s\n", ctx.Summary())
			}
			return nil
		},
	}

	// Show work context
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a work context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := workctx.NewManager("")
			ctx, err := mgr.Load(args[0])
			if err != nil {
				return fmt.Errorf("work context not found: %s", args[0])
			}

			lang, _ := cmd.Flags().GetString("lang")
			if lang == "cn" || lang == "zh" {
				fmt.Println(ctx.GenerateHandoffCN())
			} else {
				fmt.Println(ctx.GenerateHandoff())
			}
			return nil
		},
	}
	showCmd.Flags().StringP("lang", "l", "en", "Language: en or cn")

	// Update work context
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a work context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := workctx.NewManager("")
			ctx, err := mgr.Load(args[0])
			if err != nil {
				return fmt.Errorf("work context not found: %s", args[0])
			}

			// Update fields
			if goal, _ := cmd.Flags().GetString("goal"); goal != "" {
				ctx.Goal = goal
			}
			if bg, _ := cmd.Flags().GetString("background"); bg != "" {
				ctx.Background = bg
			}
			if done, _ := cmd.Flags().GetString("done"); done != "" {
				ctx.AddProgress(done)
			}
			if pending, _ := cmd.Flags().GetString("pending"); pending != "" {
				ctx.AddPending(pending)
			}
			if file, _ := cmd.Flags().GetString("file"); file != "" {
				ctx.AddKeyFile(file)
			}
			if note, _ := cmd.Flags().GetString("note"); note != "" {
				ctx.AddNote(note)
			}
			if prov, _ := cmd.Flags().GetString("provider"); prov != "" {
				ctx.Provider = prov
			}
			if mdl, _ := cmd.Flags().GetString("model"); mdl != "" {
				ctx.Model = mdl
			}

			if err := mgr.Save(ctx); err != nil {
				return err
			}

			fmt.Printf("✅ Updated work context: %s\n", ctx.ID)
			return nil
		},
	}
	updateCmd.Flags().StringP("goal", "g", "", "Update goal")
	updateCmd.Flags().StringP("background", "b", "", "Update background")
	updateCmd.Flags().StringP("done", "d", "", "Add completed item")
	updateCmd.Flags().StringP("pending", "p", "", "Add pending item")
	updateCmd.Flags().StringP("file", "f", "", "Add key file")
	updateCmd.Flags().StringP("note", "n", "", "Add note")
	updateCmd.Flags().String("provider", "", "Set last used provider")
	updateCmd.Flags().String("model", "", "Set last used model")

	// Handoff - generate handoff summary
	handoffCmd := &cobra.Command{
		Use:   "handoff <id>",
		Short: "Generate a handoff summary for switching providers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := workctx.NewManager("")
			ctx, err := mgr.Load(args[0])
			if err != nil {
				return fmt.Errorf("work context not found: %s", args[0])
			}

			lang, _ := cmd.Flags().GetString("lang")
			output, _ := cmd.Flags().GetString("output")

			var content string
			if lang == "cn" || lang == "zh" {
				content = ctx.GenerateHandoffCN()
			} else {
				content = ctx.GenerateHandoff()
			}

			if output != "" {
				if err := os.WriteFile(output, []byte(content), 0644); err != nil {
					return err
				}
				fmt.Printf("✅ Handoff saved to: %s\n", output)
			} else {
				fmt.Println(content)
			}
			return nil
		},
	}
	handoffCmd.Flags().StringP("lang", "l", "en", "Language: en or cn")
	handoffCmd.Flags().StringP("output", "o", "", "Output file path")

	// Delete work context
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a work context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := workctx.NewManager("")
			if err := mgr.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("✅ Deleted work context: %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(newCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(showCmd)
	cmd.AddCommand(updateCmd)
	cmd.AddCommand(handoffCmd)
	cmd.AddCommand(deleteCmd)

	return cmd
}

func runChat(cmd *cobra.Command, args []string) error {
	// Create UI printer
	printer := ui.NewPrinter()

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Detect provider from model
	providerType := provider.DetectProviderFromModel(model)

	// Create provider based on type
	prov, err := createProvider(providerType, apiKey, printer)
	if err != nil {
		return err
	}

	// Create tool registry
	registry := tool.NewRegistry()
	registerBuiltinTools(registry)

	// Create session manager
	sessMgr, err := session.NewSessionManager(&session.ManagerOptions{
		ProjectPath: cwd,
	})
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	// Try to resume the latest session for this project
	var sess *session.Session
	sess, err = sessMgr.ResumeLatest()
	if err != nil {
		// No existing session, create a new one
		sess, err = sessMgr.NewSession(&session.SessionOptions{
			ProjectPath: cwd,
			CWD:         cwd,
			Model:       provider.ResolveModel(model),
			Version:     version,
			MaxTokens:   200000,
		})
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		if verbose {
			printer.Dim("Started new session: %s", sess.ID[:8])
		}
	} else {
		printer.Dim("Resumed session: %s (%d messages)", sess.ID[:8], len(sess.Messages))
	}

	// Create work context manager
	workMgr := workctx.NewManager("")

	// Create engine
	eng := engine.NewEngine(&engine.EngineOptions{
		Provider:      prov,
		Registry:      registry,
		Session:       sess,
		MaxIterations: 100,
		MaxTokens:     16384,
		SystemPrompt:  getSystemPrompt(),
	})

	// Check for --no-tui flag
	noTUI, _ := cmd.Flags().GetBool("no-tui")

	// Use TUI mode if enabled and not disabled
	if useTUI && !noTUI {
		sessionID := sess.ID
		if len(sessionID) > 8 {
			sessionID = sessionID[:8]
		}
		runner := tui.NewRunner(eng, tui.Config{
			Model:        sess.Model,
			CWD:          cwd,
			Version:      version,
			SessionID:    sessionID,
			MessageCount: len(sess.Messages),
		})
		return runner.Run()
	}

	// Set callbacks using ui package (classic mode)
	eng.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			fmt.Print(text)
		},
		OnThinking: func(text string) {
			if verbose {
				printer.Thinking(text)
			}
		},
		OnToolUse: func(name string, input map[string]interface{}) {
			fmt.Println()
			printer.Tool(name)
			fmt.Println()
			for k, v := range input {
				printer.ToolParam(k, fmt.Sprintf("%v", v))
			}
		},
		OnToolResult: func(name string, result *tool.Output) {
			if result.IsError {
				printer.ToolError(name, result.Content)
			} else {
				content := result.Content
				lines := strings.Split(content, "\n")
				if len(lines) > 5 {
					printer.ToolSuccess(name, fmt.Sprintf("%d lines", len(lines)))
				} else if len(content) > 200 {
					printer.ToolSuccess(name, "")
				} else if content != "" {
					summary := strings.ReplaceAll(content, "\n", " ")
					if len(summary) > 60 {
						summary = summary[:60] + "..."
					}
					printer.ToolSuccess(name, summary)
				} else {
					printer.ToolSuccess(name, "")
				}
			}
		},
		OnError: func(err error) {
			printer.Error("%v", err)
		},
	})

	// Signal handling for Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Track current operation context
	var currentCancel context.CancelFunc
	var isRunning bool
	var mu sync.Mutex

	// Handle signals in background
	go func() {
		for range sigCh {
			mu.Lock()
			if isRunning && currentCancel != nil {
				// First Ctrl+C: cancel current operation
				printer.Warning("Interrupted. Press Ctrl+C again to exit.")
				currentCancel()
				mu.Unlock()
			} else {
				// Second Ctrl+C or idle: exit
				mu.Unlock()
				fmt.Println()
				printer.Dim("Goodbye!")
				os.Exit(0)
			}
		}
	}()

	// Print welcome banner
	printer.WelcomeBanner(version, sess.Model, cwd)

	// Create chat context for handling commands
	chatCtx := &chatContext{
		session:    sess,
		sessMgr:    sessMgr,
		workMgr:    workMgr,
		printer:    printer,
		engine:     eng,
		provider:   prov,
		provType:   providerType,
	}

	// Interactive loop
	reader := bufio.NewReader(os.Stdin)
	for {
		printer.Prompt()

		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, chatCtx) {
				continue
			}
		}

		// Create context for this operation
		ctx, cancel := context.WithCancel(context.Background())
		mu.Lock()
		currentCancel = cancel
		isRunning = true
		mu.Unlock()

		// Run engine
		fmt.Println()
		err = eng.Run(ctx, input)

		// Mark operation as done
		mu.Lock()
		isRunning = false
		currentCancel = nil
		mu.Unlock()
		cancel() // Clean up context

		if err != nil {
			if ctx.Err() != nil {
				// User interrupted, continue to next input
				fmt.Println()
				continue
			}
			printer.Error("%v", err)
		}
		fmt.Println()

		// Save session
		if err := sessMgr.SaveSession(sess); err != nil {
			if verbose {
				printer.Warning("Failed to save session: %v", err)
			}
		}
	}

	return nil
}

// chatContext holds the state for interactive chat
type chatContext struct {
	session    *session.Session
	sessMgr    *session.SessionManager
	workMgr    *workctx.Manager
	printer    *ui.Printer
	engine     *engine.Engine
	provider   provider.AIProvider
	provType   provider.ProviderType
}

func handleCommand(cmd string, ctx *chatContext) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	switch parts[0] {
	case "/help", "/h":
		ctx.printer.HelpMenu()
		return true

	case "/clear", "/cls":
		fmt.Print("\033[H\033[2J")
		return true

	case "/session":
		// Get timestamps from session messages
		var createdAt, updatedAt time.Time
		if len(ctx.session.Messages) > 0 {
			createdAt = ctx.session.Messages[0].Timestamp
			updatedAt = ctx.session.Messages[len(ctx.session.Messages)-1].Timestamp
		} else {
			createdAt = time.Now()
			updatedAt = time.Now()
		}
		ctx.printer.SessionInfo(
			ctx.session.ID,
			ctx.session.Model,
			len(ctx.session.Messages),
			createdAt,
			updatedAt,
		)
		return true

	case "/sessions":
		sessions, err := ctx.sessMgr.ListSessions()
		if err != nil {
			ctx.printer.Error("Failed to list sessions: %v", err)
			return true
		}
		items := make([]ui.SessionListItem, 0, len(sessions))
		for _, s := range sessions {
			items = append(items, ui.SessionListItem{
				ID:        s.ID,
				Preview:   fmt.Sprintf("%s (%d messages)", s.Model, s.MessageCount),
				UpdatedAt: s.LastUpdated,
				IsCurrent: s.ID == ctx.session.ID,
			})
		}
		ctx.printer.SessionList(items)
		return true

	case "/resume":
		if len(parts) < 2 {
			ctx.printer.Warning("Usage: /resume <session-id>")
			return true
		}
		sessionID := parts[1]
		sess, err := ctx.sessMgr.GetSession(sessionID)
		if err != nil {
			ctx.printer.Error("Failed to load session: %v", err)
			return true
		}
		ctx.session = sess
		ctx.printer.Success("Resumed session: %s", sessionID)
		var createdAt, updatedAt time.Time
		if len(sess.Messages) > 0 {
			createdAt = sess.Messages[0].Timestamp
			updatedAt = sess.Messages[len(sess.Messages)-1].Timestamp
		} else {
			createdAt = time.Now()
			updatedAt = time.Now()
		}
		ctx.printer.SessionInfo(sess.ID, sess.Model, len(sess.Messages), createdAt, updatedAt)
		return true

	case "/new":
		cwd, _ := os.Getwd()
		sess, err := ctx.sessMgr.NewSession(&session.SessionOptions{
			ProjectPath: cwd,
			CWD:         cwd,
			Model:       ctx.session.Model,
			Version:     version,
			MaxTokens:   200000,
		})
		if err != nil {
			ctx.printer.Error("Failed to create session: %v", err)
			return true
		}
		ctx.session = sess
		ctx.printer.Success("Started new session: %s", sess.ID)
		return true

	case "/save":
		if err := ctx.sessMgr.SaveSession(ctx.session); err != nil {
			ctx.printer.Error("Failed to save session: %v", err)
		} else {
			ctx.printer.Success("Session saved: %s", ctx.session.ID)
		}
		return true

	case "/model":
		if len(parts) > 1 {
			ctx.session.Model = provider.ResolveModel(parts[1])
			ctx.printer.Success("Model changed to: %s", ctx.session.Model)
		} else {
			ctx.printer.Info("Current model: %s", ctx.session.Model)
		}
		return true

	case "/work":
		handleWorkCommand(parts[1:], ctx)
		return true

	case "/cost":
		// Estimate tokens from session
		tokenCount := ctx.session.EstimateTokens()
		ctx.printer.CostSummary(
			int64(tokenCount),
			0, // Output tokens not tracked separately
			0, // TODO: calculate cost based on model
		)
		return true

	case "/compact":
		// TODO: implement conversation compaction
		ctx.printer.Info("Conversation compaction not yet implemented")
		return true

	case "/exit", "/quit", "/q":
		ctx.printer.Dim("Goodbye!")
		os.Exit(0)

	default:
		ctx.printer.Warning("Unknown command: %s. Type /help for available commands.", parts[0])
		return true
	}

	return false
}

func handleWorkCommand(args []string, ctx *chatContext) {
	if len(args) == 0 {
		// Show current work context or list
		if ctx.workMgr.Current() != nil {
			current := ctx.workMgr.Current()
			ctx.printer.Info("Current work: %s - %s", current.ID, current.Title)
			ctx.printer.Dim("%s", current.Summary())
		} else {
			contexts, _ := ctx.workMgr.List()
			items := make([]ui.WorkContextItem, 0, len(contexts))
			for _, c := range contexts {
				items = append(items, ui.WorkContextItem{
					ID:      c.ID[:8],
					Title:   c.Title,
					Done:    len(c.Progress),
					Pending: len(c.Pending),
				})
			}
			ctx.printer.WorkContextList(items)
		}
		return
	}

	switch args[0] {
	case "new":
		if len(args) < 2 {
			ctx.printer.Warning("Usage: /work new <title>")
			return
		}
		title := strings.Join(args[1:], " ")
		wctx := ctx.workMgr.New(title, "")
		if err := ctx.workMgr.Save(wctx); err != nil {
			ctx.printer.Error("Failed to create work context: %v", err)
			return
		}
		ctx.printer.Success("Created work context: %s", wctx.ID)

	case "list":
		contexts, _ := ctx.workMgr.List()
		items := make([]ui.WorkContextItem, 0, len(contexts))
		for _, c := range contexts {
			items = append(items, ui.WorkContextItem{
				ID:      c.ID[:8],
				Title:   c.Title,
				Done:    len(c.Progress),
				Pending: len(c.Pending),
			})
		}
		ctx.printer.WorkContextList(items)

	case "show":
		if len(args) < 2 {
			ctx.printer.Warning("Usage: /work show <id>")
			return
		}
		wctx, err := ctx.workMgr.Load(args[1])
		if err != nil {
			ctx.printer.Error("Work context not found: %s", args[1])
			return
		}
		fmt.Println(wctx.GenerateHandoff())

	case "done":
		if ctx.workMgr.Current() == nil {
			ctx.printer.Warning("No active work context. Use '/work new <title>' first.")
			return
		}
		if len(args) < 2 {
			ctx.printer.Warning("Usage: /work done <description>")
			return
		}
		current := ctx.workMgr.Current()
		current.AddProgress(strings.Join(args[1:], " "))
		ctx.workMgr.Save(current)
		ctx.printer.Success("Marked as done: %s", strings.Join(args[1:], " "))

	case "todo":
		if ctx.workMgr.Current() == nil {
			ctx.printer.Warning("No active work context. Use '/work new <title>' first.")
			return
		}
		if len(args) < 2 {
			ctx.printer.Warning("Usage: /work todo <description>")
			return
		}
		current := ctx.workMgr.Current()
		current.AddPending(strings.Join(args[1:], " "))
		ctx.workMgr.Save(current)
		ctx.printer.Success("Added todo: %s", strings.Join(args[1:], " "))

	case "handoff":
		var wctx *workctx.WorkContext
		if len(args) >= 2 {
			var err error
			wctx, err = ctx.workMgr.Load(args[1])
			if err != nil {
				ctx.printer.Error("Work context not found: %s", args[1])
				return
			}
		} else if ctx.workMgr.Current() != nil {
			wctx = ctx.workMgr.Current()
		} else {
			ctx.printer.Warning("Usage: /work handoff [id]")
			return
		}
		fmt.Println(wctx.GenerateHandoff())

	default:
		ctx.printer.Warning("Unknown work command: %s", args[0])
		ctx.printer.Dim("Available: new, list, show, done, todo, handoff")
	}
}

func registerBuiltinTools(registry *tool.Registry) {
	// Core file tools
	registry.Register(builtin.NewReadTool())
	registry.Register(builtin.NewWriteTool())
	registry.Register(builtin.NewEditTool())
	registry.Register(builtin.NewGlobTool())
	registry.Register(builtin.NewGrepTool())

	// Shell tools
	registry.Register(builtin.NewBashTool())
	shellMgr := builtin.NewShellManager()
	registry.Register(builtin.NewKillShellTool(shellMgr))

	// Web tools
	registry.Register(builtin.NewWebSearchTool())
	registry.Register(builtin.NewWebFetchTool())

	// Notebook tools
	registry.Register(builtin.NewNotebookEditTool())

	// Plan mode tools
	var inPlanMode bool
	registry.Register(builtin.NewEnterPlanModeTool(&inPlanMode, nil))
	registry.Register(builtin.NewExitPlanModeTool(&inPlanMode, nil))
}

func getSystemPrompt() string {
	builder := engine.NewPromptBuilder()
	builder.LoadClaudeMD()
	return builder.Build()
}

// createProvider creates a provider based on type
func createProvider(providerType provider.ProviderType, customKey string, printer *ui.Printer) (provider.AIProvider, error) {
	// Try to get credentials from auth manager first
	authMgr := auth.NewManager("")

	switch providerType {
	case provider.ProviderTypeClaude:
		// Try auth manager first (API key only)
		if creds, err := authMgr.GetCredentials(auth.ProviderClaude); err == nil && creds.APIKey != "" {
			printer.Dim("%s Using saved API key for Claude", ui.IconKey)
			return claude.New(creds.APIKey, claude.WithBeta("interleaved-thinking-2025-05-14")), nil
		}

		// Fall back to API key from env or custom key
		key := customKey
		if key == "" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("no authentication configured.\nSet ANTHROPIC_API_KEY environment variable, or run 'agentic-coder auth login claude'")
		}
		return claude.New(key, claude.WithBeta("interleaved-thinking-2025-05-14")), nil

	case provider.ProviderTypeClaudeCLI:
		// Use local Claude Code CLI
		printer.Dim("%s Using local Claude Code CLI", ui.IconGear)
		return claudecli.New(claudecli.WithModel("sonnet")), nil

	case provider.ProviderTypeOpenAI:
		// Try auth manager first (API key only)
		if creds, err := authMgr.GetCredentials(auth.ProviderOpenAI); err == nil && creds.APIKey != "" {
			printer.Dim("%s Using saved API key for OpenAI", ui.IconKey)
			return openai.New(creds.APIKey), nil
		}

		key := customKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("no authentication configured.\nSet OPENAI_API_KEY environment variable, or run 'agentic-coder auth login openai'")
		}
		return openai.New(key), nil

	case provider.ProviderTypeCodexCLI:
		// Use local Codex CLI
		printer.Dim("%s Using local Codex CLI", ui.IconGear)
		return codexcli.New(codexcli.WithModel("o3-mini")), nil

	case provider.ProviderTypeGemini:
		// Try auth manager first (API key only)
		if creds, err := authMgr.GetCredentials(auth.ProviderGemini); err == nil && creds.APIKey != "" {
			printer.Dim("%s Using saved API key for Gemini", ui.IconKey)
			return gemini.New(creds.APIKey), nil
		}

		key := customKey
		if key == "" {
			key = os.Getenv("GOOGLE_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("no authentication configured.\nSet GOOGLE_API_KEY environment variable, or run 'agentic-coder auth login gemini'")
		}
		return gemini.New(key), nil

	case provider.ProviderTypeGeminiCLI:
		// Use local Gemini CLI
		printer.Dim("%s Using local Gemini CLI", ui.IconGear)
		return geminicli.New(geminicli.WithModel("gemini-2.5-pro")), nil

	case provider.ProviderTypeDeepSeek:
		key := customKey
		if key == "" {
			key = os.Getenv("DEEPSEEK_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
		}
		printer.Dim("%s Using DeepSeek API", ui.IconKey)
		return deepseek.New(key), nil

	case provider.ProviderTypeOllama:
		// Ollama runs locally, no API key needed
		baseURL := os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		printer.Dim("%s Using Ollama at %s", ui.IconGear, baseURL)
		return ollama.New(ollama.WithBaseURL(baseURL)), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}
