// Package workctx provides work context management for task continuity across provider switches
package workctx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WorkContext represents a work context that can be saved and resumed
type WorkContext struct {
	// ID is the unique identifier
	ID string `json:"id"`

	// Title is a short descriptive title
	Title string `json:"title"`

	// Goal is what needs to be accomplished
	Goal string `json:"goal"`

	// Background provides context and relevant information
	Background string `json:"background"`

	// Progress lists completed items
	Progress []ProgressItem `json:"progress"`

	// Pending lists items still to be done
	Pending []ProgressItem `json:"pending"`

	// KeyFiles lists important files involved
	KeyFiles []string `json:"key_files"`

	// Notes contains important decisions, findings, or reminders
	Notes []string `json:"notes"`

	// Provider is the last used provider
	Provider string `json:"provider"`

	// Model is the last used model
	Model string `json:"model"`

	// ProjectPath is the project directory
	ProjectPath string `json:"project_path"`

	// CreatedAt is when the context was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the context was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// TokensUsed tracks token usage per provider
	TokensUsed map[string]int64 `json:"tokens_used"`
}

// ProgressItem represents a single progress item
type ProgressItem struct {
	// Description of the item
	Description string `json:"description"`

	// Status: done, in_progress, pending, blocked
	Status string `json:"status"`

	// CompletedAt is when the item was completed
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Notes for this specific item
	Notes string `json:"notes,omitempty"`
}

// Manager manages work contexts
type Manager struct {
	configDir string
	current   *WorkContext
}

// NewManager creates a new work context manager
func NewManager(configDir string) *Manager {
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config", "agentic-coder")
	}

	return &Manager{
		configDir: configDir,
	}
}

// contextsDir returns the directory for storing contexts
func (m *Manager) contextsDir() string {
	return filepath.Join(m.configDir, "workctx")
}

// New creates a new work context
func (m *Manager) New(title, goal string) *WorkContext {
	now := time.Now()
	ctx := &WorkContext{
		ID:         uuid.New().String()[:8],
		Title:      title,
		Goal:       goal,
		Progress:   make([]ProgressItem, 0),
		Pending:    make([]ProgressItem, 0),
		KeyFiles:   make([]string, 0),
		Notes:      make([]string, 0),
		TokensUsed: make(map[string]int64),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	m.current = ctx
	return ctx
}

// Current returns the current work context
func (m *Manager) Current() *WorkContext {
	return m.current
}

// SetCurrent sets the current work context
func (m *Manager) SetCurrent(ctx *WorkContext) {
	m.current = ctx
}

// Save saves a work context to disk
func (m *Manager) Save(ctx *WorkContext) error {
	if ctx == nil {
		return fmt.Errorf("no context to save")
	}

	ctx.UpdatedAt = time.Now()

	dir := m.contextsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(dir, ctx.ID+".json")
	return os.WriteFile(filename, data, 0600)
}

// Load loads a work context by ID
func (m *Manager) Load(id string) (*WorkContext, error) {
	filename := filepath.Join(m.contextsDir(), id+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var ctx WorkContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, err
	}

	m.current = &ctx
	return &ctx, nil
}

// List returns all saved work contexts
func (m *Manager) List() ([]*WorkContext, error) {
	dir := m.contextsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*WorkContext{}, nil
		}
		return nil, err
	}

	var contexts []*WorkContext
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			ctx, err := m.Load(id)
			if err == nil {
				contexts = append(contexts, ctx)
			}
		}
	}

	// Sort by updated time, newest first
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].UpdatedAt.After(contexts[j].UpdatedAt)
	})

	return contexts, nil
}

// Delete removes a work context
func (m *Manager) Delete(id string) error {
	filename := filepath.Join(m.contextsDir(), id+".json")
	return os.Remove(filename)
}

// AddProgress adds a completed item to the context
func (ctx *WorkContext) AddProgress(description string) {
	now := time.Now()
	ctx.Progress = append(ctx.Progress, ProgressItem{
		Description: description,
		Status:      "done",
		CompletedAt: &now,
	})
	ctx.UpdatedAt = now
}

// AddPending adds a pending item to the context
func (ctx *WorkContext) AddPending(description string) {
	ctx.Pending = append(ctx.Pending, ProgressItem{
		Description: description,
		Status:      "pending",
	})
	ctx.UpdatedAt = time.Now()
}

// CompletePending marks a pending item as done
func (ctx *WorkContext) CompletePending(index int) {
	if index >= 0 && index < len(ctx.Pending) {
		item := ctx.Pending[index]
		now := time.Now()
		item.Status = "done"
		item.CompletedAt = &now

		// Move to progress
		ctx.Progress = append(ctx.Progress, item)

		// Remove from pending
		ctx.Pending = append(ctx.Pending[:index], ctx.Pending[index+1:]...)
		ctx.UpdatedAt = now
	}
}

// AddKeyFile adds a key file to the context
func (ctx *WorkContext) AddKeyFile(file string) {
	for _, f := range ctx.KeyFiles {
		if f == file {
			return
		}
	}
	ctx.KeyFiles = append(ctx.KeyFiles, file)
	ctx.UpdatedAt = time.Now()
}

// AddNote adds a note to the context
func (ctx *WorkContext) AddNote(note string) {
	ctx.Notes = append(ctx.Notes, note)
	ctx.UpdatedAt = time.Now()
}

// UpdateTokens updates token usage for a provider
func (ctx *WorkContext) UpdateTokens(provider string, tokens int64) {
	if ctx.TokensUsed == nil {
		ctx.TokensUsed = make(map[string]int64)
	}
	ctx.TokensUsed[provider] += tokens
	ctx.UpdatedAt = time.Now()
}

// GenerateHandoff generates a handoff summary for switching providers
func (ctx *WorkContext) GenerateHandoff() string {
	var sb strings.Builder

	sb.WriteString("# Work Context Handoff\n\n")

	// Basic info
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", ctx.ID))
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", ctx.Title))
	sb.WriteString(fmt.Sprintf("**Last Updated:** %s\n\n", ctx.UpdatedAt.Format("2006-01-02 15:04:05")))

	// Goal
	sb.WriteString("## Goal\n\n")
	sb.WriteString(ctx.Goal)
	sb.WriteString("\n\n")

	// Background
	if ctx.Background != "" {
		sb.WriteString("## Background\n\n")
		sb.WriteString(ctx.Background)
		sb.WriteString("\n\n")
	}

	// Progress
	if len(ctx.Progress) > 0 {
		sb.WriteString("## Completed\n\n")
		for _, item := range ctx.Progress {
			sb.WriteString(fmt.Sprintf("- [x] %s\n", item.Description))
		}
		sb.WriteString("\n")
	}

	// Pending
	if len(ctx.Pending) > 0 {
		sb.WriteString("## Remaining Tasks\n\n")
		for _, item := range ctx.Pending {
			status := " "
			if item.Status == "in_progress" {
				status = "~"
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", status, item.Description))
		}
		sb.WriteString("\n")
	}

	// Key files
	if len(ctx.KeyFiles) > 0 {
		sb.WriteString("## Key Files\n\n")
		for _, file := range ctx.KeyFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
		sb.WriteString("\n")
	}

	// Notes
	if len(ctx.Notes) > 0 {
		sb.WriteString("## Important Notes\n\n")
		for _, note := range ctx.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
		sb.WriteString("\n")
	}

	// Token usage
	if len(ctx.TokensUsed) > 0 {
		sb.WriteString("## Token Usage\n\n")
		for provider, tokens := range ctx.TokensUsed {
			sb.WriteString(fmt.Sprintf("- %s: %d tokens\n", provider, tokens))
		}
		sb.WriteString("\n")
	}

	// Instructions for next provider
	sb.WriteString("## Instructions for Continuation\n\n")
	sb.WriteString("Please continue from where we left off. ")
	if len(ctx.Pending) > 0 {
		sb.WriteString(fmt.Sprintf("The next task is: **%s**\n", ctx.Pending[0].Description))
	}

	return sb.String()
}

// GenerateHandoffCN generates a Chinese handoff summary
func (ctx *WorkContext) GenerateHandoffCN() string {
	var sb strings.Builder

	sb.WriteString("# 工作上下文交接\n\n")

	// Basic info
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", ctx.ID))
	sb.WriteString(fmt.Sprintf("**标题:** %s\n", ctx.Title))
	sb.WriteString(fmt.Sprintf("**最后更新:** %s\n\n", ctx.UpdatedAt.Format("2006-01-02 15:04:05")))

	// Goal
	sb.WriteString("## 目标\n\n")
	sb.WriteString(ctx.Goal)
	sb.WriteString("\n\n")

	// Background
	if ctx.Background != "" {
		sb.WriteString("## 背景\n\n")
		sb.WriteString(ctx.Background)
		sb.WriteString("\n\n")
	}

	// Progress
	if len(ctx.Progress) > 0 {
		sb.WriteString("## 已完成\n\n")
		for _, item := range ctx.Progress {
			sb.WriteString(fmt.Sprintf("- [x] %s\n", item.Description))
		}
		sb.WriteString("\n")
	}

	// Pending
	if len(ctx.Pending) > 0 {
		sb.WriteString("## 待完成\n\n")
		for _, item := range ctx.Pending {
			status := " "
			if item.Status == "in_progress" {
				status = "~"
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", status, item.Description))
		}
		sb.WriteString("\n")
	}

	// Key files
	if len(ctx.KeyFiles) > 0 {
		sb.WriteString("## 关键文件\n\n")
		for _, file := range ctx.KeyFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
		sb.WriteString("\n")
	}

	// Notes
	if len(ctx.Notes) > 0 {
		sb.WriteString("## 重要说明\n\n")
		for _, note := range ctx.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
		sb.WriteString("\n")
	}

	// Token usage
	if len(ctx.TokensUsed) > 0 {
		sb.WriteString("## Token 使用情况\n\n")
		for provider, tokens := range ctx.TokensUsed {
			sb.WriteString(fmt.Sprintf("- %s: %d tokens\n", provider, tokens))
		}
		sb.WriteString("\n")
	}

	// Instructions
	sb.WriteString("## 继续说明\n\n")
	sb.WriteString("请从上次中断的地方继续。")
	if len(ctx.Pending) > 0 {
		sb.WriteString(fmt.Sprintf("下一个任务是：**%s**\n", ctx.Pending[0].Description))
	}

	return sb.String()
}

// Summary returns a brief summary of the context
func (ctx *WorkContext) Summary() string {
	completed := len(ctx.Progress)
	pending := len(ctx.Pending)
	total := completed + pending

	progress := "0%"
	if total > 0 {
		progress = fmt.Sprintf("%d%%", completed*100/total)
	}

	return fmt.Sprintf("[%s] %s - %s (%d/%d done)", ctx.ID, ctx.Title, progress, completed, total)
}
