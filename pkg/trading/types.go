package trading

import "time"

// StockData represents real-time stock data
type StockData struct {
	Symbol    string    // stock symbol
	Price     float64   // current price
	Open      float64   // opening price
	High      float64   // highest price
	Low       float64   // lowest price
	Volume    int64     // trading volume
	Timestamp time.Time // data timestamp
}

// SignalType represents buy/sell signal type
type SignalType string

const (
	SignalBuy  SignalType = "BUY"
	SignalSell SignalType = "SELL"
	SignalHold SignalType = "HOLD"
)

// TradingSignal represents a trading signal
type TradingSignal struct {
	Symbol     string     // stock symbol
	Type       SignalType // signal type
	Price      float64    // suggested price
	Timestamp  time.Time  // signal generation time
	ExecuteAt  time.Time  // suggested execution time
	Reason     string     // signal reason
	Confidence float64    // confidence level (0-1)
}

// Position represents a stock position
type Position struct {
	Symbol       string    // stock symbol
	Quantity     int       // quantity held
	AvgPrice     float64   // average purchase price
	CurrentPrice float64   // current price
	PnL          float64   // profit/loss
	UpdatedAt    time.Time // last update time
}
