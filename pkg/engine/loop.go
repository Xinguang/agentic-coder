// Package engine implements the core agent loop
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/session"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// Engine represents the agent loop engine
type Engine struct {
	provider     provider.AIProvider
	registry     *tool.Registry
	session      *session.Session
	hooks        *HookManager
	systemPrompt string

	// Configuration
	maxIterations int
	maxTokens     int
	temperature   float64
	thinkingLevel string // high, medium, low, none

	// Callbacks
	onText      func(text string)
	onThinking  func(text string)
	onToolUse   func(name string, input map[string]interface{})
	onToolResult func(name string, result *tool.Output)
	onError     func(err error)
}

// EngineOptions holds engine configuration
type EngineOptions struct {
	Provider      provider.AIProvider
	Registry      *tool.Registry
	Session       *session.Session
	MaxIterations int
	MaxTokens     int
	Temperature   float64
	ThinkingLevel string
	SystemPrompt  string
}

// NewEngine creates a new agent engine
func NewEngine(opts *EngineOptions) *Engine {
	maxIterations := opts.MaxIterations
	if maxIterations == 0 {
		maxIterations = 100 // Default max iterations
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 16384
	}

	return &Engine{
		provider:      opts.Provider,
		registry:      opts.Registry,
		session:       opts.Session,
		hooks:         NewHookManager(),
		systemPrompt:  opts.SystemPrompt,
		maxIterations: maxIterations,
		maxTokens:     maxTokens,
		temperature:   opts.Temperature,
		thinkingLevel: opts.ThinkingLevel,
	}
}

// SetCallbacks sets event callbacks
func (e *Engine) SetCallbacks(opts *CallbackOptions) {
	if opts.OnText != nil {
		e.onText = opts.OnText
	}
	if opts.OnThinking != nil {
		e.onThinking = opts.OnThinking
	}
	if opts.OnToolUse != nil {
		e.onToolUse = opts.OnToolUse
	}
	if opts.OnToolResult != nil {
		e.onToolResult = opts.OnToolResult
	}
	if opts.OnError != nil {
		e.onError = opts.OnError
	}
}

// CallbackOptions holds callback functions
type CallbackOptions struct {
	OnText       func(text string)
	OnThinking   func(text string)
	OnToolUse    func(name string, input map[string]interface{})
	OnToolResult func(name string, result *tool.Output)
	OnError      func(err error)
}

// Run executes a single turn of conversation
func (e *Engine) Run(ctx context.Context, userMessage string) error {
	// Add user message to session
	e.session.AddUserMessage(userMessage)

	// Run agent loop
	return e.runLoop(ctx)
}

// runLoop executes the agent loop until completion
func (e *Engine) runLoop(ctx context.Context) error {
	for iteration := 0; iteration < e.maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Build request
		req := e.buildRequest()

		// Call AI provider
		resp, err := e.callProvider(ctx, req)
		if err != nil {
			if e.onError != nil {
				e.onError(err)
			}
			return fmt.Errorf("provider error: %w", err)
		}

		// Add assistant message to session
		e.session.AddAssistantMessage(resp)

		// Process response
		hasToolUse := false
		for _, block := range resp.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				if e.onText != nil && b.Text != "" {
					e.onText(b.Text)
				}

			case *provider.ThinkingBlock:
				if e.onThinking != nil && b.Thinking != "" {
					e.onThinking(b.Thinking)
				}

			case *provider.ToolUseBlock:
				hasToolUse = true
				if err := e.executeToolUse(ctx, b); err != nil {
					return err
				}
			}
		}

		// Check stop condition
		if resp.StopReason == provider.StopReasonMaxTokens {
			// Response was truncated due to token limit, ask to continue
			if e.onText != nil {
				e.onText("\n")
			}
			e.session.AddUserMessage("continue")
			continue
		}
		if resp.StopReason == provider.StopReasonEndTurn || !hasToolUse {
			return nil
		}
	}

	return fmt.Errorf("max iterations (%d) exceeded", e.maxIterations)
}

// buildRequest constructs the API request
func (e *Engine) buildRequest() *provider.Request {
	messages := e.session.GetMessages()
	tools := e.registry.ToAPITools()

	req := &provider.Request{
		Model:       e.session.Model,
		Messages:    messages,
		Tools:       tools,
		MaxTokens:   e.maxTokens,
		Temperature: e.temperature,
		Stream:      true,
	}

	// Build system prompt
	systemPrompt := e.buildSystemPrompt()
	if systemPrompt != "" {
		req.System = []provider.ContentBlock{
			&provider.TextBlock{Text: systemPrompt},
		}
	}

	// Configure thinking
	if e.thinkingLevel != "" && e.thinkingLevel != "none" {
		req.Thinking = &provider.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: e.getThinkingBudget(),
		}
	}

	return req
}

// buildSystemPrompt constructs the system prompt
func (e *Engine) buildSystemPrompt() string {
	var parts []string

	// Base system prompt
	if e.systemPrompt != "" {
		parts = append(parts, e.systemPrompt)
	}

	// Add environment info
	envInfo := e.getEnvironmentInfo()
	if envInfo != "" {
		parts = append(parts, envInfo)
	}

	// Add tool descriptions
	toolDesc := e.getToolDescriptions()
	if toolDesc != "" {
		parts = append(parts, toolDesc)
	}

	return strings.Join(parts, "\n\n")
}

// getEnvironmentInfo returns environment context
func (e *Engine) getEnvironmentInfo() string {
	cwd := e.session.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	info := fmt.Sprintf(`<env>
Working directory: %s
Platform: %s
Today's date: %s
</env>`, cwd, getOS(), time.Now().Format("2006-01-02"))

	return info
}

// getToolDescriptions returns tool descriptions
func (e *Engine) getToolDescriptions() string {
	tools := e.registry.List()
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Available tools:\n")

	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name(), t.Description()))
	}

	return sb.String()
}

// getThinkingBudget returns token budget for thinking
func (e *Engine) getThinkingBudget() int {
	switch e.thinkingLevel {
	case "high":
		return 10000
	case "medium":
		return 5000
	case "low":
		return 2000
	default:
		return 5000
	}
}

// callProvider calls the AI provider with streaming
func (e *Engine) callProvider(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if !req.Stream {
		return e.provider.CreateMessage(ctx, req)
	}

	// Streaming request
	stream, err := e.provider.CreateMessageStream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var response *provider.Response
	var currentBlockIndex int
	var toolInputJSON strings.Builder

	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch ev := event.(type) {
		case *provider.MessageStartEvent:
			response = &provider.Response{
				ID:      ev.Message.ID,
				Model:   ev.Message.Model,
				Content: make([]provider.ContentBlock, 0),
			}

		case *provider.ContentBlockStartEvent:
			currentBlockIndex = ev.Index
			// Initialize content block
			if ev.ContentBlock != nil {
				response.Content = append(response.Content, ev.ContentBlock)
			}
			toolInputJSON.Reset()

		case *provider.ContentBlockDeltaEvent:
			// Handle deltas
			if ev.Delta != nil {
				switch d := ev.Delta.(type) {
				case *provider.TextDelta:
					// Accumulate text to the current block
					if currentBlockIndex < len(response.Content) {
						if tb, ok := response.Content[currentBlockIndex].(*provider.TextBlock); ok {
							tb.Text += d.Text
						}
					}
					if e.onText != nil {
						e.onText(d.Text)
					}

				case *provider.ThinkingDelta:
					// Accumulate thinking to the current block
					if currentBlockIndex < len(response.Content) {
						if tb, ok := response.Content[currentBlockIndex].(*provider.ThinkingBlock); ok {
							tb.Thinking += d.Thinking
						}
					}
					if e.onThinking != nil {
						e.onThinking(d.Thinking)
					}

				case *provider.InputJSONDelta:
					// Accumulate tool input JSON
					toolInputJSON.WriteString(d.PartialJSON)
				}
			}

		case *provider.ContentBlockStopEvent:
			// Parse accumulated tool input JSON if applicable
			if toolInputJSON.Len() > 0 && currentBlockIndex < len(response.Content) {
				if tb, ok := response.Content[currentBlockIndex].(*provider.ToolUseBlock); ok {
					// Parse the JSON string to map
					tb.Input = parseJSONToMap(toolInputJSON.String())
				}
			}

		case *provider.MessageDeltaEvent:
			if ev.Delta != nil {
				response.StopReason = ev.Delta.StopReason
			}
			if ev.Usage != nil {
				response.Usage = *ev.Usage
			}

		case *provider.MessageStopEvent:
			// Message complete
		}
	}

	return response, nil
}

// parseJSONToMap parses a JSON string to a map
func parseJSONToMap(jsonStr string) map[string]interface{} {
	result := make(map[string]interface{})
	if jsonStr == "" {
		return result
	}
	_ = json.Unmarshal([]byte(jsonStr), &result)
	return result
}

// executeToolUse executes a tool use block
func (e *Engine) executeToolUse(ctx context.Context, block *provider.ToolUseBlock) error {
	toolName := block.Name
	toolID := block.ID

	// Get input (already parsed as map)
	input := block.Input
	if input == nil {
		input = make(map[string]interface{})
	}

	// Callback
	if e.onToolUse != nil {
		e.onToolUse(toolName, input)
	}

	// Get tool
	t, err := e.registry.Get(toolName)
	if err != nil {
		e.session.AddToolResult(toolID, fmt.Sprintf("Error: %v", err), true, nil)
		return nil
	}

	// Run pre-tool-use hooks
	hookResult := e.hooks.RunPreToolUse(ctx, toolName, input)
	if hookResult.Blocked {
		e.session.AddToolResult(toolID, fmt.Sprintf("Tool blocked: %s", hookResult.Message), true, nil)
		return nil
	}

	// Build tool input
	toolInput := &tool.Input{
		Params: input,
		Context: &tool.ExecutionContext{
			CWD:       e.session.CWD,
			SessionID: e.session.ID,
		},
	}

	// Validate
	if err := t.Validate(toolInput); err != nil {
		e.session.AddToolResult(toolID, fmt.Sprintf("Validation error: %v", err), true, nil)
		return nil
	}

	// Execute
	output, err := t.Execute(ctx, toolInput)
	if err != nil {
		e.session.AddToolResult(toolID, fmt.Sprintf("Execution error: %v", err), true, nil)
		return nil
	}

	// Callback
	if e.onToolResult != nil {
		e.onToolResult(toolName, output)
	}

	// Run post-tool-use hooks
	e.hooks.RunPostToolUse(ctx, toolName, input, output)

	// Add result to session
	e.session.AddToolResult(toolID, output.Content, output.IsError, output.Metadata)

	return nil
}

// getOS returns the operating system name
func getOS() string {
	switch os := os.Getenv("GOOS"); os {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return "Unknown"
	}
}

// HookManager manages lifecycle hooks
type HookManager struct {
	preToolUse  []PreToolUseHook
	postToolUse []PostToolUseHook
	onStop      []StopHook
}

// NewHookManager creates a new hook manager
func NewHookManager() *HookManager {
	return &HookManager{
		preToolUse:  make([]PreToolUseHook, 0),
		postToolUse: make([]PostToolUseHook, 0),
		onStop:      make([]StopHook, 0),
	}
}

// PreToolUseHook is called before tool execution
type PreToolUseHook func(ctx context.Context, toolName string, input map[string]interface{}) *HookResult

// PostToolUseHook is called after tool execution
type PostToolUseHook func(ctx context.Context, toolName string, input map[string]interface{}, output *tool.Output)

// StopHook is called when agent stops
type StopHook func(ctx context.Context, reason string)

// HookResult represents the result of a hook
type HookResult struct {
	Blocked bool
	Message string
	ModifiedInput map[string]interface{}
}

// RegisterPreToolUse registers a pre-tool-use hook
func (h *HookManager) RegisterPreToolUse(hook PreToolUseHook) {
	h.preToolUse = append(h.preToolUse, hook)
}

// RegisterPostToolUse registers a post-tool-use hook
func (h *HookManager) RegisterPostToolUse(hook PostToolUseHook) {
	h.postToolUse = append(h.postToolUse, hook)
}

// RegisterOnStop registers a stop hook
func (h *HookManager) RegisterOnStop(hook StopHook) {
	h.onStop = append(h.onStop, hook)
}

// RunPreToolUse runs all pre-tool-use hooks
func (h *HookManager) RunPreToolUse(ctx context.Context, toolName string, input map[string]interface{}) *HookResult {
	for _, hook := range h.preToolUse {
		result := hook(ctx, toolName, input)
		if result != nil && result.Blocked {
			return result
		}
		if result != nil && result.ModifiedInput != nil {
			input = result.ModifiedInput
		}
	}
	return &HookResult{Blocked: false}
}

// RunPostToolUse runs all post-tool-use hooks
func (h *HookManager) RunPostToolUse(ctx context.Context, toolName string, input map[string]interface{}, output *tool.Output) {
	for _, hook := range h.postToolUse {
		hook(ctx, toolName, input, output)
	}
}

// RunOnStop runs all stop hooks
func (h *HookManager) RunOnStop(ctx context.Context, reason string) {
	for _, hook := range h.onStop {
		hook(ctx, reason)
	}
}
