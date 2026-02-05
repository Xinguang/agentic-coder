package review

import (
	"testing"
)

func TestExtractCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		langs    []string
	}{
		{
			name:     "single go block",
			input:    "Here is code:\n```go\nfunc main() {}\n```\n",
			expected: 1,
			langs:    []string{"go"},
		},
		{
			name:     "multiple blocks",
			input:    "```python\nprint('hi')\n```\nText\n```javascript\nconsole.log('hi')\n```",
			expected: 2,
			langs:    []string{"python", "javascript"},
		},
		{
			name:     "no blocks",
			input:    "No code here",
			expected: 0,
			langs:    nil,
		},
		{
			name:     "block without language",
			input:    "```\nplain text\n```",
			expected: 1,
			langs:    []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ExtractCodeBlocks(tt.input)
			if len(blocks) != tt.expected {
				t.Errorf("expected %d blocks, got %d", tt.expected, len(blocks))
			}
			for i, lang := range tt.langs {
				if blocks[i].Language != lang {
					t.Errorf("block %d: expected lang %q, got %q", i, lang, blocks[i].Language)
				}
			}
		})
	}
}

func TestHashContent(t *testing.T) {
	// Same content should produce same hash
	hash1 := hashContent("test content")
	hash2 := hashContent("test content")
	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}

	// Different content should produce different hash
	hash3 := hashContent("different content")
	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}
}

func TestIncrementalReviewer_ReviewChanges(t *testing.T) {
	ir := NewIncrementalReviewer(nil)

	oldResponse := "```go\nfunc old() {}\n```"
	newResponse := "```go\nfunc new() {}\n```"

	result, err := ir.ReviewChanges(oldResponse, newResponse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalBlocks != 1 {
		t.Errorf("expected 1 total block, got %d", result.TotalBlocks)
	}

	if !result.NeedsReview() {
		t.Error("expected to need review for changed content")
	}
}

func TestIncrementalReviewer_UnchangedContent(t *testing.T) {
	ir := NewIncrementalReviewer(nil)

	response := "```go\nfunc same() {}\n```"

	result, err := ir.ReviewChanges(response, response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NeedsReview() {
		t.Error("should not need review for unchanged content")
	}
}

func TestIncrementalReviewer_CacheResult(t *testing.T) {
	ir := NewIncrementalReviewer(nil)

	block := CodeBlock{
		Language: "go",
		Content:  "func test() {}",
		Hash:     hashContent("func test() {}"),
	}

	ir.CacheResult(block, true, "", "")

	cached, ok := ir.GetCached(block.Hash)
	if !ok {
		t.Error("expected to find cached result")
	}
	if !cached.Passed {
		t.Error("expected cached result to be passed")
	}
}

func TestDiffBlocks(t *testing.T) {
	oldBlocks := []CodeBlock{
		{Language: "go", Content: "old1", Hash: hashContent("old1")},
		{Language: "go", Content: "same", Hash: hashContent("same")},
	}
	newBlocks := []CodeBlock{
		{Language: "go", Content: "same", Hash: hashContent("same")},
		{Language: "go", Content: "new1", Hash: hashContent("new1")},
	}

	diff := DiffBlocks(oldBlocks, newBlocks)

	if !diff.HasChanges() {
		t.Error("expected to have changes")
	}
}
