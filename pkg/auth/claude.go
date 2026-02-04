// Package auth provides authentication for AI providers
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	claudeAPIBaseURL = "https://api.anthropic.com"
)

// ClaudeAuthHandler handles Claude authentication
type ClaudeAuthHandler struct {
	httpClient *http.Client
}

// NewClaudeAuthHandler creates a new Claude auth handler
func NewClaudeAuthHandler() *ClaudeAuthHandler {
	return &ClaudeAuthHandler{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Authenticate prompts for API key
func (h *ClaudeAuthHandler) Authenticate(ctx context.Context) (*Credentials, error) {
	fmt.Println("\n🔐 Claude Authentication")
	fmt.Println("========================")
	fmt.Println("\nGet your API key from: https://console.anthropic.com/settings/keys")
	fmt.Print("Enter your Anthropic API key: ")

	apiKey := readLine()

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	creds := &Credentials{
		Provider: ProviderClaude,
		AuthType: AuthTypeAPIKey,
		APIKey:   apiKey,
	}

	// Validate
	if err := h.Validate(ctx, creds); err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	fmt.Println("✅ Authentication successful!")
	return creds, nil
}

// Refresh is not applicable for API keys
func (h *ClaudeAuthHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("API keys cannot be refreshed")
}

// Validate checks if credentials are valid
func (h *ClaudeAuthHandler) Validate(ctx context.Context, creds *Credentials) error {
	if creds.APIKey == "" {
		return fmt.Errorf("API key is empty")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", claudeAPIBaseURL+"/v1/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-api-key", creds.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}
