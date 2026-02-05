package cost

import (
	"testing"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tracker.model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", tracker.model)
	}
}

func TestTrackerAddUsage(t *testing.T) {
	tracker := NewTracker("gpt-4o")

	tracker.AddUsage(1000, 500)
	tracker.AddUsage(2000, 1000)

	stats := tracker.GetStats()
	if stats.InputTokens != 3000 {
		t.Errorf("expected 3000 input tokens, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 1500 {
		t.Errorf("expected 1500 output tokens, got %d", stats.OutputTokens)
	}
}

func TestTrackerGetCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		wantMinCost  float64
		wantMaxCost  float64
	}{
		{
			name:         "claude sonnet",
			model:        "claude-sonnet-4-20250514",
			inputTokens:  1000000,
			outputTokens: 100000,
			wantMinCost:  4.0, // 3.0 + 1.5
			wantMaxCost:  5.0,
		},
		{
			name:         "gpt-4o",
			model:        "gpt-4o",
			inputTokens:  1000000,
			outputTokens: 100000,
			wantMinCost:  6.0, // 5.0 + 1.5
			wantMaxCost:  7.0,
		},
		{
			name:         "ollama free",
			model:        "llama3.2",
			inputTokens:  1000000,
			outputTokens: 1000000,
			wantMinCost:  0,
			wantMaxCost:  0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker(tt.model)
			tracker.AddUsage(tt.inputTokens, tt.outputTokens)

			cost := tracker.GetCost()
			if cost < tt.wantMinCost || cost > tt.wantMaxCost {
				t.Errorf("cost %f not in range [%f, %f]", cost, tt.wantMinCost, tt.wantMaxCost)
			}
		})
	}
}

func TestTrackerReset(t *testing.T) {
	tracker := NewTracker("gpt-4o")
	tracker.AddUsage(1000, 500)
	tracker.Reset()

	stats := tracker.GetStats()
	if stats.InputTokens != 0 {
		t.Errorf("expected 0 input tokens after reset, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 0 {
		t.Errorf("expected 0 output tokens after reset, got %d", stats.OutputTokens)
	}
}

func TestTrackerSetModel(t *testing.T) {
	tracker := NewTracker("gpt-4o")
	tracker.SetModel("claude-sonnet-4-20250514")

	stats := tracker.GetStats()
	if stats.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", stats.Model)
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0.001, "<$0.01"},
		{0.01, "$0.01"},
		{1.23, "$1.23"},
		{10.50, "$10.50"},
	}

	for _, tt := range tests {
		got := FormatCost(tt.cost)
		if got != tt.want {
			t.Errorf("FormatCost(%f) = %s, want %s", tt.cost, got, tt.want)
		}
	}
}

func TestFindPricingByPrefix(t *testing.T) {
	tests := []struct {
		model    string
		wantZero bool
	}{
		{"claude-sonnet-new-model", false},
		{"gpt-5-turbo", false},
		{"gemini-3.0-ultra", false},
		{"deepseek-coder", false},
		{"unknown-model", true},
	}

	for _, tt := range tests {
		tracker := NewTracker(tt.model)
		tracker.AddUsage(1000000, 0)
		cost := tracker.GetCost()

		if tt.wantZero && cost != 0 {
			t.Errorf("model %s: expected zero cost, got %f", tt.model, cost)
		}
		if !tt.wantZero && cost == 0 {
			t.Errorf("model %s: expected non-zero cost", tt.model)
		}
	}
}

func TestProviderPricing(t *testing.T) {
	// Ensure all known models have pricing
	knownModels := []string{
		"claude-sonnet-4-20250514",
		"gpt-4o",
		"gemini-2.0-flash",
		"deepseek-chat",
		"llama3.2",
	}

	for _, model := range knownModels {
		if _, ok := ProviderPricing[model]; !ok {
			t.Errorf("missing pricing for known model: %s", model)
		}
	}
}
