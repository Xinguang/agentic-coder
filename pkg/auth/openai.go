// Package auth provides authentication for AI providers
package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openaiAPIBaseURL = "https://api.openai.com/v1"
)

// OpenAIAuthHandler handles OpenAI authentication
type OpenAIAuthHandler struct {
	httpClient *http.Client
}

// NewOpenAIAuthHandler creates a new OpenAI auth handler
func NewOpenAIAuthHandler() *OpenAIAuthHandler {
	return &OpenAIAuthHandler{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Authenticate prompts for API key
func (h *OpenAIAuthHandler) Authenticate(ctx context.Context) (*Credentials, error) {
	fmt.Println("\n🔐 OpenAI Authentication")
	fmt.Println("========================")
	fmt.Println("\nGet your API key from: https://platform.openai.com/api-keys")
	fmt.Print("Enter your OpenAI API key: ")

	apiKey := readLine()

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	creds := &Credentials{
		Provider: ProviderOpenAI,
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
func (h *OpenAIAuthHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("API keys cannot be refreshed")
}

// Validate checks if credentials are valid
func (h *OpenAIAuthHandler) Validate(ctx context.Context, creds *Credentials) error {
	if creds.APIKey == "" {
		return fmt.Errorf("API key is empty")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", openaiAPIBaseURL+"/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+creds.APIKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("invalid credentials: %s", string(body))
	}

	return nil
}
