// Package session manages conversation sessions and transcripts
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SessionManager manages multiple sessions
type SessionManager struct {
	storage      Storage
	activeSess   map[string]*Session
	projectPath  string
	claudeDir    string // .claude directory path

	mu sync.RWMutex
}

// ManagerOptions holds options for SessionManager
type ManagerOptions struct {
	ProjectPath string
	ClaudeDir   string // defaults to ~/.claude
}

// NewSessionManager creates a new session manager
func NewSessionManager(opts *ManagerOptions) (*SessionManager, error) {
	claudeDir := opts.ClaudeDir
	if claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		claudeDir = filepath.Join(home, ".claude")
	}

	// Ensure claude directory exists
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create claude directory: %w", err)
	}

	storage, err := NewFileStorage(claudeDir, opts.ProjectPath)
	if err != nil {
		return nil, err
	}

	return &SessionManager{
		storage:     storage,
		activeSess:  make(map[string]*Session),
		projectPath: opts.ProjectPath,
		claudeDir:   claudeDir,
	}, nil
}

// NewSession creates a new session
func (m *SessionManager) NewSession(opts *SessionOptions) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := NewSession(opts)
	m.activeSess[sess.ID] = sess

	return sess, nil
}

// GetSession retrieves a session by ID
func (m *SessionManager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	if sess, ok := m.activeSess[id]; ok {
		m.mu.RUnlock()
		return sess, nil
	}
	m.mu.RUnlock()

	// Try to load from storage
	sess, err := m.storage.Load(id)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.activeSess[id] = sess
	m.mu.Unlock()

	return sess, nil
}

// SaveSession saves a session
func (m *SessionManager) SaveSession(sess *Session) error {
	return m.storage.Save(sess)
}

// ListSessions lists all sessions
func (m *SessionManager) ListSessions() ([]*SessionInfo, error) {
	return m.storage.List()
}

// DeleteSession deletes a session
func (m *SessionManager) DeleteSession(id string) error {
	m.mu.Lock()
	delete(m.activeSess, id)
	m.mu.Unlock()

	return m.storage.Delete(id)
}

// ResumeLatest resumes the latest session for the current project
func (m *SessionManager) ResumeLatest() (*Session, error) {
	sessions, err := m.storage.List()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	// Sort by last updated (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})

	return m.GetSession(sessions[0].ID)
}

// SessionInfo holds session metadata
type SessionInfo struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	ProjectPath  string    `json:"projectPath"`
	Model        string    `json:"model"`
	Created      time.Time `json:"created"`
	LastUpdated  time.Time `json:"lastUpdated"`
	MessageCount int       `json:"messageCount"`
}

// Storage interface for session persistence
type Storage interface {
	Save(sess *Session) error
	Load(id string) (*Session, error)
	List() ([]*SessionInfo, error)
	Delete(id string) error
	AppendEntry(sessionID string, entry *TranscriptEntry) error
}

// FileStorage implements Storage using file system
type FileStorage struct {
	baseDir     string
	projectDir  string
	projectPath string
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(claudeDir, projectPath string) (*FileStorage, error) {
	// Create project-specific directory
	// Hash the project path for directory name
	projectDir := filepath.Join(claudeDir, "projects", sanitizePath(projectPath))

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	return &FileStorage{
		baseDir:     claudeDir,
		projectDir:  projectDir,
		projectPath: projectPath,
	}, nil
}

// Save saves a session to file
func (s *FileStorage) Save(sess *Session) error {
	sess.mu.RLock()
	defer sess.mu.RUnlock()

	// Save session metadata
	metaPath := filepath.Join(s.projectDir, sess.ID+".meta.json")
	meta := SessionInfo{
		ID:           sess.ID,
		Title:        sess.Title,
		ProjectPath:  sess.ProjectPath,
		Model:        sess.Model,
		MessageCount: len(sess.Messages),
	}

	if len(sess.Messages) > 0 {
		meta.Created = sess.Messages[0].Timestamp
		meta.LastUpdated = sess.Messages[len(sess.Messages)-1].Timestamp
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return err
	}

	// Save transcript as JSONL
	transcriptPath := filepath.Join(s.projectDir, sess.ID+".jsonl")
	f, err := os.Create(transcriptPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range sess.Messages {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}

	return nil
}

// Load loads a session from file
func (s *FileStorage) Load(id string) (*Session, error) {
	// Load metadata
	metaPath := filepath.Join(s.projectDir, id+".meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, err
	}

	var meta SessionInfo
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, err
	}

	// Load transcript
	transcriptPath := filepath.Join(s.projectDir, id+".jsonl")
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sess := &Session{
		ID:          meta.ID,
		Title:       meta.Title,
		ProjectPath: meta.ProjectPath,
		Model:       meta.Model,
		Messages:    make([]*TranscriptEntry, 0),
		MessageTree: make(map[string]*TranscriptEntry),
	}

	decoder := json.NewDecoder(f)
	for {
		var entry TranscriptEntry
		if err := decoder.Decode(&entry); err != nil {
			break // EOF or error
		}
		sess.Messages = append(sess.Messages, &entry)
		sess.MessageTree[entry.UUID] = &entry
		sess.CurrentUUID = entry.UUID
	}

	return sess, nil
}

// List lists all sessions
func (s *FileStorage) List() ([]*SessionInfo, error) {
	entries, err := os.ReadDir(s.projectDir)
	if err != nil {
		return nil, err
	}

	var sessions []*SessionInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".meta.json") {
			metaPath := filepath.Join(s.projectDir, entry.Name())
			metaData, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}

			var meta SessionInfo
			if err := json.Unmarshal(metaData, &meta); err != nil {
				continue
			}

			sessions = append(sessions, &meta)
		}
	}

	return sessions, nil
}

// Delete deletes a session
func (s *FileStorage) Delete(id string) error {
	metaPath := filepath.Join(s.projectDir, id+".meta.json")
	transcriptPath := filepath.Join(s.projectDir, id+".jsonl")

	os.Remove(metaPath)
	os.Remove(transcriptPath)

	return nil
}

// AppendEntry appends a single entry to the transcript file
func (s *FileStorage) AppendEntry(sessionID string, entry *TranscriptEntry) error {
	transcriptPath := filepath.Join(s.projectDir, sessionID+".jsonl")

	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(entry)
}

// sanitizePath converts a file path to a safe directory name
func sanitizePath(path string) string {
	// Replace path separators and special characters
	result := strings.ReplaceAll(path, "/", "_")
	result = strings.ReplaceAll(result, "\\", "_")
	result = strings.ReplaceAll(result, ":", "_")
	result = strings.ReplaceAll(result, " ", "_")

	// Remove leading underscores
	result = strings.TrimLeft(result, "_")

	// Truncate if too long
	if len(result) > 100 {
		result = result[:100]
	}

	if result == "" {
		result = "default"
	}

	return result
}
