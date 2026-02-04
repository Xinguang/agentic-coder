// Package builtin provides built-in tools
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// WebSearchTool performs web searches
type WebSearchTool struct {
	client     *http.Client
	searchFunc func(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error)
}

// SearchOptions contains options for web search
type SearchOptions struct {
	AllowedDomains  []string `json:"allowed_domains,omitempty"`
	BlockedDomains  []string `json:"blocked_domains,omitempty"`
	MaxResults      int      `json:"max_results,omitempty"`
	SafeSearch      bool     `json:"safe_search,omitempty"`
}

// SearchResults contains search results
type SearchResults struct {
	Results []SearchResult `json:"results"`
	Query   string         `json:"query"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// NewWebSearchTool creates a new web search tool
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetSearchFunc sets a custom search function (for testing or custom search providers)
func (t *WebSearchTool) SetSearchFunc(fn func(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error)) {
	t.searchFunc = fn
}

func (t *WebSearchTool) Name() string {
	return "WebSearch"
}

func (t *WebSearchTool) Description() string {
	return `Search the web for information.

Usage notes:
- Provides up-to-date information for current events and recent data
- Returns search results with titles, URLs, and snippets
- Domain filtering is supported to include or block specific websites
- After answering, include a "Sources:" section with relevant URLs

IMPORTANT: Use the correct year in search queries based on today's date.`
}

func (t *WebSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query to use"
			},
			"allowed_domains": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Only include search results from these domains"
			},
			"blocked_domains": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Never include search results from these domains"
			}
		},
		"required": ["query"]
	}`)
}

func (t *WebSearchTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[webSearchParams](input.Params)
	if err != nil {
		return err
	}

	if params.Query == "" {
		return fmt.Errorf("query is required")
	}

	if len(params.Query) < 2 {
		return fmt.Errorf("query must be at least 2 characters")
	}

	return nil
}

type webSearchParams struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

func (t *WebSearchTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[webSearchParams](input.Params)
	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Error parsing parameters: %v", err), IsError: true}, nil
	}

	opts := &SearchOptions{
		AllowedDomains: params.AllowedDomains,
		BlockedDomains: params.BlockedDomains,
		MaxResults:     10,
	}

	var results *SearchResults

	// Use custom search function if provided
	if t.searchFunc != nil {
		results, err = t.searchFunc(ctx, params.Query, opts)
	} else {
		// Default: use DuckDuckGo HTML search (no API key required)
		results, err = t.searchDuckDuckGo(ctx, params.Query, opts)
	}

	if err != nil {
		return &tool.Output{Content: fmt.Sprintf("Search error: %v", err), IsError: true}, nil
	}

	// Filter results by domain
	filteredResults := t.filterResults(results.Results, opts)

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %s\n\n", params.Query))

	if len(filteredResults) == 0 {
		sb.WriteString("No results found.")
	} else {
		for i, result := range filteredResults {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, result.Title))
			sb.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
			if result.Snippet != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
			}
			sb.WriteString("\n")
		}
	}

	return &tool.Output{
		Content: sb.String(),
		Metadata: map[string]interface{}{
			"query":         params.Query,
			"result_count":  len(filteredResults),
			"results":       filteredResults,
		},
	}, nil
}

// searchDuckDuckGo performs a search using DuckDuckGo
func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error) {
	// Use DuckDuckGo HTML interface (lite version)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; agentic-coder/1.0)")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse HTML results (simple extraction)
	results := t.parseHTMLResults(string(body), opts.MaxResults)

	return &SearchResults{
		Query:   query,
		Results: results,
	}, nil
}

// parseHTMLResults extracts search results from DuckDuckGo HTML
func (t *WebSearchTool) parseHTMLResults(html string, maxResults int) []SearchResult {
	var results []SearchResult

	// Simple HTML parsing for DuckDuckGo lite results
	// Look for result links with class "result__a"
	parts := strings.Split(html, `class="result__a"`)

	for i, part := range parts[1:] {
		if i >= maxResults {
			break
		}

		// Extract href
		hrefStart := strings.Index(part, `href="`)
		if hrefStart == -1 {
			continue
		}
		hrefStart += 6
		hrefEnd := strings.Index(part[hrefStart:], `"`)
		if hrefEnd == -1 {
			continue
		}
		href := part[hrefStart : hrefStart+hrefEnd]

		// Decode DuckDuckGo redirect URL
		if strings.Contains(href, "uddg=") {
			if decoded, err := url.QueryUnescape(href); err == nil {
				if idx := strings.Index(decoded, "uddg="); idx != -1 {
					href = decoded[idx+5:]
					if ampIdx := strings.Index(href, "&"); ampIdx != -1 {
						href = href[:ampIdx]
					}
				}
			}
		}

		// Extract title (text between > and </a>)
		titleStart := strings.Index(part, ">")
		if titleStart == -1 {
			continue
		}
		titleStart++
		titleEnd := strings.Index(part[titleStart:], "</a>")
		if titleEnd == -1 {
			continue
		}
		title := strings.TrimSpace(part[titleStart : titleStart+titleEnd])
		title = stripHTMLTags(title)

		// Extract snippet
		snippet := ""
		snippetStart := strings.Index(part, `class="result__snippet"`)
		if snippetStart != -1 {
			snippetStart = strings.Index(part[snippetStart:], ">")
			if snippetStart != -1 {
				snippetStart += snippetStart + 1
				snippetEnd := strings.Index(part[snippetStart:], "</a>")
				if snippetEnd != -1 {
					snippet = strings.TrimSpace(part[snippetStart : snippetStart+snippetEnd])
					snippet = stripHTMLTags(snippet)
				}
			}
		}

		if title != "" && href != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     href,
				Snippet: snippet,
			})
		}
	}

	return results
}

// filterResults filters results by allowed/blocked domains
func (t *WebSearchTool) filterResults(results []SearchResult, opts *SearchOptions) []SearchResult {
	if len(opts.AllowedDomains) == 0 && len(opts.BlockedDomains) == 0 {
		return results
	}

	var filtered []SearchResult
	for _, result := range results {
		parsedURL, err := url.Parse(result.URL)
		if err != nil {
			continue
		}
		domain := parsedURL.Host

		// Check blocked domains
		blocked := false
		for _, blockedDomain := range opts.BlockedDomains {
			if strings.Contains(domain, blockedDomain) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		// Check allowed domains (if specified)
		if len(opts.AllowedDomains) > 0 {
			allowed := false
			for _, allowedDomain := range opts.AllowedDomains {
				if strings.Contains(domain, allowedDomain) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		filtered = append(filtered, result)
	}

	return filtered
}

// stripHTMLTags removes HTML tags from a string
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false

	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}
