package signal

import (
	"sync"
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading"
)

// Generator generates and manages trading signals
type Generator struct {
	mu         sync.RWMutex
	signals    []*trading.TradingSignal
	positions  map[string]*trading.Position
	maxSignals int
}

// NewGenerator creates a new signal generator
func NewGenerator() *Generator {
	return &Generator{
		signals:    make([]*trading.TradingSignal, 0),
		positions:  make(map[string]*trading.Position),
		maxSignals: 1000, // keep last 1000 signals
	}
}

// AddSignals adds new signals
func (g *Generator) AddSignals(signals []*trading.TradingSignal) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.signals = append(g.signals, signals...)
	if len(g.signals) > g.maxSignals {
		g.signals = g.signals[len(g.signals)-g.maxSignals:]
	}
}

// UpdatePosition updates position information
func (g *Generator) UpdatePosition(symbol string, quantity int, avgPrice float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.positions[symbol] = &trading.Position{
		Symbol:    symbol,
		Quantity:  quantity,
		AvgPrice:  avgPrice,
		UpdatedAt: time.Now(),
	}
}

// GetPositions returns current positions
func (g *Generator) GetPositions() map[string]*trading.Position {
	g.mu.RLock()
	defer g.mu.RUnlock()

	positions := make(map[string]*trading.Position)
	for k, v := range g.positions {
		positions[k] = v
	}
	return positions
}

// GetRecentSignals returns recent signals
func (g *Generator) GetRecentSignals(limit int) []*trading.TradingSignal {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 || limit > len(g.signals) {
		limit = len(g.signals)
	}

	start := len(g.signals) - limit
	return g.signals[start:]
}

// ClearOldSignals clears signals older than specified duration
func (g *Generator) ClearOldSignals(duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	cutoff := time.Now().Add(-duration)
	filtered := make([]*trading.TradingSignal, 0)

	for _, signal := range g.signals {
		if signal.Timestamp.After(cutoff) {
			filtered = append(filtered, signal)
		}
	}

	g.signals = filtered
}
