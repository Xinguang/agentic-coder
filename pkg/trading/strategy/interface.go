package strategy

import (
	"github.com/xinguang/agentic-coder/pkg/trading"
)

// Strategy interface defines trading strategy
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// Analyze analyzes stock data and generates trading signals
	Analyze(data []*trading.StockData, positions map[string]*trading.Position) ([]*trading.TradingSignal, error)
}
