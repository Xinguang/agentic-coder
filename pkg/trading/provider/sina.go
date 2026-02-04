package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SinaProvider provides stock data from Sina Finance API
type SinaProvider struct {
	client *http.Client
}

// NewSinaProvider creates a new Sina data provider
func NewSinaProvider() *SinaProvider {
	return &SinaProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *SinaProvider) Name() string {
	return "sina"
}

// GetRealtimeQuote gets real-time quote from Sina Finance
func (s *SinaProvider) GetRealtimeQuote(ctx context.Context, symbol string) (*Quote, error) {
	// Convert symbol format: 600000 -> sh600000, 000001 -> sz000001
	fullSymbol := s.normalizeSymbol(symbol)

	url := fmt.Sprintf("http://hq.sinajs.cn/list=%s", fullSymbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return s.parseRealtimeQuote(symbol, string(body))
}

// GetMultipleQuotes gets multiple real-time quotes
func (s *SinaProvider) GetMultipleQuotes(ctx context.Context, symbols []string) ([]*Quote, error) {
	if len(symbols) == 0 {
		return []*Quote{}, nil
	}

	normalizedSymbols := make([]string, len(symbols))
	for i, symbol := range symbols {
		normalizedSymbols[i] = s.normalizeSymbol(symbol)
	}

	url := fmt.Sprintf("http://hq.sinajs.cn/list=%s", strings.Join(normalizedSymbols, ","))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return s.parseMultipleQuotes(symbols, string(body))
}

// GetHistoricalData gets historical OHLCV data (simplified version)
func (s *SinaProvider) GetHistoricalData(ctx context.Context, symbol string, interval Interval, limit int) ([]OHLCV, error) {
	// Note: Sina's historical data API is more complex and may require different endpoints
	// This is a placeholder implementation
	return nil, fmt.Errorf("historical data not implemented for Sina provider")
}

// normalizeSymbol converts stock code to Sina format
func (s *SinaProvider) normalizeSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)

	// If already has prefix, return as is
	if strings.HasPrefix(symbol, "sh") || strings.HasPrefix(symbol, "sz") {
		return symbol
	}

	// Shanghai stocks: 60xxxx, 688xxx (STAR Market)
	if strings.HasPrefix(symbol, "60") || strings.HasPrefix(symbol, "688") {
		return "sh" + symbol
	}

	// Shenzhen stocks: 00xxxx, 30xxxx (ChiNext)
	if strings.HasPrefix(symbol, "00") || strings.HasPrefix(symbol, "30") {
		return "sz" + symbol
	}

	// Default to Shanghai
	return "sh" + symbol
}

// parseRealtimeQuote parses Sina's real-time quote data
func (s *SinaProvider) parseRealtimeQuote(symbol, data string) (*Quote, error) {
	// Format: var hq_str_sh600000="浦发银行,8.44,8.43,8.39,8.45,8.38,8.39,8.40,..."
	start := strings.Index(data, "\"")
	end := strings.LastIndex(data, "\"")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid data format")
	}

	fields := strings.Split(data[start+1:end], ",")
	if len(fields) < 32 {
		return nil, fmt.Errorf("insufficient fields: got %d", len(fields))
	}

	parseFloat := func(s string) float64 {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}

	parseInt := func(s string) int64 {
		v, _ := strconv.ParseInt(s, 10, 64)
		return v
	}

	price := parseFloat(fields[3])
	prevClose := parseFloat(fields[2])
	change := price - prevClose
	changePercent := 0.0
	if prevClose > 0 {
		changePercent = (change / prevClose) * 100
	}

	return &Quote{
		Symbol:        symbol,
		Name:          fields[0],
		Price:         price,
		Open:          parseFloat(fields[1]),
		PrevClose:     prevClose,
		High:          parseFloat(fields[4]),
		Low:           parseFloat(fields[5]),
		Volume:        parseInt(fields[8]),
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Now(),
	}, nil
}

// parseMultipleQuotes parses multiple quotes
func (s *SinaProvider) parseMultipleQuotes(symbols []string, data string) ([]*Quote, error) {
	lines := strings.Split(data, "\n")
	quotes := make([]*Quote, 0, len(symbols))

	for i, symbol := range symbols {
		if i >= len(lines) {
			break
		}

		quote, err := s.parseRealtimeQuote(symbol, lines[i])
		if err != nil {
			// Skip failed quotes but continue processing
			continue
		}
		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// TencentProvider provides stock data from Tencent Finance API
type TencentProvider struct {
	client *http.Client
}

// NewTencentProvider creates a new Tencent data provider
func NewTencentProvider() *TencentProvider {
	return &TencentProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TencentProvider) Name() string {
	return "tencent"
}

// GetRealtimeQuote gets real-time quote from Tencent Finance
func (t *TencentProvider) GetRealtimeQuote(ctx context.Context, symbol string) (*Quote, error) {
	fullSymbol := t.normalizeSymbol(symbol)

	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s", fullSymbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return t.parseRealtimeQuote(symbol, string(body))
}

// GetMultipleQuotes gets multiple real-time quotes
func (t *TencentProvider) GetMultipleQuotes(ctx context.Context, symbols []string) ([]*Quote, error) {
	if len(symbols) == 0 {
		return []*Quote{}, nil
	}

	normalizedSymbols := make([]string, len(symbols))
	for i, symbol := range symbols {
		normalizedSymbols[i] = t.normalizeSymbol(symbol)
	}

	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s", strings.Join(normalizedSymbols, ","))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Split by line
	lines := strings.Split(string(body), "\n")
	quotes := make([]*Quote, 0, len(symbols))

	for i, symbol := range symbols {
		if i >= len(lines) || strings.TrimSpace(lines[i]) == "" {
			continue
		}

		quote, err := t.parseRealtimeQuote(symbol, lines[i])
		if err != nil {
			continue
		}
		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// GetHistoricalData gets historical OHLCV data
func (t *TencentProvider) GetHistoricalData(ctx context.Context, symbol string, interval Interval, limit int) ([]OHLCV, error) {
	// Tencent historical data endpoint
	fullSymbol := t.normalizeSymbol(symbol)

	// Map interval to Tencent format
	var intervalParam string
	switch interval {
	case IntervalDaily:
		intervalParam = "day"
	case IntervalWeekly:
		intervalParam = "week"
	case IntervalMonthly:
		intervalParam = "month"
	default:
		intervalParam = "day"
	}

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,,%d",
		fullSymbol, intervalParam, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return t.parseHistoricalData(body)
}

func (t *TencentProvider) normalizeSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)

	if strings.HasPrefix(symbol, "sh") || strings.HasPrefix(symbol, "sz") {
		return symbol
	}

	if strings.HasPrefix(symbol, "60") || strings.HasPrefix(symbol, "688") {
		return "sh" + symbol
	}

	if strings.HasPrefix(symbol, "00") || strings.HasPrefix(symbol, "30") {
		return "sz" + symbol
	}

	return "sh" + symbol
}

func (t *TencentProvider) parseRealtimeQuote(symbol, data string) (*Quote, error) {
	// Format: v_sh600000="51~浦发银行~600000~8.39~8.43~8.44~..."
	start := strings.Index(data, "\"")
	end := strings.LastIndex(data, "\"")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid data format")
	}

	fields := strings.Split(data[start+1:end], "~")
	if len(fields) < 50 {
		return nil, fmt.Errorf("insufficient fields: got %d", len(fields))
	}

	parseFloat := func(s string) float64 {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}

	parseInt := func(s string) int64 {
		v, _ := strconv.ParseInt(s, 10, 64)
		return v
	}

	price := parseFloat(fields[3])
	prevClose := parseFloat(fields[4])
	change := price - prevClose
	changePercent := 0.0
	if prevClose > 0 {
		changePercent = (change / prevClose) * 100
	}

	return &Quote{
		Symbol:        symbol,
		Name:          fields[1],
		Price:         price,
		Open:          parseFloat(fields[5]),
		PrevClose:     prevClose,
		High:          parseFloat(fields[33]),
		Low:           parseFloat(fields[34]),
		Volume:        parseInt(fields[36]),
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Now(),
	}, nil
}

func (t *TencentProvider) parseHistoricalData(body []byte) ([]OHLCV, error) {
	var result struct {
		Code int `json:"code"`
		Data map[string]struct {
			Day [][]interface{} `json:"day"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("api error: code %d", result.Code)
	}

	var ohlcvList []OHLCV
	for _, stockData := range result.Data {
		for _, item := range stockData.Day {
			if len(item) < 6 {
				continue
			}

			dateStr, _ := item[0].(string)
			timestamp, _ := time.Parse("2006-01-02", dateStr)

			open, _ := strconv.ParseFloat(fmt.Sprint(item[1]), 64)
			close, _ := strconv.ParseFloat(fmt.Sprint(item[2]), 64)
			high, _ := strconv.ParseFloat(fmt.Sprint(item[3]), 64)
			low, _ := strconv.ParseFloat(fmt.Sprint(item[4]), 64)
			volume, _ := strconv.ParseInt(fmt.Sprint(item[5]), 10, 64)

			ohlcvList = append(ohlcvList, OHLCV{
				Timestamp: timestamp,
				Open:      open,
				High:      high,
				Low:       low,
				Close:     close,
				Volume:    volume,
			})
		}
	}

	return ohlcvList, nil
}
