// Package provider contains the provider factory
package provider

import (
	"fmt"
	"os"
	"strings"
)

// ProviderType represents a provider type
type ProviderType string

const (
	ProviderTypeClaude    ProviderType = "claude"
	ProviderTypeClaudeCLI ProviderType = "claudecli"
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeCodexCLI  ProviderType = "codexcli"
	ProviderTypeGemini    ProviderType = "gemini"
	ProviderTypeGeminiCLI ProviderType = "geminicli"
	ProviderTypeDeepSeek  ProviderType = "deepseek"
	ProviderTypeOllama    ProviderType = "ollama"
)

// ProviderFactory creates providers
type ProviderFactory struct {
	// API keys for each provider
	claudeAPIKey   string
	openaiAPIKey   string
	geminiAPIKey   string
	deepseekAPIKey string

	// Custom base URLs
	claudeBaseURL   string
	openaiBaseURL   string
	geminiBaseURL   string
	deepseekBaseURL string
}

// NewProviderFactory creates a factory with keys from environment
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		claudeAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		openaiAPIKey:   os.Getenv("OPENAI_API_KEY"),
		geminiAPIKey:   os.Getenv("GOOGLE_API_KEY"),
		deepseekAPIKey: os.Getenv("DEEPSEEK_API_KEY"),
	}
}

// SetAPIKey sets the API key for a provider
func (f *ProviderFactory) SetAPIKey(provider ProviderType, key string) {
	switch provider {
	case ProviderTypeClaude:
		f.claudeAPIKey = key
	case ProviderTypeOpenAI:
		f.openaiAPIKey = key
	case ProviderTypeGemini:
		f.geminiAPIKey = key
	case ProviderTypeDeepSeek:
		f.deepseekAPIKey = key
	}
}

// SetBaseURL sets a custom base URL for a provider
func (f *ProviderFactory) SetBaseURL(provider ProviderType, url string) {
	switch provider {
	case ProviderTypeClaude:
		f.claudeBaseURL = url
	case ProviderTypeOpenAI:
		f.openaiBaseURL = url
	case ProviderTypeGemini:
		f.geminiBaseURL = url
	case ProviderTypeDeepSeek:
		f.deepseekBaseURL = url
	}
}

// Create creates a provider by type
// Note: The actual provider implementations are in subpackages.
// This returns a function to create the provider to avoid import cycles.
func (f *ProviderFactory) Create(providerType ProviderType) (CreateProviderFunc, error) {
	switch providerType {
	case ProviderTypeClaude:
		if f.claudeAPIKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return func() (AIProvider, error) {
			// Import claude package and create provider
			// This is handled by the caller to avoid import cycles
			return nil, fmt.Errorf("use claude.New() directly")
		}, nil

	case ProviderTypeOpenAI:
		if f.openaiAPIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		return func() (AIProvider, error) {
			return nil, fmt.Errorf("use openai.New() directly")
		}, nil

	case ProviderTypeGemini:
		if f.geminiAPIKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY not set")
		}
		return func() (AIProvider, error) {
			return nil, fmt.Errorf("use gemini.New() directly")
		}, nil

	case ProviderTypeDeepSeek:
		if f.deepseekAPIKey == "" {
			return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
		}
		return func() (AIProvider, error) {
			return nil, fmt.Errorf("use deepseek.New() directly")
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// CreateProviderFunc is a function that creates a provider
type CreateProviderFunc func() (AIProvider, error)

// DetectProviderFromModel determines the provider from model name
func DetectProviderFromModel(model string) ProviderType {
	model = strings.ToLower(model)

	// Claude CLI (local Claude Code)
	if model == "claudecli" || model == "claude-cli" {
		return ProviderTypeClaudeCLI
	}

	// Codex CLI (local Codex)
	if model == "codexcli" || model == "codex-cli" || model == "codex" {
		return ProviderTypeCodexCLI
	}

	// Gemini CLI (local Gemini)
	if model == "geminicli" || model == "gemini-cli" {
		return ProviderTypeGeminiCLI
	}

	// Claude models
	if strings.HasPrefix(model, "claude") ||
		model == "sonnet" || model == "opus" || model == "haiku" {
		return ProviderTypeClaude
	}

	// OpenAI models (exclude gpt-oss which is Ollama)
	if (strings.HasPrefix(model, "gpt") && !strings.HasPrefix(model, "gpt-oss")) ||
		strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4") ||
		model == "gpt4" || model == "gpt4o" || model == "gpt5" {
		return ProviderTypeOpenAI
	}

	// Gemini models
	if strings.HasPrefix(model, "gemini") {
		return ProviderTypeGemini
	}

	// DeepSeek models
	if strings.HasPrefix(model, "deepseek") ||
		model == "coder" || model == "reasoner" || model == "r1" {
		return ProviderTypeDeepSeek
	}

	// Ollama models
	if strings.HasPrefix(model, "llama") ||
		strings.HasPrefix(model, "codellama") ||
		strings.HasPrefix(model, "mistral") ||
		strings.HasPrefix(model, "mixtral") ||
		strings.HasPrefix(model, "phi") ||
		strings.HasPrefix(model, "qwen") ||
		strings.HasPrefix(model, "gemma") ||
		strings.HasPrefix(model, "gpt-oss") ||
		model == "ollama" {
		return ProviderTypeOllama
	}

	// Default to Claude
	return ProviderTypeClaude
}

// GetAPIKey returns the API key for a provider
func (f *ProviderFactory) GetAPIKey(provider ProviderType) string {
	switch provider {
	case ProviderTypeClaude:
		return f.claudeAPIKey
	case ProviderTypeOpenAI:
		return f.openaiAPIKey
	case ProviderTypeGemini:
		return f.geminiAPIKey
	case ProviderTypeDeepSeek:
		return f.deepseekAPIKey
	default:
		return ""
	}
}

// GetBaseURL returns the base URL for a provider
func (f *ProviderFactory) GetBaseURL(provider ProviderType) string {
	switch provider {
	case ProviderTypeClaude:
		return f.claudeBaseURL
	case ProviderTypeOpenAI:
		return f.openaiBaseURL
	case ProviderTypeGemini:
		return f.geminiBaseURL
	case ProviderTypeDeepSeek:
		return f.deepseekBaseURL
	default:
		return ""
	}
}

// AvailableProviders returns providers that have API keys configured
func (f *ProviderFactory) AvailableProviders() []ProviderType {
	var available []ProviderType

	if f.claudeAPIKey != "" {
		available = append(available, ProviderTypeClaude)
	}
	if f.openaiAPIKey != "" {
		available = append(available, ProviderTypeOpenAI)
	}
	if f.geminiAPIKey != "" {
		available = append(available, ProviderTypeGemini)
	}
	if f.deepseekAPIKey != "" {
		available = append(available, ProviderTypeDeepSeek)
	}

	return available
}
