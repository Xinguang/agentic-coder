package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xinguang/agentic-coder/pkg/trading/engine"
	"github.com/xinguang/agentic-coder/pkg/trading/provider"
	"github.com/xinguang/agentic-coder/pkg/trading/strategy"
)

func main() {
	fmt.Println("=================================================")
	fmt.Println("       Daily Stock Trading System")
	fmt.Println("=================================================")
	fmt.Println()

	// Configuration
	symbols := []string{"AAPL", "GOOGL", "MSFT", "TSLA", "AMZN"}
	updateInterval := 10 * time.Second // fetch data every 10 seconds

	fmt.Printf("Monitoring stocks: %v\n", symbols)
	fmt.Printf("Update interval: %v\n", updateInterval)
	fmt.Println()

	// Create data provider (using mock provider for demonstration)
	dataProvider := provider.NewMockProvider()

	// Create trading strategies
	strategies := []engine.Strategy{
		strategy.NewMACrossStrategy(5, 20),   // 5-day and 20-day MA crossover
		strategy.NewMACrossStrategy(10, 50),  // 10-day and 50-day MA crossover
	}

	fmt.Println("Active strategies:")
	for i, s := range strategies {
		fmt.Printf("  %d. %s\n", i+1, s.Name())
	}
	fmt.Println()

	// Create engine
	config := &engine.Config{
		Symbols:        symbols,
		UpdateInterval: updateInterval,
	}

	eng := engine.NewEngine(config, dataProvider, strategies)

	// Start engine
	if err := eng.Start(); err != nil {
		fmt.Printf("Error starting engine: %v\n", err)
		os.Exit(1)
	}

	// Example: Set initial positions
	// eng.UpdatePosition("AAPL", 100, 150.0)
	// eng.UpdatePosition("GOOGL", 50, 140.0)

	fmt.Println("System running... Press Ctrl+C to stop")
	fmt.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")

	// Stop engine
	if err := eng.Stop(); err != nil {
		fmt.Printf("Error stopping engine: %v\n", err)
	}

	// Display recent signals before exit
	fmt.Println("\nRecent signals summary:")
	signals, err := eng.GetRecentSignals(5)
	if err != nil {
		fmt.Printf("Error getting signals: %v\n", err)
	} else {
		for i, sig := range signals {
			fmt.Printf("%d. %s %s @ $%.2f (confidence: %.1f%%) - %s\n",
				i+1, sig.Type, sig.Symbol, sig.Price, sig.Confidence*100,
				sig.Timestamp.Format("2006-01-02 15:04:05"))
		}
	}

	fmt.Println("\nSystem stopped gracefully")
}
