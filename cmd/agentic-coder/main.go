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
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	model   string
	apiKey  string
	verbose bool
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

	// Subcommands
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(authCmd())

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

func runChat(cmd *cobra.Command, args []string) error {
	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Detect provider from model
	providerType := provider.DetectProviderFromModel(model)

	// Create provider based on type
	prov, err := createProvider(providerType, apiKey)
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

	// Create new session
	sess, err := sessMgr.NewSession(&session.SessionOptions{
		ProjectPath: cwd,
		CWD:         cwd,
		Model:       provider.ResolveModel(model),
		Version:     version,
		MaxTokens:   200000,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create engine
	eng := engine.NewEngine(&engine.EngineOptions{
		Provider:      prov,
		Registry:      registry,
		Session:       sess,
		MaxIterations: 100,
		MaxTokens:     16384,
		SystemPrompt:  getSystemPrompt(),
	})

	// Set callbacks
	eng.SetCallbacks(&engine.CallbackOptions{
		OnText: func(text string) {
			fmt.Print(text)
		},
		OnThinking: func(text string) {
			if verbose {
				fmt.Printf("\033[90m[thinking] %s\033[0m", text)
			}
		},
		OnToolUse: func(name string, input map[string]interface{}) {
			fmt.Printf("\n\033[33m⚡ Using tool: %s\033[0m\n", name)
			// Show tool input parameters
			for k, v := range input {
				valStr := fmt.Sprintf("%v", v)
				if len(valStr) > 100 {
					valStr = valStr[:100] + "..."
				}
				// Replace newlines for cleaner display
				valStr = strings.ReplaceAll(valStr, "\n", "\\n")
				fmt.Printf("   \033[90m%s: %s\033[0m\n", k, valStr)
			}
		},
		OnToolResult: func(name string, result *tool.Output) {
			if result.IsError {
				fmt.Printf("\033[31m✗ Tool error: %s\033[0m\n", result.Content)
			} else {
				// Show result summary
				content := result.Content
				lines := strings.Split(content, "\n")
				if len(lines) > 5 {
					fmt.Printf("\033[32m✓ %s completed (%d lines)\033[0m\n", name, len(lines))
					// Show first 3 lines as preview
					for i := 0; i < 3 && i < len(lines); i++ {
						line := lines[i]
						if len(line) > 80 {
							line = line[:80] + "..."
						}
						fmt.Printf("   \033[90m%s\033[0m\n", line)
					}
					if len(lines) > 3 {
						fmt.Printf("   \033[90m... (%d more lines)\033[0m\n", len(lines)-3)
					}
				} else if len(content) > 200 {
					fmt.Printf("\033[32m✓ %s completed\033[0m\n", name)
					fmt.Printf("   \033[90m%s...\033[0m\n", content[:200])
				} else if content != "" {
					fmt.Printf("\033[32m✓ %s: %s\033[0m\n", name, strings.ReplaceAll(content, "\n", " "))
				} else {
					fmt.Printf("\033[32m✓ %s completed\033[0m\n", name)
				}
			}
		},
		OnError: func(err error) {
			fmt.Printf("\033[31mError: %v\033[0m\n", err)
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
				fmt.Println("\n\033[33m⚠ Interrupted. Press Ctrl+C again to exit.\033[0m")
				currentCancel()
				mu.Unlock()
			} else {
				// Second Ctrl+C or idle: exit
				mu.Unlock()
				fmt.Println("\n\nGoodbye!")
				os.Exit(0)
			}
		}
	}()

	// Print welcome message
	fmt.Printf("\033[1magentic-coder v%s\033[0m\n", version)
	fmt.Printf("Model: %s | CWD: %s\n", sess.Model, cwd)
	fmt.Println("Type your message, or /help for commands. Ctrl+C to interrupt, twice to exit.")

	// Interactive loop
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\033[1m> \033[0m")

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
			if handleCommand(input, sess, sessMgr) {
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
			fmt.Printf("\033[31mError: %v\033[0m\n", err)
		}
		fmt.Println()

		// Save session
		if err := sessMgr.SaveSession(sess); err != nil {
			if verbose {
				fmt.Printf("\033[33mWarning: Failed to save session: %v\033[0m\n", err)
			}
		}
	}

	return nil
}

func handleCommand(cmd string, sess *session.Session, mgr *session.SessionManager) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	switch parts[0] {
	case "/help", "/h":
		printHelp()
		return true

	case "/clear":
		fmt.Print("\033[H\033[2J")
		return true

	case "/session":
		fmt.Printf("Session ID: %s\n", sess.ID)
		fmt.Printf("Messages: %d\n", len(sess.Messages))
		return true

	case "/model":
		if len(parts) > 1 {
			sess.Model = provider.ResolveModel(parts[1])
			fmt.Printf("Model changed to: %s\n", sess.Model)
		} else {
			fmt.Printf("Current model: %s\n", sess.Model)
		}
		return true

	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		os.Exit(0)

	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
		return true
	}

	return false
}

func printHelp() {
	help := `
Commands:
  /help, /h      Show this help
  /clear         Clear the screen
  /session       Show current session info
  /model [name]  Show or change the model
  /exit, /quit   Exit the program

Keyboard shortcuts:
  Ctrl+C         Interrupt current operation
  Ctrl+D         Exit the program
`
	fmt.Println(help)
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
func createProvider(providerType provider.ProviderType, customKey string) (provider.AIProvider, error) {
	// Try to get credentials from auth manager first
	authMgr := auth.NewManager("")

	switch providerType {
	case provider.ProviderTypeClaude:
		// Try auth manager first (API key only)
		if creds, err := authMgr.GetCredentials(auth.ProviderClaude); err == nil && creds.APIKey != "" {
			fmt.Println("🔐 Using saved API key for Claude")
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
		fmt.Println("🔧 Using local Claude Code CLI")
		return claudecli.New(claudecli.WithModel("sonnet")), nil

	case provider.ProviderTypeOpenAI:
		// Try auth manager first (API key only)
		if creds, err := authMgr.GetCredentials(auth.ProviderOpenAI); err == nil && creds.APIKey != "" {
			fmt.Println("🔐 Using saved API key for OpenAI")
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
		fmt.Println("🔧 Using local Codex CLI")
		return codexcli.New(codexcli.WithModel("o3-mini")), nil

	case provider.ProviderTypeGemini:
		// Try auth manager first (API key only)
		if creds, err := authMgr.GetCredentials(auth.ProviderGemini); err == nil && creds.APIKey != "" {
			fmt.Println("🔐 Using saved API key for Gemini")
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
		fmt.Println("🔧 Using local Gemini CLI")
		return geminicli.New(geminicli.WithModel("gemini-2.5-pro")), nil

	case provider.ProviderTypeDeepSeek:
		key := customKey
		if key == "" {
			key = os.Getenv("DEEPSEEK_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
		}
		return deepseek.New(key), nil

	case provider.ProviderTypeOllama:
		// Ollama runs locally, no API key needed
		baseURL := os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		fmt.Printf("🦙 Using Ollama at %s\n", baseURL)
		return ollama.New(ollama.WithBaseURL(baseURL)), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}
