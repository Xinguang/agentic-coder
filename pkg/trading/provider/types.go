package provider

import (
	"context"
	"time"
)

// OHLCV represents Open, High, Low, Close, Volume data for a single period
type OHLCV struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
}

// Quote represents real-time stock quote
type Quote struct {
	Symbol       string    `json:"symbol"`
	Name         string    `json:"name"`
	Price        float64   `json:"price"`
	Change       float64   `json:"change"`
	ChangePercent float64  `json:"change_percent"`
	Volume       int64     `json:"volume"`
	Timestamp    time.Time `json:"timestamp"`
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	PrevClose    float64   `json:"prev_close"`
}

// Interval represents the time interval for K-line data
type Interval string

const (
	Interval1Min  Interval = "1min"
	Interval5Min  Interval = "5min"
	Interval15Min Interval = "15min"
	Interval30Min Interval = "30min"
	Interval60Min Interval = "60min"
	IntervalDaily Interval = "daily"
	IntervalWeekly Interval = "weekly"
	IntervalMonthly Interval = "monthly"
)

// StockDataProvider defines the interface for stock data providers
type StockDataProvider interface {
	// GetHistoricalData gets historical OHLCV data
	GetHistoricalData(ctx context.Context, symbol string, interval Interval, limit int) ([]OHLCV, error)

	// GetRealtimeQuote gets real-time quote
	GetRealtimeQuote(ctx context.Context, symbol string) (*Quote, error)

	// GetMultipleQuotes gets multiple real-time quotes
	GetMultipleQuotes(ctx context.Context, symbols []string) ([]*Quote, error)

	// Name returns the provider name
	Name() string
}
