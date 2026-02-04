// Package session manages conversation sessions and transcripts
package session

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/xinguang/agentic-coder/pkg/provider"
	"github.com/xinguang/agentic-coder/pkg/tool"
	"github.com/google/uuid"
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

	s.AddEntry(entry)
	return entry
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

// MarshalJSON implements json.Marshaler for TranscriptEntry
func (e *TranscriptEntry) MarshalJSON() ([]byte, error) {
	type Alias TranscriptEntry
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	})
}
