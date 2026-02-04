package provider

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading"
)

// MockProvider is a mock data provider for testing
type MockProvider struct {
	basePrice map[string]float64
}

// NewMockProvider creates a new mock data provider
func NewMockProvider() *MockProvider {
	return &MockProvider{
		basePrice: make(map[string]float64),
	}
}

// GetStockData implements DataProvider interface
func (m *MockProvider) GetStockData(ctx context.Context, symbols []string) ([]*trading.StockData, error) {
	result := make([]*trading.StockData, 0, len(symbols))

	for _, symbol := range symbols {
		// Initialize base price if not exists
		if _, exists := m.basePrice[symbol]; !exists {
			m.basePrice[symbol] = 100.0 + rand.Float64()*900.0 // random price between 100-1000
		}

		basePrice := m.basePrice[symbol]
		// Simulate price fluctuation
		change := (rand.Float64() - 0.5) * basePrice * 0.02 // ±2% change
		currentPrice := basePrice + change

		data := &trading.StockData{
			Symbol:    symbol,
			Price:     currentPrice,
			Open:      basePrice,
			High:      currentPrice * (1 + rand.Float64()*0.01),
			Low:       currentPrice * (1 - rand.Float64()*0.01),
			Volume:    int64(rand.Intn(1000000) + 100000),
			Timestamp: time.Now(),
		}

		// Update base price for next call
		m.basePrice[symbol] = currentPrice

		result = append(result, data)
	}

	return result, nil
}

// Subscribe implements DataProvider interface
func (m *MockProvider) Subscribe(ctx context.Context, symbols []string, callback func(*trading.StockData)) error {
	go func() {
		ticker := time.NewTicker(5 * time.Second) // update every 5 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				data, err := m.GetStockData(ctx, symbols)
				if err != nil {
					fmt.Printf("Error fetching data: %v\n", err)
					continue
				}
				for _, d := range data {
					callback(d)
				}
			}
		}
	}()

	return nil
}

// Close implements DataProvider interface
func (m *MockProvider) Close() error {
	return nil
}
