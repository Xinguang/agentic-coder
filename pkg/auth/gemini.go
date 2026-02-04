// Package auth provides authentication for AI providers
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	geminiAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// GeminiAuthHandler handles Gemini/Google authentication
type GeminiAuthHandler struct {
	httpClient *http.Client
}

// NewGeminiAuthHandler creates a new Gemini auth handler
func NewGeminiAuthHandler() *GeminiAuthHandler {
	return &GeminiAuthHandler{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Authenticate prompts for API key
func (h *GeminiAuthHandler) Authenticate(ctx context.Context) (*Credentials, error) {
	fmt.Println("\n🔐 Gemini Authentication")
	fmt.Println("========================")
	fmt.Println("\nGet your API key from: https://aistudio.google.com/app/apikey")
	fmt.Print("Enter your Gemini API key: ")

	apiKey := readLine()

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	creds := &Credentials{
		Provider: ProviderGemini,
		AuthType: AuthTypeAPIKey,
		APIKey:   apiKey,
	}

	if err := h.Validate(ctx, creds); err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	fmt.Println("✅ Authentication successful!")
	return creds, nil
}

// Refresh is not applicable for API keys
func (h *GeminiAuthHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("API keys cannot be refreshed")
}

// Validate checks if credentials are valid
func (h *GeminiAuthHandler) Validate(ctx context.Context, creds *Credentials) error {
	if creds.APIKey == "" {
		return fmt.Errorf("API key is empty")
	}

	apiURL := fmt.Sprintf("%s/models?key=%s", geminiAPIBaseURL, creds.APIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}
