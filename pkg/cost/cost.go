// Package cost provides token usage and cost tracking
package cost

import (
	"sync"
)

// Pricing defines the cost per million tokens for a model
type Pricing struct {
	InputPer1M  float64 // Cost per 1M input tokens in USD
	OutputPer1M float64 // Cost per 1M output tokens in USD
}

// ProviderPricing contains pricing for known models
var ProviderPricing = map[string]Pricing{
	// Claude models
	"claude-sonnet-4-20250514":    {InputPer1M: 3.0, OutputPer1M: 15.0},
	"claude-3-5-sonnet-20241022":  {InputPer1M: 3.0, OutputPer1M: 15.0},
	"claude-3-5-haiku-20241022":   {InputPer1M: 0.8, OutputPer1M: 4.0},
	"claude-3-opus-20240229":      {InputPer1M: 15.0, OutputPer1M: 75.0},
	"claude-opus-4-20250514":      {InputPer1M: 15.0, OutputPer1M: 75.0},

	// OpenAI models
	"gpt-4o":      {InputPer1M: 5.0, OutputPer1M: 15.0},
	"gpt-4o-mini": {InputPer1M: 0.15, OutputPer1M: 0.6},
	"gpt-4-turbo": {InputPer1M: 10.0, OutputPer1M: 30.0},
	"o1":          {InputPer1M: 15.0, OutputPer1M: 60.0},
	"o1-mini":     {InputPer1M: 3.0, OutputPer1M: 12.0},
	"o3-mini":     {InputPer1M: 1.1, OutputPer1M: 4.4},

	// Gemini models
	"gemini-2.0-flash": {InputPer1M: 0.1, OutputPer1M: 0.4},
	"gemini-1.5-pro":   {InputPer1M: 1.25, OutputPer1M: 5.0},
	"gemini-1.5-flash": {InputPer1M: 0.075, OutputPer1M: 0.3},

	// DeepSeek models
	"deepseek-chat":     {InputPer1M: 0.14, OutputPer1M: 0.28},
	"deepseek-reasoner": {InputPer1M: 0.55, OutputPer1M: 2.19},

	// Ollama (free, local)
	"llama3.2": {InputPer1M: 0, OutputPer1M: 0},
	"qwen2.5":  {InputPer1M: 0, OutputPer1M: 0},
}

// Tracker tracks token usage and costs
type Tracker struct {
	InputTokens  int64
	OutputTokens int64
	model        string
	mu           sync.Mutex
}

// NewTracker creates a new cost tracker
func NewTracker(model string) *Tracker {
	return &Tracker{
		model: model,
	}
}

// SetModel updates the model for cost calculation
func (t *Tracker) SetModel(model string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.model = model
}

// AddUsage adds token usage
func (t *Tracker) AddUsage(inputTokens, outputTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.InputTokens += int64(inputTokens)
	t.OutputTokens += int64(outputTokens)
}

// GetCost calculates the total cost in USD
func (t *Tracker) GetCost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calculateCost()
}

// calculateCost calculates cost (caller must hold lock)
func (t *Tracker) calculateCost() float64 {
	pricing, ok := ProviderPricing[t.model]
	if !ok {
		// Try to find a matching model by prefix
		pricing = t.findPricingByPrefix()
	}

	inputCost := float64(t.InputTokens) * pricing.InputPer1M / 1_000_000
	outputCost := float64(t.OutputTokens) * pricing.OutputPer1M / 1_000_000
	return inputCost + outputCost
}

// findPricingByPrefix tries to match model by common prefixes
func (t *Tracker) findPricingByPrefix() Pricing {
	model := t.model

	// Claude models
	if len(model) > 6 && model[:6] == "claude" {
		if len(model) > 12 && model[7:13] == "sonnet" {
			return ProviderPricing["claude-sonnet-4-20250514"]
		}
		if len(model) > 11 && model[7:12] == "haiku" {
			return ProviderPricing["claude-3-5-haiku-20241022"]
		}
		if len(model) > 10 && model[7:11] == "opus" {
			return ProviderPricing["claude-3-opus-20240229"]
		}
	}

	// GPT models
	if len(model) > 3 && model[:3] == "gpt" {
		return ProviderPricing["gpt-4o"]
	}

	// Gemini models
	if len(model) > 6 && model[:6] == "gemini" {
		return ProviderPricing["gemini-2.0-flash"]
	}

	// DeepSeek models
	if len(model) > 8 && model[:8] == "deepseek" {
		return ProviderPricing["deepseek-chat"]
	}

	// Default: return zero pricing (unknown model)
	return Pricing{}
}

// GetStats returns usage statistics
func (t *Tracker) GetStats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()

	return Stats{
		InputTokens:  t.InputTokens,
		OutputTokens: t.OutputTokens,
		TotalTokens:  t.InputTokens + t.OutputTokens,
		TotalCost:    t.calculateCost(),
		Model:        t.model,
	}
}

// Stats contains usage statistics
type Stats struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	TotalCost    float64
	Model        string
}

// Reset clears the tracker
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.InputTokens = 0
	t.OutputTokens = 0
}

// FormatCost formats cost as a string
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return "<$0.01"
	}
	return "$" + formatFloat(cost, 2)
}

// formatFloat formats a float with specified decimal places
func formatFloat(f float64, decimals int) string {
	// Simple formatting without using fmt.Sprintf for performance
	intPart := int64(f)
	fracPart := int64((f - float64(intPart)) * 100)
	if fracPart < 0 {
		fracPart = -fracPart
	}

	result := make([]byte, 0, 10)
	if intPart == 0 {
		result = append(result, '0')
	} else {
		temp := make([]byte, 0, 10)
		for intPart > 0 {
			temp = append(temp, byte('0'+intPart%10))
			intPart /= 10
		}
		for i := len(temp) - 1; i >= 0; i-- {
			result = append(result, temp[i])
		}
	}

	if decimals > 0 {
		result = append(result, '.')
		if fracPart < 10 {
			result = append(result, '0')
		}
		if fracPart == 0 {
			result = append(result, '0')
		} else {
			temp := make([]byte, 0, 2)
			for fracPart > 0 {
				temp = append(temp, byte('0'+fracPart%10))
				fracPart /= 10
			}
			for i := len(temp) - 1; i >= 0; i-- {
				result = append(result, temp[i])
			}
		}
	}

	return string(result)
}
