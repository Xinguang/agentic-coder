package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ReviewRecord stores a single review event
type ReviewRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
	Cycle        int       `json:"cycle"`
	Passed       bool      `json:"passed"`
	Issues       string    `json:"issues,omitempty"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Duration     int64     `json:"duration_ms"`
}

// ReviewHistory manages review history storage and analysis
type ReviewHistory struct {
	baseDir string
	records []ReviewRecord
	mu      sync.RWMutex
}

// NewReviewHistory creates a new review history manager
func NewReviewHistory(appDir string) (*ReviewHistory, error) {
	historyDir := filepath.Join(appDir, "review_history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create review history dir: %w", err)
	}

	h := &ReviewHistory{
		baseDir: historyDir,
		records: make([]ReviewRecord, 0),
	}

	// Load existing records
	h.loadRecords()

	return h, nil
}

// Record adds a review record
func (h *ReviewHistory) Record(sessionID string, cycle int, result *ReviewResult, durationMs int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	record := ReviewRecord{
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		Cycle:        cycle,
		Passed:       result.Passed,
		Issues:       result.Issues,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		Duration:     durationMs,
	}

	h.records = append(h.records, record)

	// Persist to file
	h.saveRecord(record)
}

// GetStats returns review statistics
func (h *ReviewHistory) GetStats() *ReviewStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := &ReviewStats{
		TotalReviews: len(h.records),
		CommonIssues: make(map[string]int),
	}

	if len(h.records) == 0 {
		return stats
	}

	var totalTokens int
	var totalDuration int64

	for _, r := range h.records {
		if r.Passed {
			stats.PassedCount++
		} else {
			stats.FailedCount++
			// Track common issues (simplified - just count non-empty issues)
			if r.Issues != "" {
				stats.CommonIssues[r.Issues]++
			}
		}
		totalTokens += r.InputTokens + r.OutputTokens
		totalDuration += r.Duration
	}

	stats.PassRate = float64(stats.PassedCount) / float64(stats.TotalReviews) * 100
	stats.AvgTokensPerReview = totalTokens / len(h.records)
	stats.AvgDurationMs = totalDuration / int64(len(h.records))

	return stats
}

// ReviewStats contains aggregated review statistics
type ReviewStats struct {
	TotalReviews       int            `json:"total_reviews"`
	PassedCount        int            `json:"passed_count"`
	FailedCount        int            `json:"failed_count"`
	PassRate           float64        `json:"pass_rate"`
	AvgTokensPerReview int            `json:"avg_tokens_per_review"`
	AvgDurationMs      int64          `json:"avg_duration_ms"`
	CommonIssues       map[string]int `json:"common_issues"`
}

// loadRecords loads existing records from file
func (h *ReviewHistory) loadRecords() {
	historyFile := filepath.Join(h.baseDir, "history.jsonl")
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return // File doesn't exist yet
	}

	// Parse JSONL
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	for decoder.More() {
		var record ReviewRecord
		if err := decoder.Decode(&record); err != nil {
			break
		}
		h.records = append(h.records, record)
	}
}

// saveRecord appends a record to the history file atomically
func (h *ReviewHistory) saveRecord(record ReviewRecord) {
	historyFile := filepath.Join(h.baseDir, "history.jsonl")

	// Marshal first to avoid partial writes
	data, err := json.Marshal(record)
	if err != nil {
		return
	}

	// Append newline to create complete line
	data = append(data, '\n')

	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// Single write call ensures atomicity for small writes (< PIPE_BUF on most systems)
	if _, err := f.Write(data); err != nil {
		return
	}

	// Sync to ensure data is flushed to disk
	_ = f.Sync()
}

// ClearHistory clears all review history
func (h *ReviewHistory) ClearHistory() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = make([]ReviewRecord, 0)

	historyFile := filepath.Join(h.baseDir, "history.jsonl")
	return os.Remove(historyFile)
}
