// Package storage provides persistent storage abstraction
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Storage is the interface for persistent storage
type Storage interface {
	// Get retrieves a value by key
	Get(key string) ([]byte, error)

	// Set stores a value by key
	Set(key string, value []byte) error

	// Delete removes a value by key
	Delete(key string) error

	// List returns all keys with a prefix
	List(prefix string) ([]string, error)

	// Close closes the storage
	Close() error
}

// FileStorage implements Storage using the filesystem
type FileStorage struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(basePath string) (*FileStorage, error) {
	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &FileStorage{
		basePath: basePath,
	}, nil
}

// Get retrieves a value by key
func (s *FileStorage) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.keyToPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, err
	}

	return data, nil
}

// Set stores a value by key
func (s *FileStorage) Set(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyToPath(key)

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first, then rename (atomic)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, value, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// Delete removes a value by key
func (s *FileStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyToPath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// List returns all keys with a prefix
func (s *FileStorage) List(prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	prefixPath := filepath.Join(s.basePath, prefix)

	err := filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if !info.IsDir() {
			relPath, _ := filepath.Rel(s.basePath, path)
			keys = append(keys, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return keys, nil
}

// Close closes the storage
func (s *FileStorage) Close() error {
	return nil
}

// keyToPath converts a key to a file path
func (s *FileStorage) keyToPath(key string) string {
	return filepath.Join(s.basePath, key)
}

// MemoryStorage implements Storage using in-memory map
type MemoryStorage struct {
	data map[string][]byte
	mu   sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string][]byte),
	}
}

// Get retrieves a value by key
func (s *MemoryStorage) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, ok := s.data[key]; ok {
		return value, nil
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// Set stores a value by key
func (s *MemoryStorage) Set(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return nil
}

// Delete removes a value by key
func (s *MemoryStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// List returns all keys with a prefix
func (s *MemoryStorage) List(prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for key := range s.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Close closes the storage
func (s *MemoryStorage) Close() error {
	return nil
}

// JSONStore provides typed JSON storage
type JSONStore struct {
	storage Storage
}

// NewJSONStore creates a new JSON store
func NewJSONStore(storage Storage) *JSONStore {
	return &JSONStore{storage: storage}
}

// Get retrieves and unmarshals a value
func (s *JSONStore) Get(key string, v interface{}) error {
	data, err := s.storage.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

// Set marshals and stores a value
func (s *JSONStore) Set(key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return s.storage.Set(key, data)
}

// Delete removes a value
func (s *JSONStore) Delete(key string) error {
	return s.storage.Delete(key)
}

// SessionStore stores session data
type SessionStore struct {
	store    *JSONStore
	basePath string
}

// NewSessionStore creates a new session store
func NewSessionStore(basePath string) (*SessionStore, error) {
	storage, err := NewFileStorage(basePath)
	if err != nil {
		return nil, err
	}

	return &SessionStore{
		store:    NewJSONStore(storage),
		basePath: basePath,
	}, nil
}

// SessionMetadata represents session metadata
type SessionMetadata struct {
	ID          string    `json:"id"`
	ProjectPath string    `json:"project_path"`
	Model       string    `json:"model"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MessageCount int      `json:"message_count"`
}

// SaveSession saves a session
func (s *SessionStore) SaveSession(id string, data interface{}) error {
	key := filepath.Join("sessions", id+".json")
	return s.store.Set(key, data)
}

// LoadSession loads a session
func (s *SessionStore) LoadSession(id string, data interface{}) error {
	key := filepath.Join("sessions", id+".json")
	return s.store.Get(key, data)
}

// ListSessions lists all sessions
func (s *SessionStore) ListSessions() ([]SessionMetadata, error) {
	storage := s.store.storage
	keys, err := storage.List("sessions")
	if err != nil {
		return nil, err
	}

	var sessions []SessionMetadata
	for _, key := range keys {
		var meta SessionMetadata
		if err := s.store.Get(key, &meta); err != nil {
			continue
		}
		sessions = append(sessions, meta)
	}

	return sessions, nil
}

// DeleteSession deletes a session
func (s *SessionStore) DeleteSession(id string) error {
	key := filepath.Join("sessions", id+".json")
	return s.store.Delete(key)
}

// CacheStore provides caching with TTL
type CacheStore struct {
	storage Storage
	ttl     time.Duration
	mu      sync.RWMutex
	expiry  map[string]time.Time
}

// NewCacheStore creates a new cache store
func NewCacheStore(storage Storage, ttl time.Duration) *CacheStore {
	return &CacheStore{
		storage: storage,
		ttl:     ttl,
		expiry:  make(map[string]time.Time),
	}
}

// Get retrieves a cached value
func (c *CacheStore) Get(key string) ([]byte, error) {
	c.mu.RLock()
	expiry, exists := c.expiry[key]
	c.mu.RUnlock()

	if exists && time.Now().After(expiry) {
		c.Delete(key)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	return c.storage.Get(key)
}

// Set stores a cached value
func (c *CacheStore) Set(key string, value []byte) error {
	c.mu.Lock()
	c.expiry[key] = time.Now().Add(c.ttl)
	c.mu.Unlock()

	return c.storage.Set(key, value)
}

// SetWithTTL stores a cached value with custom TTL
func (c *CacheStore) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	c.expiry[key] = time.Now().Add(ttl)
	c.mu.Unlock()

	return c.storage.Set(key, value)
}

// Delete removes a cached value
func (c *CacheStore) Delete(key string) error {
	c.mu.Lock()
	delete(c.expiry, key)
	c.mu.Unlock()

	return c.storage.Delete(key)
}

// List returns all keys with a prefix
func (c *CacheStore) List(prefix string) ([]string, error) {
	return c.storage.List(prefix)
}

// Close closes the cache
func (c *CacheStore) Close() error {
	return c.storage.Close()
}

// Cleanup removes expired entries
func (c *CacheStore) Cleanup() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, expiry := range c.expiry {
		if now.After(expiry) {
			c.storage.Delete(key)
			delete(c.expiry, key)
		}
	}

	return nil
}

// GetDefaultStoragePath returns the default storage path
func GetDefaultStoragePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".agentic-coder", "data"), nil
}

// GetProjectStoragePath returns the project storage path
func GetProjectStoragePath(projectPath string) string {
	return filepath.Join(projectPath, ".agentic-coder")
}
