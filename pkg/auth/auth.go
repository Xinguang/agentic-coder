// Package auth provides authentication for AI providers
package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	stdinReader     *bufio.Reader
	stdinReaderOnce sync.Once
)

func getReader() *bufio.Reader {
	stdinReaderOnce.Do(func() {
		stdinReader = bufio.NewReader(os.Stdin)
	})
	return stdinReader
}

// readLine reads a non-empty line from stdin
func readLine() string {
	os.Stdout.Sync()
	reader := getReader()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if line != "" {
				return strings.TrimSpace(line)
			}
			fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
			return ""
		}
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
		// Skip empty lines, continue reading
	}
}

// AuthType represents the authentication type
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
)

// Provider represents an auth provider
type Provider string

const (
	ProviderClaude  Provider = "claude"
	ProviderGemini  Provider = "gemini"
	ProviderOpenAI  Provider = "openai"
)

// Credentials holds authentication credentials
type Credentials struct {
	Provider     Provider  `json:"provider"`
	AuthType     AuthType  `json:"auth_type"`
	APIKey       string    `json:"api_key,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	Email        string    `json:"email,omitempty"`
}

// IsExpired checks if the credentials are expired
func (c *Credentials) IsExpired() bool {
	if c.AuthType == AuthTypeAPIKey {
		return false
	}
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt.Add(-5 * time.Minute)) // 5 min buffer
}

// Manager manages authentication for multiple providers
type Manager struct {
	mu          sync.RWMutex
	credentials map[Provider]*Credentials
	configDir   string
	handlers    map[Provider]AuthHandler
}

// AuthHandler handles provider-specific authentication
type AuthHandler interface {
	// Authenticate performs authentication (may open browser)
	Authenticate(ctx context.Context) (*Credentials, error)
	// Refresh refreshes expired credentials
	Refresh(ctx context.Context, creds *Credentials) (*Credentials, error)
	// Validate checks if credentials are valid
	Validate(ctx context.Context, creds *Credentials) error
}

// NewManager creates a new auth manager
func NewManager(configDir string) *Manager {
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config", "agentic-coder")
	}

	m := &Manager{
		credentials: make(map[Provider]*Credentials),
		configDir:   configDir,
		handlers:    make(map[Provider]AuthHandler),
	}

	// Register handlers
	m.handlers[ProviderClaude] = NewClaudeAuthHandler()
	m.handlers[ProviderGemini] = NewGeminiAuthHandler()
	m.handlers[ProviderOpenAI] = NewOpenAIAuthHandler()

	// Load saved credentials
	m.loadCredentials()

	return m
}

// GetCredentials returns credentials for a provider
func (m *Manager) GetCredentials(provider Provider) (*Credentials, error) {
	m.mu.RLock()
	creds, exists := m.credentials[provider]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no credentials for %s", provider)
	}

	// Check if refresh needed
	if creds.IsExpired() && creds.RefreshToken != "" {
		handler := m.handlers[provider]
		if handler != nil {
			newCreds, err := handler.Refresh(context.Background(), creds)
			if err == nil {
				m.SetCredentials(provider, newCreds)
				return newCreds, nil
			}
		}
	}

	return creds, nil
}

// SetCredentials sets credentials for a provider
func (m *Manager) SetCredentials(provider Provider, creds *Credentials) {
	m.mu.Lock()
	m.credentials[provider] = creds
	m.mu.Unlock()
	m.saveCredentials()
}

// Authenticate performs authentication for a provider
func (m *Manager) Authenticate(ctx context.Context, provider Provider) (*Credentials, error) {
	// Check if credentials already exist
	existingCreds, err := m.GetCredentials(provider)
	if err == nil && existingCreds != nil {
		// Validate existing credentials
		handler := m.handlers[provider]
		if handler != nil {
			fmt.Printf("✓ Found existing %s credentials in config\n", provider)
			if err := handler.Validate(ctx, existingCreds); err != nil {
				fmt.Printf("⚠️  Existing credentials invalid: %v\n", err)
				fmt.Println("   Please re-authenticate...")
			} else {
				fmt.Printf("✓ Credentials valid for %s\n", provider)
				// Save updated credentials (may have new access token)
				m.SetCredentials(provider, existingCreds)
				return existingCreds, nil
			}
		}
	}

	handler, exists := m.handlers[provider]
	if !exists {
		return nil, fmt.Errorf("no auth handler for %s", provider)
	}

	creds, err := handler.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	m.SetCredentials(provider, creds)
	return creds, nil
}

// SetAPIKey sets an API key for a provider
func (m *Manager) SetAPIKey(provider Provider, apiKey string) {
	creds := &Credentials{
		Provider: provider,
		AuthType: AuthTypeAPIKey,
		APIKey:   apiKey,
	}
	m.SetCredentials(provider, creds)
}

// GetAPIKey returns API key or access token for a provider
func (m *Manager) GetAPIKey(provider Provider) (string, error) {
	creds, err := m.GetCredentials(provider)
	if err != nil {
		return "", err
	}

	if creds.APIKey != "" {
		return creds.APIKey, nil
	}
	if creds.AccessToken != "" {
		return creds.AccessToken, nil
	}

	return "", fmt.Errorf("no valid token for %s", provider)
}

// Logout removes credentials for a provider
func (m *Manager) Logout(provider Provider) {
	m.mu.Lock()
	delete(m.credentials, provider)
	m.mu.Unlock()
	m.saveCredentials()
}

// ListProviders returns all authenticated providers
func (m *Manager) ListProviders() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]Provider, 0, len(m.credentials))
	for p := range m.credentials {
		providers = append(providers, p)
	}
	return providers
}

// credentialsFile returns the path to the credentials file
func (m *Manager) credentialsFile() string {
	return filepath.Join(m.configDir, "credentials.json")
}

// loadCredentials loads credentials from file
func (m *Manager) loadCredentials() {
	data, err := os.ReadFile(m.credentialsFile())
	if err != nil {
		return
	}

	var creds map[Provider]*Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return
	}

	m.mu.Lock()
	m.credentials = creds
	m.mu.Unlock()
}

// saveCredentials saves credentials to file
func (m *Manager) saveCredentials() {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.credentials, "", "  ")
	m.mu.RUnlock()

	if err != nil {
		return
	}

	os.MkdirAll(m.configDir, 0700)
	os.WriteFile(m.credentialsFile(), data, 0600)
}

// OAuthConfig holds OAuth2 configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
}

// OAuthServer handles the OAuth callback
type OAuthServer struct {
	server   *http.Server
	codeChan chan string
	errChan  chan error
	port     int
}

// NewOAuthServer creates a new OAuth callback server
func NewOAuthServer(port int) *OAuthServer {
	return &OAuthServer{
		codeChan: make(chan string, 1),
		errChan:  make(chan error, 1),
		port:     port,
	}
}

// Start starts the OAuth callback server
func (s *OAuthServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			s.errChan <- err
		}
	}()

	return nil
}

// WaitForCode waits for the OAuth code
func (s *OAuthServer) WaitForCode(ctx context.Context) (string, error) {
	select {
	case code := <-s.codeChan:
		return code, nil
	case err := <-s.errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Stop stops the OAuth server
func (s *OAuthServer) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "no code received"
		}
		s.errChan <- fmt.Errorf("OAuth error: %s", errMsg)
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	s.codeChan <- code

	// Show success page
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body style="font-family: system-ui; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5;">
<div style="text-align: center; padding: 40px; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1);">
<h1 style="color: #22c55e;">✓ Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
</div>
</body>
</html>
`)
}

func (s *OAuthServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head><title>Agentic Coder Auth</title></head>
<body style="font-family: system-ui; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0;">
<div style="text-align: center;">
<h1>Agentic Coder Authentication</h1>
<p>Waiting for authentication...</p>
</div>
</body>
</html>
`)
}
