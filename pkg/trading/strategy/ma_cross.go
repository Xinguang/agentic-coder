package strategy

import (
	"fmt"
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading"
)

// MACrossStrategy implements moving average crossover strategy
type MACrossStrategy struct {
	name            string
	shortPeriod     int                            // short-term MA period
	longPeriod      int                            // long-term MA period
	historicalData  map[string][]*trading.StockData // historical data cache
	maxHistorySize  int
}

// NewMACrossStrategy creates a new MA crossover strategy
func NewMACrossStrategy(shortPeriod, longPeriod int) *MACrossStrategy {
	return &MACrossStrategy{
		name:           fmt.Sprintf("MA_Cross_%d_%d", shortPeriod, longPeriod),
		shortPeriod:    shortPeriod,
		longPeriod:     longPeriod,
		historicalData: make(map[string][]*trading.StockData),
		maxHistorySize: longPeriod * 2,
	}
}

// Name implements Strategy interface
func (s *MACrossStrategy) Name() string {
	return s.name
}

// Analyze implements Strategy interface
func (s *MACrossStrategy) Analyze(data []*trading.StockData, positions map[string]*trading.Position) ([]*trading.TradingSignal, error) {
	signals := make([]*trading.TradingSignal, 0)

	for _, currentData := range data {
		symbol := currentData.Symbol

		// Update historical data
		if _, exists := s.historicalData[symbol]; !exists {
			s.historicalData[symbol] = make([]*trading.StockData, 0, s.maxHistorySize)
		}
		s.historicalData[symbol] = append(s.historicalData[symbol], currentData)

		// Keep only recent data
		if len(s.historicalData[symbol]) > s.maxHistorySize {
			s.historicalData[symbol] = s.historicalData[symbol][len(s.historicalData[symbol])-s.maxHistorySize:]
		}

		history := s.historicalData[symbol]

		// Need enough data for both MAs
		if len(history) < s.longPeriod {
			continue
		}

		// Calculate short-term and long-term moving averages
		shortMA := calculateMA(history, s.shortPeriod)
		longMA := calculateMA(history, s.longPeriod)

		// Calculate previous MAs for crossover detection
		if len(history) < s.longPeriod+1 {
			continue
		}
		prevShortMA := calculateMA(history[:len(history)-1], s.shortPeriod)
		prevLongMA := calculateMA(history[:len(history)-1], s.longPeriod)

		// Detect crossover
		signal := s.detectCrossover(symbol, currentData.Price, shortMA, longMA, prevShortMA, prevLongMA, positions)
		if signal != nil {
			signals = append(signals, signal)
		}
	}

	return signals, nil
}

// detectCrossover detects MA crossover and generates signal
func (s *MACrossStrategy) detectCrossover(
	symbol string,
	price float64,
	shortMA, longMA, prevShortMA, prevLongMA float64,
	positions map[string]*trading.Position,
) *trading.TradingSignal {
	now := time.Now()

	// Golden cross: short MA crosses above long MA (buy signal)
	if prevShortMA <= prevLongMA && shortMA > longMA {
		// Check if already holding position
		if pos, exists := positions[symbol]; exists && pos.Quantity > 0 {
			return nil // already holding, no new signal
		}

		return &trading.TradingSignal{
			Symbol:     symbol,
			Type:       trading.SignalBuy,
			Price:      price,
			Timestamp:  now,
			ExecuteAt:  getNextTradingTime(now), // execute at next trading session
			Reason:     fmt.Sprintf("Golden cross: MA%d(%.2f) > MA%d(%.2f)", s.shortPeriod, shortMA, s.longPeriod, longMA),
			Confidence: calculateConfidence(shortMA, longMA),
		}
	}

	// Death cross: short MA crosses below long MA (sell signal)
	if prevShortMA >= prevLongMA && shortMA < longMA {
		// Check if holding position
		if pos, exists := positions[symbol]; !exists || pos.Quantity <= 0 {
			return nil // no position to sell
		}

		return &trading.TradingSignal{
			Symbol:     symbol,
			Type:       trading.SignalSell,
			Price:      price,
			Timestamp:  now,
			ExecuteAt:  getNextTradingTime(now),
			Reason:     fmt.Sprintf("Death cross: MA%d(%.2f) < MA%d(%.2f)", s.shortPeriod, shortMA, s.longPeriod, longMA),
			Confidence: calculateConfidence(longMA, shortMA),
		}
	}

	return nil
}

// calculateMA calculates moving average
func calculateMA(data []*trading.StockData, period int) float64 {
	if len(data) < period {
		return 0
	}

	sum := 0.0
	start := len(data) - period
	for i := start; i < len(data); i++ {
		sum += data[i].Price
	}

	return sum / float64(period)
}

// calculateConfidence calculates signal confidence based on MA divergence
func calculateConfidence(ma1, ma2 float64) float64 {
	divergence := (ma1 - ma2) / ma2
	if divergence < 0 {
		divergence = -divergence
	}

	// Confidence increases with divergence, capped at 1.0
	confidence := divergence * 10
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

// getNextTradingTime returns the next trading time
// For daily trading, this would be the next market open
func getNextTradingTime(now time.Time) time.Time {
	// Simplified: assume market opens at 9:30 AM next trading day
	next := now.Add(24 * time.Hour)
	next = time.Date(next.Year(), next.Month(), next.Day(), 9, 30, 0, 0, next.Location())

	// Skip weekends
	for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
		next = next.Add(24 * time.Hour)
	}

	return next
}
