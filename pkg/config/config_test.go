package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultModel != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %s", cfg.DefaultModel)
	}
	if cfg.MaxTokens != 200000 {
		t.Errorf("expected max tokens 200000, got %d", cfg.MaxTokens)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected log level 'info', got %s", cfg.LogLevel)
	}
}

func TestConfigValidate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	result := cfg.Validate()

	if !result.IsValid() {
		t.Errorf("default config should be valid, got errors: %v", result.Errors)
	}
}

func TestConfigValidate_InvalidTemperature(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Temperature = 3.0

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("config with temperature > 2 should be invalid")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "temperature" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected temperature validation error")
	}
}

func TestConfigValidate_InvalidMaxTokens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTokens = -1

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("config with negative max_tokens should be invalid")
	}
}

func TestConfigValidate_InvalidThinkingLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ThinkingLevel = "invalid"

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("config with invalid thinking_level should be invalid")
	}
}

func TestConfigValidate_InvalidPermissionMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionMode = "invalid"

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("config with invalid permission_mode should be invalid")
	}
}

func TestConfigValidate_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogLevel = "invalid"

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("config with invalid log_level should be invalid")
	}
}

func TestConfigValidate_UnknownModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultModel = "unknown-model-xyz"

	result := cfg.Validate()

	// Should produce warning, not error
	if !result.IsValid() {
		t.Error("unknown model should be valid (with warning)")
	}
	if !result.HasWarnings() {
		t.Error("expected warning for unknown model")
	}
}

func TestConfigValidate_HookValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Hooks = []HookConfig{
		{Event: "", Command: "echo test"}, // Missing event
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("hook without event should be invalid")
	}
}

func TestConfigValidate_MCPServerValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCPServers = []MCPServerConfig{
		{Name: "", Type: "stdio", Command: "test"}, // Missing name
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("MCP server without name should be invalid")
	}
}

func TestConfigValidate_MCPServerStdioWithoutCommand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCPServers = []MCPServerConfig{
		{Name: "test", Type: "stdio", Command: ""}, // Missing command
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("stdio MCP server without command should be invalid")
	}
}

func TestConfigValidate_MCPServerSSEWithoutURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCPServers = []MCPServerConfig{
		{Name: "test", Type: "sse", URL: ""}, // Missing URL
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("sse MCP server without URL should be invalid")
	}
}

func TestConfigValidate_LargeMaxTokensWarning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTokens = 2000000

	result := cfg.Validate()

	if !result.HasWarnings() {
		t.Error("expected warning for very large max_tokens")
	}
}

func TestConfigSet(t *testing.T) {
	cfg := DefaultConfig()

	cfg.Set("default_model", "opus")
	if cfg.DefaultModel != "opus" {
		t.Errorf("expected 'opus', got %s", cfg.DefaultModel)
	}

	cfg.Set("max_tokens", 100000)
	if cfg.MaxTokens != 100000 {
		t.Errorf("expected 100000, got %d", cfg.MaxTokens)
	}

	cfg.Set("verbose", true)
	if !cfg.Verbose {
		t.Error("expected verbose to be true")
	}
}

func TestConfigGetString(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultModel = "test"

	if cfg.GetString("default_model") != "test" {
		t.Error("GetString failed")
	}
}

func TestConfigGetInt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTokens = 12345

	if cfg.GetInt("max_tokens") != 12345 {
		t.Error("GetInt failed")
	}
}

func TestConfigGetBool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Verbose = true

	if !cfg.GetBool("verbose") {
		t.Error("GetBool failed")
	}
}

func TestConfigAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetAPIKey("test_provider", "test_key")

	if cfg.GetAPIKey("test_provider") != "test_key" {
		t.Error("SetAPIKey/GetAPIKey failed")
	}
}

func TestValidationResult(t *testing.T) {
	result := &ValidationResult{}

	if !result.IsValid() {
		t.Error("empty result should be valid")
	}
	if result.HasWarnings() {
		t.Error("empty result should have no warnings")
	}

	result.Errors = append(result.Errors, ValidationError{Field: "test", Message: "error"})
	if result.IsValid() {
		t.Error("result with errors should not be valid")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test_field",
		Value:   "test_value",
		Message: "test message",
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("error string should not be empty")
	}
}
