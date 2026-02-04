package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/xinguang/agentic-coder/pkg/tool"
)

// WebFetchTool fetches and processes web content
type WebFetchTool struct {
	client     *http.Client
	maxSize    int // Max response size in bytes
	userAgent  string
}

// WebFetchInput represents the input for WebFetch tool
type WebFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

// NewWebFetchTool creates a new WebFetch tool
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		maxSize:   5 * 1024 * 1024, // 5MB
		userAgent: "agentic-coder/1.0",
	}
}

func (w *WebFetchTool) Name() string {
	return "WebFetch"
}

func (w *WebFetchTool) Description() string {
	return `Fetches content from a URL and processes it.
- Takes a URL and a prompt as input
- Fetches the URL content, converts HTML to markdown
- Processes the content with the prompt
- Returns the processed content or a summary`
}

func (w *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch content from",
				"format": "uri"
			},
			"prompt": {
				"type": "string",
				"description": "The prompt describing what information to extract"
			}
		},
		"required": ["url", "prompt"]
	}`)
}

func (w *WebFetchTool) Validate(input *tool.Input) error {
	params, err := tool.ParamsTo[WebFetchInput](input.Params)
	if err != nil {
		return err
	}

	if params.URL == "" {
		return fmt.Errorf("url is required")
	}

	// Validate URL
	u, err := url.Parse(params.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	if params.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}

	return nil
}

func (w *WebFetchTool) Execute(ctx context.Context, input *tool.Input) (*tool.Output, error) {
	params, err := tool.ParamsTo[WebFetchInput](input.Params)
	if err != nil {
		return nil, err
	}

	// Upgrade http to https
	fetchURL := params.URL
	if strings.HasPrefix(fetchURL, "http://") {
		fetchURL = "https://" + strings.TrimPrefix(fetchURL, "http://")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Failed to create request: %v", err),
			IsError: true,
		}, nil
	}

	req.Header.Set("User-Agent", w.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Send request
	resp, err := w.client.Do(req)
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Failed to fetch URL: %v", err),
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	// Check for redirect to different host
	finalURL := resp.Request.URL.String()
	originalHost, _ := url.Parse(params.URL)
	finalHost, _ := url.Parse(finalURL)
	if originalHost != nil && finalHost != nil && originalHost.Host != finalHost.Host {
		return &tool.Output{
			Content: fmt.Sprintf("Redirected to different host: %s\nPlease fetch the new URL.", finalURL),
			Metadata: map[string]interface{}{
				"redirect_url": finalURL,
			},
		}, nil
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return &tool.Output{
			Content: fmt.Sprintf("HTTP error: %d %s", resp.StatusCode, resp.Status),
			IsError: true,
		}, nil
	}

	// Read response body with size limit
	limitReader := io.LimitReader(resp.Body, int64(w.maxSize))
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return &tool.Output{
			Content: fmt.Sprintf("Failed to read response: %v", err),
			IsError: true,
		}, nil
	}

	// Convert HTML to text/markdown
	content := w.htmlToMarkdown(string(body))

	// Truncate if too long
	if len(content) > 50000 {
		content = content[:50000] + "\n\n[Content truncated...]"
	}

	return &tool.Output{
		Content: fmt.Sprintf("Fetched content from %s\n\nPrompt: %s\n\n---\n\n%s",
			fetchURL, params.Prompt, content),
		Metadata: map[string]interface{}{
			"url":          fetchURL,
			"content_type": resp.Header.Get("Content-Type"),
			"size":         len(body),
		},
	}, nil
}

// htmlToMarkdown converts HTML to readable markdown/text
func (w *WebFetchTool) htmlToMarkdown(html string) string {
	// Remove script and style tags
	html = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`).ReplaceAllString(html, "")

	// Convert common elements
	// Headings
	html = regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`).ReplaceAllString(html, "\n# $1\n")
	html = regexp.MustCompile(`(?i)<h2[^>]*>(.*?)</h2>`).ReplaceAllString(html, "\n## $1\n")
	html = regexp.MustCompile(`(?i)<h3[^>]*>(.*?)</h3>`).ReplaceAllString(html, "\n### $1\n")
	html = regexp.MustCompile(`(?i)<h4[^>]*>(.*?)</h4>`).ReplaceAllString(html, "\n#### $1\n")
	html = regexp.MustCompile(`(?i)<h5[^>]*>(.*?)</h5>`).ReplaceAllString(html, "\n##### $1\n")
	html = regexp.MustCompile(`(?i)<h6[^>]*>(.*?)</h6>`).ReplaceAllString(html, "\n###### $1\n")

	// Links
	html = regexp.MustCompile(`(?i)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`).ReplaceAllString(html, "[$2]($1)")

	// Bold and italic
	html = regexp.MustCompile(`(?i)<(strong|b)[^>]*>(.*?)</(strong|b)>`).ReplaceAllString(html, "**$2**")
	html = regexp.MustCompile(`(?i)<(em|i)[^>]*>(.*?)</(em|i)>`).ReplaceAllString(html, "*$2*")

	// Code
	html = regexp.MustCompile(`(?i)<code[^>]*>(.*?)</code>`).ReplaceAllString(html, "`$1`")
	html = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`).ReplaceAllString(html, "\n```\n$1\n```\n")

	// Lists
	html = regexp.MustCompile(`(?i)<li[^>]*>(.*?)</li>`).ReplaceAllString(html, "- $1\n")

	// Paragraphs and breaks
	html = regexp.MustCompile(`(?i)<p[^>]*>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`(?i)</p>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`(?i)<br[^>]*/?>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`(?i)<hr[^>]*/?>`).ReplaceAllString(html, "\n---\n")

	// Remove remaining HTML tags
	html = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, "")

	// Decode common HTML entities
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = strings.ReplaceAll(html, "&apos;", "'")

	// Clean up whitespace
	html = regexp.MustCompile(`\n{3,}`).ReplaceAllString(html, "\n\n")
	html = regexp.MustCompile(`[ \t]+`).ReplaceAllString(html, " ")

	return strings.TrimSpace(html)
}
