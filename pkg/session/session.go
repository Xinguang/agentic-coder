// Package session manages conversation sessions and transcripts
package session

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// EntryType represents the type of transcript entry
type EntryType string

const (
	EntryTypeUser      EntryType = "user"
	EntryTypeAssistant EntryType = "assistant"
	EntryTypeSystem    EntryType = "system"
)

// TranscriptEntry represents a single entry in the conversation transcript
type TranscriptEntry struct {
	// Common fields
	Type       EntryType `json:"type"`
	UUID       string    `json:"uuid"`
	ParentUUID *string   `json:"parentUuid"`
	Timestamp  time.Time `json:"timestamp"`

	// Session context
	SessionID string `json:"sessionId"`
	Version   string `json:"version"`
	CWD       string `json:"cwd"`
	GitBranch string `json:"gitBranch,omitempty"`
	Slug      string `json:"slug,omitempty"`

	// Message content
	Message *Message `json:"message,omitempty"`

	// Subagent related
	IsSidechain bool   `json:"isSidechain"`
	AgentID     string `json:"agentId,omitempty"`

	// Tool execution result (for user entries)
	ToolUseResult interface{} `json:"toolUseResult,omitempty"`

	// Todo state
	Todos []Todo `json:"todos,omitempty"`

	// Thinking metadata
	ThinkingMetadata *ThinkingMetadata `json:"thinkingMetadata,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role    string               `json:"role"`
	Content []provider.ContentBlock `json:"content"`

	// Assistant message specific fields
	ID         string          `json:"id,omitempty"`
	Model      string          `json:"model,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
	Usage      *provider.Usage `json:"usage,omitempty"`
}

// ThinkingMetadata holds thinking mode metadata
type ThinkingMetadata struct {
	Level    string   `json:"level"`    // high, medium, low
	Disabled bool     `json:"disabled"`
	Triggers []string `json:"triggers"`
}

// Todo represents a todo item
type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending, in_progress, completed
	ActiveForm string `json:"activeForm"`
}

// Session represents a conversation session
type Session struct {
	ID          string
	Title       string // Human-readable title (from first message)
	ProjectPath string
	CWD         string
	GitBranch   string
	Model       string
	Version     string

	// Message history
	Messages    []*TranscriptEntry
	MessageTree map[string]*TranscriptEntry // UUID -> Entry

	// Current state
	CurrentUUID    string
	Todos          []Todo
	PermissionMode tool.PermissionMode

	// Context window management
	TokenCount     int
	MaxTokens      int
	CompactPercent float64 // Compact threshold (default 95%)

	// Subagent info
	ParentID    string // Parent session ID (for subagents)
	IsSidechain bool
	AgentID     string

	mu sync.RWMutex
}

// NewSession creates a new session
func NewSession(opts *SessionOptions) *Session {
	id := uuid.New().String()

	return &Session{
		ID:             id,
		ProjectPath:    opts.ProjectPath,
		CWD:            opts.CWD,
		Model:          opts.Model,
		Version:        opts.Version,
		Messages:       make([]*TranscriptEntry, 0),
		MessageTree:    make(map[string]*TranscriptEntry),
		MaxTokens:      opts.MaxTokens,
		CompactPercent: 0.95,
		PermissionMode: tool.PermissionModeDefault,
	}
}

// SessionOptions holds options for creating a session
type SessionOptions struct {
	ProjectPath string
	CWD         string
	Model       string
	Version     string
	MaxTokens   int
}

// AddEntry adds a new entry to the session
func (s *Session) AddEntry(entry *TranscriptEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate UUID if not set
	if entry.UUID == "" {
		entry.UUID = uuid.New().String()
	}

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Set parent UUID
	if s.CurrentUUID != "" && entry.ParentUUID == nil {
		parent := s.CurrentUUID
		entry.ParentUUID = &parent
	}

	// Set session context
	entry.SessionID = s.ID
	entry.CWD = s.CWD
	entry.Version = s.Version
	entry.GitBranch = s.GitBranch
	entry.Todos = s.Todos

	s.Messages = append(s.Messages, entry)
	s.MessageTree[entry.UUID] = entry
	s.CurrentUUID = entry.UUID
}

// AddUserMessage adds a user message
func (s *Session) AddUserMessage(content string) *TranscriptEntry {
	entry := &TranscriptEntry{
		Type: EntryTypeUser,
		Message: &Message{
			Role: "user",
			Content: []provider.ContentBlock{
				&provider.TextBlock{Text: content},
			},
		},
	}

	// Set title from first user message if not set
	if s.Title == "" {
		s.Title = truncateTitle(content, 50)
	}

	s.AddEntry(entry)
	return entry
}

// truncateTitle creates a short title from content
func truncateTitle(content string, maxLen int) string {
	// Remove newlines and extra spaces
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.Join(strings.Fields(content), " ")

	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// AddToolResult adds a tool result message
func (s *Session) AddToolResult(toolUseID string, content string, isError bool, metadata interface{}) *TranscriptEntry {
	entry := &TranscriptEntry{
		Type: EntryTypeUser,
		Message: &Message{
			Role: "user",
			Content: []provider.ContentBlock{
				&provider.ToolResultBlock{
					ToolUseID: toolUseID,
					Content:   content,
					IsError:   isError,
				},
			},
		},
		ToolUseResult: metadata,
	}

	s.AddEntry(entry)
	return entry
}

// AddAssistantMessage adds an assistant message
func (s *Session) AddAssistantMessage(resp *provider.Response) *TranscriptEntry {
	entry := &TranscriptEntry{
		Type: EntryTypeAssistant,
		Message: &Message{
			Role:       "assistant",
			Content:    resp.Content,
			ID:         resp.ID,
			Model:      resp.Model,
			StopReason: string(resp.StopReason),
			Usage:      &resp.Usage,
		},
	}

	s.AddEntry(entry)
	return entry
}

// GetMessages returns messages in API format
func (s *Session) GetMessages() []provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]provider.Message, 0, len(s.Messages))

	for _, entry := range s.Messages {
		if entry.Message == nil {
			continue
		}

		msg := provider.Message{
			Role:    provider.Role(entry.Message.Role),
			Content: entry.Message.Content,
		}

		messages = append(messages, msg)
	}

	return messages
}

// UpdateTodos updates the todo list
func (s *Session) UpdateTodos(todos []Todo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Todos = todos
}

// ShouldCompact checks if the session should be compacted
func (s *Session) ShouldCompact() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.MaxTokens == 0 {
		return false
	}

	return float64(s.TokenCount)/float64(s.MaxTokens) >= s.CompactPercent
}

// EstimateTokens estimates the token count (rough estimate)
func (s *Session) EstimateTokens() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	total := 0
	for _, entry := range s.Messages {
		if entry.Message == nil {
			continue
		}

		for _, block := range entry.Message.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				// Rough estimate: 1 token per 4 characters
				total += len(b.Text) / 4
			case *provider.ToolResultBlock:
				total += len(b.Content) / 4
			case *provider.ThinkingBlock:
				total += len(b.Thinking) / 4
			}
		}
	}

	s.TokenCount = total
	return total
}

// CompactOptions configures compaction behavior
type CompactOptions struct {
	KeepRecentMessages int     // Number of recent messages to always keep
	TargetRatio        float64 // Target token usage ratio (e.g., 0.5 = 50%)
	PreserveTodos      bool    // Keep todo-related messages
	PreserveToolCalls  bool    // Keep tool call results (summarized)
}

// DefaultCompactOptions returns sensible defaults
func DefaultCompactOptions() *CompactOptions {
	return &CompactOptions{
		KeepRecentMessages: 10,
		TargetRatio:        0.5,
		PreserveTodos:      true,
		PreserveToolCalls:  true,
	}
}

// CompactResult contains information about the compaction
type CompactResult struct {
	OriginalMessages  int
	RemainingMessages int
	OriginalTokens    int
	RemainingTokens   int
	Summary           string
}

// Compact reduces the session size by summarizing old messages
func (s *Session) Compact(opts *CompactOptions) *CompactResult {
	if opts == nil {
		opts = DefaultCompactOptions()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	originalCount := len(s.Messages)
	originalTokens := s.TokenCount

	if originalCount <= opts.KeepRecentMessages {
		return &CompactResult{
			OriginalMessages:  originalCount,
			RemainingMessages: originalCount,
			OriginalTokens:    originalTokens,
			RemainingTokens:   originalTokens,
			Summary:           "No compaction needed",
		}
	}

	// Split messages: old (to compact) and recent (to keep)
	splitPoint := originalCount - opts.KeepRecentMessages
	oldMessages := s.Messages[:splitPoint]
	recentMessages := s.Messages[splitPoint:]

	// Generate summary of old messages
	summary := s.summarizeMessages(oldMessages, opts)

	// Create a new system entry with the summary
	summaryEntry := &TranscriptEntry{
		Type:      EntryTypeSystem,
		UUID:      uuid.New().String(),
		Timestamp: time.Now(),
		SessionID: s.ID,
		Message: &Message{
			Role: "user",
			Content: []provider.ContentBlock{
				&provider.TextBlock{
					Text: "[Conversation Summary]\n" + summary,
				},
			},
		},
	}

	// Rebuild message tree
	newMessages := make([]*TranscriptEntry, 0, len(recentMessages)+1)
	newMessages = append(newMessages, summaryEntry)
	newMessages = append(newMessages, recentMessages...)

	// Update parent UUIDs
	for i, entry := range newMessages {
		if i == 0 {
			entry.ParentUUID = nil
		} else {
			parent := newMessages[i-1].UUID
			entry.ParentUUID = &parent
		}
	}

	// Rebuild message tree map
	newTree := make(map[string]*TranscriptEntry)
	for _, entry := range newMessages {
		newTree[entry.UUID] = entry
	}

	s.Messages = newMessages
	s.MessageTree = newTree
	s.CurrentUUID = newMessages[len(newMessages)-1].UUID

	// Recalculate tokens
	newTokens := s.estimateTokensUnsafe()

	return &CompactResult{
		OriginalMessages:  originalCount,
		RemainingMessages: len(newMessages),
		OriginalTokens:    originalTokens,
		RemainingTokens:   newTokens,
		Summary:           summary,
	}
}

// summarizeMessages creates a summary of messages
func (s *Session) summarizeMessages(entries []*TranscriptEntry, opts *CompactOptions) string {
	var sb strings.Builder

	sb.WriteString("Previous conversation summary:\n\n")

	// Track topics discussed
	topics := make([]string, 0)
	toolsUsed := make(map[string]int)
	keyDecisions := make([]string, 0)

	for _, entry := range entries {
		if entry.Message == nil {
			continue
		}

		for _, block := range entry.Message.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				// Extract key information from text
				text := b.Text
				if len(text) > 200 {
					// For long texts, just note the topic
					firstLine := strings.Split(text, "\n")[0]
					if len(firstLine) > 100 {
						firstLine = firstLine[:100] + "..."
					}
					topics = append(topics, firstLine)
				}
			case *provider.ToolUseBlock:
				if opts.PreserveToolCalls {
					toolsUsed[b.Name]++
				}
			case *provider.ToolResultBlock:
				// Note significant tool results
				if !b.IsError && len(b.Content) > 0 {
					// Just count, don't store full content
				}
			}
		}

		// Check for todos
		if opts.PreserveTodos && len(entry.Todos) > 0 {
			for _, todo := range entry.Todos {
				if todo.Status == "completed" {
					keyDecisions = append(keyDecisions, "✓ "+todo.Content)
				}
			}
		}
	}

	// Build summary
	if len(topics) > 0 {
		sb.WriteString("Topics discussed:\n")
		for i, topic := range topics {
			if i >= 5 { // Limit to 5 topics
				sb.WriteString("  ... and more\n")
				break
			}
			sb.WriteString("  • " + topic + "\n")
		}
		sb.WriteString("\n")
	}

	if len(toolsUsed) > 0 {
		sb.WriteString("Tools used:\n")
		for tool, count := range toolsUsed {
			sb.WriteString("  • " + tool)
			if count > 1 {
				sb.WriteString(" (×" + string(rune('0'+count)) + ")")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(keyDecisions) > 0 {
		sb.WriteString("Completed tasks:\n")
		for _, decision := range keyDecisions {
			sb.WriteString("  " + decision + "\n")
		}
	}

	return sb.String()
}

// estimateTokensUnsafe estimates tokens without locking (caller must hold lock)
func (s *Session) estimateTokensUnsafe() int {
	total := 0
	for _, entry := range s.Messages {
		if entry.Message == nil {
			continue
		}

		for _, block := range entry.Message.Content {
			switch b := block.(type) {
			case *provider.TextBlock:
				total += len(b.Text) / 4
			case *provider.ToolResultBlock:
				total += len(b.Content) / 4
			case *provider.ThinkingBlock:
				total += len(b.Thinking) / 4
			}
		}
	}

	s.TokenCount = total
	return total
}

// MarshalJSON implements json.Marshaler for TranscriptEntry
func (e *TranscriptEntry) MarshalJSON() ([]byte, error) {
	type Alias TranscriptEntry
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	})
}
