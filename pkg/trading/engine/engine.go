package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading"
	"github.com/xinguang/agentic-coder/pkg/trading/provider"
	"github.com/xinguang/agentic-coder/pkg/trading/signal"
	"github.com/xinguang/agentic-coder/pkg/trading/storage"
)

// Strategy interface defines trading strategy
type Strategy interface {
	Name() string
	Analyze(data []*trading.StockData, positions map[string]*trading.Position) ([]*trading.TradingSignal, error)
}

// Config holds engine configuration
type Config struct {
	Symbols       []string      // stock symbols to monitor
	UpdateInterval time.Duration // data update interval
}

// Engine is the main trading engine
type Engine struct {
	config       *Config
	provider     provider.DataProvider
	strategies   []Strategy
	generator    *signal.Generator
	storage      *storage.MemoryStorage
	mu           sync.RWMutex
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	signalChan   chan *trading.TradingSignal
}

// NewEngine creates a new trading engine
func NewEngine(config *Config, dataProvider provider.DataProvider, strategies []Strategy) *Engine {
	return &Engine{
		config:     config,
		provider:   dataProvider,
		strategies: strategies,
		generator:  signal.NewGenerator(),
		storage:    storage.NewMemoryStorage(1000, 500),
		signalChan: make(chan *trading.TradingSignal, 100),
	}
}

// Start starts the trading engine
func (e *Engine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.mu.Unlock()

	fmt.Printf("Trading engine started, monitoring %d symbols: %v\n", len(e.config.Symbols), e.config.Symbols)

	// Start data collection
	go e.collectData()

	// Start signal monitoring
	go e.monitorSignals()

	return nil
}

// Stop stops the trading engine
func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine not running")
	}
	e.running = false
	e.mu.Unlock()

	e.cancel()
	if err := e.provider.Close(); err != nil {
		return fmt.Errorf("error closing provider: %w", err)
	}

	fmt.Println("Trading engine stopped")
	return nil
}

// collectData continuously collects stock data
func (e *Engine) collectData() {
	ticker := time.NewTicker(e.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.fetchAndAnalyze()
		}
	}
}

// fetchAndAnalyze fetches data and generates signals
func (e *Engine) fetchAndAnalyze() {
	// Fetch current data
	data, err := e.provider.GetStockData(e.ctx, e.config.Symbols)
	if err != nil {
		fmt.Printf("Error fetching stock data: %v\n", err)
		return
	}

	// Save to storage
	for _, d := range data {
		if err := e.storage.SaveStockData(d); err != nil {
			fmt.Printf("Error saving stock data: %v\n", err)
		}
	}

	// Get current positions
	positions := e.generator.GetPositions()

	// Run all strategies
	allSignals := make([]*trading.TradingSignal, 0)
	for _, strat := range e.strategies {
		signals, err := strat.Analyze(data, positions)
		if err != nil {
			fmt.Printf("Error in strategy %s: %v\n", strat.Name(), err)
			continue
		}
		allSignals = append(allSignals, signals...)
	}

	// Add signals to generator
	if len(allSignals) > 0 {
		e.generator.AddSignals(allSignals)
	}

	// Process new signals
	for _, sig := range allSignals {
		// Save signal
		if err := e.storage.SaveSignal(sig); err != nil {
			fmt.Printf("Error saving signal: %v\n", err)
		}

		// Send to signal channel
		select {
		case e.signalChan <- sig:
		default:
			fmt.Println("Signal channel full, dropping signal")
		}
	}
}

// monitorSignals monitors and displays trading signals
func (e *Engine) monitorSignals() {
	for {
		select {
		case <-e.ctx.Done():
			return
		case sig := <-e.signalChan:
			e.displaySignal(sig)
		}
	}
}

// displaySignal displays a trading signal
func (e *Engine) displaySignal(sig *trading.TradingSignal) {
	separator := "================================================================================"
	fmt.Println("\n" + separator)
	fmt.Printf("🔔 NEW TRADING SIGNAL\n")
	fmt.Println(separator)
	fmt.Printf("Symbol:      %s\n", sig.Symbol)
	fmt.Printf("Action:      %s\n", sig.Type)
	fmt.Printf("Price:       $%.2f\n", sig.Price)
	fmt.Printf("Confidence:  %.1f%%\n", sig.Confidence*100)
	fmt.Printf("Generated:   %s\n", sig.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Execute At:  %s\n", sig.ExecuteAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Reason:      %s\n", sig.Reason)
	fmt.Println(separator)

	// Execution time recommendation
	now := time.Now()
	if sig.ExecuteAt.After(now) {
		duration := sig.ExecuteAt.Sub(now)
		fmt.Printf("⏰ Execute in: %s\n", formatDuration(duration))
		fmt.Printf("📅 Suggested execution time: %s (%s)\n",
			sig.ExecuteAt.Format("2006-01-02 15:04:05"),
			sig.ExecuteAt.Format("Monday"))
	} else {
		fmt.Println("⚠️  Signal execution time has passed")
	}
	fmt.Println()
}

// GetRecentSignals returns recent trading signals
func (e *Engine) GetRecentSignals(limit int) ([]*trading.TradingSignal, error) {
	return e.storage.GetSignals(limit)
}

// GetStockData returns historical stock data
func (e *Engine) GetStockData(symbol string, limit int) ([]*trading.StockData, error) {
	return e.storage.GetStockData(symbol, limit)
}

// GetPositions returns current positions
func (e *Engine) GetPositions() map[string]*trading.Position {
	return e.generator.GetPositions()
}

// UpdatePosition updates a position
func (e *Engine) UpdatePosition(symbol string, quantity int, avgPrice float64) {
	e.generator.UpdatePosition(symbol, quantity, avgPrice)
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}
