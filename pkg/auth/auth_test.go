package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCredentials(t *testing.T) {
	creds := &Credentials{
		Provider:  ProviderClaude,
		AuthType:  AuthTypeAPIKey,
		APIKey:    "test-key",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if creds.IsExpired() {
		t.Error("Credentials should not be expired")
	}

	expiredCreds := &Credentials{
		Provider:  ProviderClaude,
		AuthType:  AuthTypeOAuth,
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	if !expiredCreds.IsExpired() {
		t.Error("Credentials should be expired")
	}
}

func TestManager(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".config", "agentic-coder")

	mgr := NewManager(configPath)

	// Test saving and loading credentials
	creds := &Credentials{
		Provider: ProviderClaude,
		AuthType: AuthTypeAPIKey,
		APIKey:   "test-api-key",
	}

	mgr.SetCredentials(ProviderClaude, creds)

	loaded, err := mgr.GetCredentials(ProviderClaude)
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}

	if loaded.APIKey != creds.APIKey {
		t.Errorf("APIKey mismatch: got %s, want %s", loaded.APIKey, creds.APIKey)
	}

	// Test listing providers
	providers := mgr.ListProviders()
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	// Test logout
	mgr.Logout(ProviderClaude)
	_, err = mgr.GetCredentials(ProviderClaude)
	if err == nil {
		t.Error("Expected error after logout")
	}
}

func TestOAuthServer(t *testing.T) {
	server := NewOAuthServer(0) // Use random port

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Server should be running
	if server.server == nil {
		t.Error("Server should not be nil after start")
	}
}

func TestManagerWithEmptyPath(t *testing.T) {
	// Should not panic with empty path
	mgr := NewManager("")
	if mgr == nil {
		t.Error("Manager should not be nil")
	}

	// ListProviders should work (empty result for new manager)
	providers := mgr.ListProviders()
	// Note: May not be empty if user has saved credentials
	_ = providers
}
