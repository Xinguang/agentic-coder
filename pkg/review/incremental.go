package review

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
)

// IncrementalReviewer supports reviewing only changed content
type IncrementalReviewer struct {
	cache    map[string]CacheEntry
	reviewer *Reviewer
	mu       sync.RWMutex
}

// CacheEntry stores a previously reviewed content hash and result
type CacheEntry struct {
	Hash     string
	Passed   bool
	Issues   string
	Feedback string
}

// NewIncrementalReviewer creates a new incremental reviewer
func NewIncrementalReviewer(reviewer *Reviewer) *IncrementalReviewer {
	return &IncrementalReviewer{
		cache:    make(map[string]CacheEntry),
		reviewer: reviewer,
	}
}

// ExtractCodeBlocks extracts code blocks from a response
func ExtractCodeBlocks(response string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(response, "\n")

	var currentBlock *CodeBlock
	var content strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if currentBlock == nil {
				// Start of code block
				lang := strings.TrimPrefix(line, "```")
				lang = strings.TrimSpace(lang)
				currentBlock = &CodeBlock{
					Language: lang,
				}
				content.Reset()
			} else {
				// End of code block
				currentBlock.Content = content.String()
				currentBlock.Hash = hashContent(currentBlock.Content)
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
			}
		} else if currentBlock != nil {
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	return blocks
}

// CodeBlock represents a code block in a response
type CodeBlock struct {
	Language string `json:"language"`
	Content  string `json:"content"`
	Hash     string `json:"hash"`
	FilePath string `json:"file_path,omitempty"` // If associated with a file
}

// ReviewChanges reviews only the changed code blocks
func (ir *IncrementalReviewer) ReviewChanges(oldResponse, newResponse string) (*IncrementalResult, error) {
	oldBlocks := ExtractCodeBlocks(oldResponse)
	newBlocks := ExtractCodeBlocks(newResponse)

	result := &IncrementalResult{
		TotalBlocks:     len(newBlocks),
		ChangedBlocks:   0,
		UnchangedBlocks: 0,
		NewBlocks:       0,
		ChangedContent:  make([]CodeBlock, 0),
	}

	// Build hash map of old blocks
	oldHashes := make(map[string]bool)
	for _, block := range oldBlocks {
		oldHashes[block.Hash] = true
	}

	// Find changed and new blocks
	for _, block := range newBlocks {
		if oldHashes[block.Hash] {
			result.UnchangedBlocks++
		} else {
			// Check if it's in cache
			ir.mu.RLock()
			_, cached := ir.cache[block.Hash]
			ir.mu.RUnlock()

			if cached {
				result.UnchangedBlocks++
			} else {
				result.ChangedBlocks++
				result.ChangedContent = append(result.ChangedContent, block)
			}
		}
	}

	// Count truly new blocks (not in old response)
	result.NewBlocks = result.ChangedBlocks

	return result, nil
}

// CacheResult caches a review result for a code block
func (ir *IncrementalReviewer) CacheResult(block CodeBlock, passed bool, issues, feedback string) {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	ir.cache[block.Hash] = CacheEntry{
		Hash:     block.Hash,
		Passed:   passed,
		Issues:   issues,
		Feedback: feedback,
	}
}

// GetCached returns a cached result if available
func (ir *IncrementalReviewer) GetCached(hash string) (*CacheEntry, bool) {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	entry, ok := ir.cache[hash]
	if ok {
		return &entry, true
	}
	return nil, false
}

// ClearCache clears the review cache
func (ir *IncrementalReviewer) ClearCache() {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.cache = make(map[string]CacheEntry)
}

// IncrementalResult contains the result of incremental review analysis
type IncrementalResult struct {
	TotalBlocks     int         `json:"total_blocks"`
	ChangedBlocks   int         `json:"changed_blocks"`
	UnchangedBlocks int         `json:"unchanged_blocks"`
	NewBlocks       int         `json:"new_blocks"`
	ChangedContent  []CodeBlock `json:"changed_content"`
}

// NeedsReview returns true if there are changes that need review
func (ir *IncrementalResult) NeedsReview() bool {
	return ir.ChangedBlocks > 0
}

// hashContent generates a SHA256 hash of content
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// DiffBlocks compares two sets of code blocks and returns the differences
func DiffBlocks(oldBlocks, newBlocks []CodeBlock) *BlockDiff {
	diff := &BlockDiff{
		Added:    make([]CodeBlock, 0),
		Removed:  make([]CodeBlock, 0),
		Modified: make([]BlockModification, 0),
	}

	oldMap := make(map[string]CodeBlock)
	for _, b := range oldBlocks {
		// Use language + position as key for rough matching
		key := b.Language + ":" + b.Hash[:8]
		oldMap[key] = b
	}

	newMap := make(map[string]CodeBlock)
	for _, b := range newBlocks {
		key := b.Language + ":" + b.Hash[:8]
		newMap[key] = b
	}

	// Find added and modified
	for key, newBlock := range newMap {
		if oldBlock, exists := oldMap[key]; exists {
			if oldBlock.Hash != newBlock.Hash {
				diff.Modified = append(diff.Modified, BlockModification{
					Old: oldBlock,
					New: newBlock,
				})
			}
		} else {
			diff.Added = append(diff.Added, newBlock)
		}
	}

	// Find removed
	for key, oldBlock := range oldMap {
		if _, exists := newMap[key]; !exists {
			diff.Removed = append(diff.Removed, oldBlock)
		}
	}

	return diff
}

// BlockDiff represents differences between two sets of code blocks
type BlockDiff struct {
	Added    []CodeBlock         `json:"added"`
	Removed  []CodeBlock         `json:"removed"`
	Modified []BlockModification `json:"modified"`
}

// BlockModification represents a modification to a code block
type BlockModification struct {
	Old CodeBlock `json:"old"`
	New CodeBlock `json:"new"`
}

// HasChanges returns true if there are any differences
func (d *BlockDiff) HasChanges() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || len(d.Modified) > 0
}
