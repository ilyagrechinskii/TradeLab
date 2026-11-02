package marketdata_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

func fixtureCandles() []backtest.Candle {
	candles := make([]backtest.Candle, 4)
	for i := range candles {
		candles[i] = backtest.Candle{
			OpenTime: testStart.Add(time.Duration(i) * time.Hour),
			Open:     100, High: 120, Low: 90, Close: 110, Volume: 10,
		}
	}
	return candles
}

func TestFixtureCopiesFiltersAndRunsBacktest(t *testing.T) {
	input := fixtureCandles()
	want := append([]backtest.Candle(nil), input[1:3]...)
	provider, err := marketdata.NewFixtureProvider("BTCUSDT", "1h", input)
	if err != nil {
		t.Fatal(err)
	}
	input[1].Close = 119
	candles, err := provider.GetCandles(context.Background(), "BTCUSDT", "1h", testStart.Add(time.Hour), testStart.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(candles, want) {
		t.Fatalf("fixture data changed or range is incorrect: %+v", candles)
	}
	report, err := strategy.Run(candles, strategy.BuyAndHold{}, backtest.Config{InitialCapital: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if report.Result.FinalEquity != 1100 || len(report.Result.Trades) != 1 {
		t.Errorf("fixture cannot feed the existing strategy/engine: %+v", report)
	}
	candles[0].Close = 119
	again, err := provider.GetCandles(context.Background(), "BTCUSDT", "1h", testStart.Add(time.Hour), testStart.Add(3*time.Hour))
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Errorf("caller changed provider data: %+v, %v", again, err)
	}
}

func TestFixtureValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func([]backtest.Candle) []backtest.Candle
	}{
		{"empty", func(c []backtest.Candle) []backtest.Candle { return nil }},
		{"invalid OHLCV", func(c []backtest.Candle) []backtest.Candle { c[1].Open = 0; return c }},
		{"gap", func(c []backtest.Candle) []backtest.Candle { return append(c[:1], c[2:]...) }},
		{"misaligned", func(c []backtest.Candle) []backtest.Candle { c[0].OpenTime = c[0].OpenTime.Add(time.Minute); return c }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := marketdata.NewFixtureProvider("BTCUSDT", "1h", tt.mutate(fixtureCandles())); err == nil {
				t.Error("accepted invalid fixture")
			}
		})
	}
	if _, err := marketdata.NewFixtureProvider("UNKNOWNUSDT", "1h", fixtureCandles()); err == nil {
		t.Error("accepted unsupported fixture symbol")
	}
	provider, err := marketdata.NewFixtureProvider("BTCUSDT", "1h", fixtureCandles())
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		symbol   string
		interval string
		start    time.Time
		end      time.Time
	}{
		{"ETHUSDT", "1h", testStart, testStart.Add(time.Hour)},
		{"BTCUSDT", "4h", testStart, testStart.Add(4 * time.Hour)},
		{"BTCUSDT", "1h", testStart, testStart.Add(5 * time.Hour)},
		{"BTCUSDT", "1h", testStart.Add(time.Minute), testStart.Add(time.Hour)},
	} {
		if candles, err := provider.GetCandles(context.Background(), tt.symbol, tt.interval, tt.start, tt.end); err == nil || candles != nil {
			t.Errorf("expected error without partial fixture: %+v, %v", candles, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.GetCandles(ctx, "BTCUSDT", "1h", testStart, testStart.Add(time.Hour)); !errors.Is(err, context.Canceled) {
		t.Errorf("expected canceled fixture request, got %v", err)
	}
	if _, err := provider.GetCandles(nil, "BTCUSDT", "1h", testStart, testStart.Add(time.Hour)); err == nil {
		t.Error("accepted nil context")
	}
}
