package signal

import (
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading/provider"
)

// SignalType represents the type of trading signal
type SignalType string

const (
	SignalBuy  SignalType = "BUY"
	SignalSell SignalType = "SELL"
	SignalHold SignalType = "HOLD"
)

// Signal represents a trading signal
type Signal struct {
	Symbol    string                `json:"symbol"`
	Type      SignalType            `json:"type"`
	Price     float64               `json:"price"`
	Timestamp time.Time             `json:"timestamp"`
	Reason    string                `json:"reason"`
	Strategy  string                `json:"strategy"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`

	// Suggested execution time
	ExecuteAt time.Time `json:"execute_at"`

	// Stop loss and take profit levels
	StopLoss   *float64 `json:"stop_loss,omitempty"`
	TakeProfit *float64 `json:"take_profit,omitempty"`

	// Position size suggestion (percentage of portfolio)
	PositionSize *float64 `json:"position_size,omitempty"`
}

// Context represents the market context for signal generation
type Context struct {
	Symbol        string
	CurrentQuote  *provider.Quote
	HistoricalData []provider.OHLCV
	Timestamp     time.Time
}

// String returns a human-readable string representation
func (s *Signal) String() string {
	return s.Reason
}
