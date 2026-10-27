package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
)

func main() {
	fixture := flag.Bool("fixture", false, "use synthetic demo candles without network access")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := run(ctx, *fixture); err != nil {
		fmt.Fprintln(os.Stderr, "marketcheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, useFixture bool) error {
	// Fixed historical period keeps manual checks repeatable and fully closed.
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(6 * time.Hour)
	var provider marketdata.Provider
	if useFixture {
		fmt.Println("Source: SYNTHETIC FIXTURE — demo data, not market quotes")
		candles := make([]backtest.Candle, 6)
		for i := range candles {
			price := 100 + float64(i)
			candles[i] = backtest.Candle{
				OpenTime: start.Add(time.Duration(i) * time.Hour),
				Open:     price, High: price + 2, Low: price - 1, Close: price + 1, Volume: 10,
			}
		}
		fixture, err := marketdata.NewFixtureProvider("BTCUSDT", "1h", candles)
		if err != nil {
			return err
		}
		provider = fixture
	} else {
		fmt.Println("Source: Binance public Spot market data")
		binance, err := marketdata.NewBinance(marketdata.BinanceOptions{})
		if err != nil {
			return err
		}
		provider = binance
	}
	candles, err := provider.GetCandles(ctx, "BTCUSDT", "1h", start, end)
	if err != nil {
		return err
	}
	fmt.Printf("BTCUSDT 1h: %d candles\nFirst: %s\nLast:  %s\n", len(candles),
		candles[0].OpenTime.Format(time.RFC3339), candles[len(candles)-1].OpenTime.Format(time.RFC3339))
	for _, candle := range candles[:min(3, len(candles))] {
		fmt.Printf("%s O=%.8f H=%.8f L=%.8f C=%.8f V=%.8f\n", candle.OpenTime.Format(time.RFC3339),
			candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
	}
	return nil
}
