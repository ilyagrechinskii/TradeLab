package simulation_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/simulation"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

type providerFunc func(context.Context, string, string, time.Time, time.Time) ([]backtest.Candle, error)

func (f providerFunc) GetCandles(ctx context.Context, symbol, interval string, start, end time.Time) ([]backtest.Candle, error) {
	return f(ctx, symbol, interval, start, end)
}

func TestWarmupLoadsOnlyNeededHistoryAndPreservesInputs(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	closes := []float64{3, 2, 1, 2, 3, 4, 3, 2, 1}
	candles := make([]backtest.Candle, len(closes))
	for i, price := range closes {
		candles[i] = backtest.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: price, High: price, Low: price, Close: price}
	}
	candles[5].Open, candles[5].High = 10, 10
	candles[8].Open, candles[8].High = 20, 20
	original := append([]backtest.Candle(nil), candles...)
	service, err := simulation.NewService(providerFunc(func(ctx context.Context, symbol, interval string, loadStart, loadEnd time.Time) ([]backtest.Candle, error) {
		if !loadStart.Equal(start) || !loadEnd.Equal(start.Add(9*time.Hour)) {
			t.Errorf("wrong warmup range: %s %s", loadStart, loadEnd)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("provider context has no deadline")
		}
		return candles, nil
	}), simulation.Options{Source: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := service.Run(context.Background(), simulation.Request{
		Market:   simulation.MarketRequest{Symbol: "BTCUSDT", Interval: "1h", Start: start.Add(3 * time.Hour), End: start.Add(9 * time.Hour)},
		Strategy: simulation.StrategySpec{Type: "sma_crossover", FastPeriod: 2, SlowPeriod: 3}, Config: backtest.Config{InitialCapital: 1000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(candles, original) {
		t.Error("warmup adapter changed market data")
	}
	if out.Report.Result.InitialCapital != 1000 || out.Report.Result.FinalEquity != 2000 || len(out.Report.Result.EquityCurve) != 6 {
		t.Errorf("warmup affected trading balances/curve: %+v", out.Report.Result)
	}
	for _, trade := range out.Report.Result.Trades {
		if trade.Time.Before(out.Market.Start) {
			t.Error("trade executed during warmup")
		}
	}
}

func TestCanceledCalculationVariants(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := backtest.RunContext(ctx, nil, nil, backtest.DefaultConfig()); !errors.Is(err, context.Canceled) {
		t.Errorf("engine did not honor cancellation: %v", err)
	}
	if _, err := backtest.CalculateMetricsContext(ctx, backtest.BacktestResult{}); !errors.Is(err, context.Canceled) {
		t.Errorf("metrics did not honor cancellation: %v", err)
	}
	if _, err := strategy.DefaultSMACrossover().AnalyzeContext(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("strategy did not honor cancellation: %v", err)
	}
}

func TestServiceRejectsEmptyAndInterruptedData(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, mode := range []string{"empty", "cancel_after_load"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service, err := simulation.NewService(providerFunc(func(context.Context, string, string, time.Time, time.Time) ([]backtest.Candle, error) {
				if mode == "cancel_after_load" {
					cancel()
				}
				return nil, nil
			}), simulation.Options{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Candles(ctx, simulation.MarketRequest{Symbol: "BTCUSDT", Interval: "1h", Start: start, End: start.Add(2 * time.Hour)})
			if mode == "cancel_after_load" {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("processing continued after cancellation: %v", err)
				}
			} else {
				var upstream *simulation.DataError
				if !errors.As(err, &upstream) {
					t.Errorf("empty data was not classified as upstream error: %v", err)
				}
			}
		})
	}
}

func TestValidatedRangeUsesClosedPortion(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	r, err := marketdata.ValidateRange("BTCUSDT", "1h", start, start.Add(3*time.Hour), start.Add(150*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if r.Count != 2 || r.Step != time.Hour || !r.End.Equal(start.Add(2*time.Hour)) {
		t.Errorf("wrong closed range: %+v", r)
	}
	if err := r.ValidateCandles(nil); err == nil {
		t.Error("accepted empty range data")
	}
}
