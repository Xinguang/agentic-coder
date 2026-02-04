package storage

import (
	"sync"
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading"
)

// MemoryStorage is an in-memory storage for trading data
type MemoryStorage struct {
	mu            sync.RWMutex
	stockData     map[string][]*trading.StockData
	signals       []*trading.TradingSignal
	maxDataPoints int
	maxSignals    int
}

// NewMemoryStorage creates a new memory storage
func NewMemoryStorage(maxDataPoints, maxSignals int) *MemoryStorage {
	return &MemoryStorage{
		stockData:     make(map[string][]*trading.StockData),
		signals:       make([]*trading.TradingSignal, 0),
		maxDataPoints: maxDataPoints,
		maxSignals:    maxSignals,
	}
}

// SaveStockData saves stock data
func (s *MemoryStorage) SaveStockData(data *trading.StockData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.stockData[data.Symbol]; !exists {
		s.stockData[data.Symbol] = make([]*trading.StockData, 0)
	}

	s.stockData[data.Symbol] = append(s.stockData[data.Symbol], data)

	// Keep only recent data
	if len(s.stockData[data.Symbol]) > s.maxDataPoints {
		s.stockData[data.Symbol] = s.stockData[data.Symbol][len(s.stockData[data.Symbol])-s.maxDataPoints:]
	}

	return nil
}

// GetStockData retrieves stock data for a symbol
func (s *MemoryStorage) GetStockData(symbol string, limit int) ([]*trading.StockData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.stockData[symbol]
	if !exists {
		return nil, nil
	}

	if limit <= 0 || limit > len(data) {
		limit = len(data)
	}

	start := len(data) - limit
	result := make([]*trading.StockData, limit)
	copy(result, data[start:])

	return result, nil
}

// SaveSignal saves a trading signal
func (s *MemoryStorage) SaveSignal(signal *trading.TradingSignal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.signals = append(s.signals, signal)

	// Keep only recent signals
	if len(s.signals) > s.maxSignals {
		s.signals = s.signals[len(s.signals)-s.maxSignals:]
	}

	return nil
}

// GetSignals retrieves recent signals
func (s *MemoryStorage) GetSignals(limit int) ([]*trading.TradingSignal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.signals) {
		limit = len(s.signals)
	}

	start := len(s.signals) - limit
	result := make([]*trading.TradingSignal, limit)
	copy(result, s.signals[start:])

	return result, nil
}

// GetSignalsBySymbol retrieves signals for a specific symbol
func (s *MemoryStorage) GetSignalsBySymbol(symbol string, limit int) ([]*trading.TradingSignal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]*trading.TradingSignal, 0)
	for _, signal := range s.signals {
		if signal.Symbol == symbol {
			filtered = append(filtered, signal)
		}
	}

	if limit <= 0 || limit > len(filtered) {
		limit = len(filtered)
	}

	start := len(filtered) - limit
	result := make([]*trading.TradingSignal, limit)
	copy(result, filtered[start:])

	return result, nil
}

// GetSignalsByTimeRange retrieves signals within a time range
func (s *MemoryStorage) GetSignalsByTimeRange(start, end time.Time) ([]*trading.TradingSignal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*trading.TradingSignal, 0)
	for _, signal := range s.signals {
		if signal.Timestamp.After(start) && signal.Timestamp.Before(end) {
			result = append(result, signal)
		}
	}

	return result, nil
}
