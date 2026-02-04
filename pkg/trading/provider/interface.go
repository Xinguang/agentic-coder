package provider

import (
	"context"
	"github.com/xinguang/agentic-coder/pkg/trading"
)

// DataProvider interface for stock data providers
type DataProvider interface {
	// GetStockData fetches current stock data for given symbols
	GetStockData(ctx context.Context, symbols []string) ([]*trading.StockData, error)

	// Subscribe subscribes to real-time updates for given symbols
	Subscribe(ctx context.Context, symbols []string, callback func(*trading.StockData)) error

	// Close closes the data provider connection
	Close() error
}
