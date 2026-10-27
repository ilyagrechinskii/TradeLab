// Package strategy generates causal signals and composes strategies with the
// existing backtest engine. Strategies never execute trades or manage money.
package strategy

import (
	"errors"
	"fmt"
	"math"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

type Strategy interface {
	Generate(candles []backtest.Candle) ([]backtest.Signal, error)
}

type Report struct {
	Result  backtest.BacktestResult
	Metrics backtest.Metrics
}

// Comparison uses the entire supplied interval and the same execution settings
// for both strategies, including SMA warm-up candles in the benchmark period.
type Comparison struct {
	Strategy     Report
	BuyAndHold   Report
	ExcessReturn float64 // Strategy minus Buy & Hold, in percentage points.
}

func Run(candles []backtest.Candle, selected Strategy, cfg backtest.Config) (Report, error) {
	if selected == nil {
		return Report{}, errors.New("strategy is required")
	}
	if err := cfg.Validate(); err != nil {
		return Report{}, err
	}
	signals, err := selected.Generate(candles)
	if err != nil {
		return Report{}, fmt.Errorf("generate signals: %w", err)
	}
	result, err := backtest.Run(candles, signals, cfg)
	if err != nil {
		return Report{}, fmt.Errorf("backtest: %w", err)
	}
	metrics, err := backtest.CalculateMetrics(result)
	if err != nil {
		return Report{}, fmt.Errorf("metrics: %w", err)
	}
	return Report{Result: result, Metrics: metrics}, nil
}

func CompareWithBuyAndHold(candles []backtest.Candle, selected Strategy, cfg backtest.Config) (Comparison, error) {
	report, err := Run(candles, selected, cfg)
	if err != nil {
		return Comparison{}, err
	}
	benchmark, err := Run(candles, BuyAndHold{}, cfg)
	if err != nil {
		return Comparison{}, fmt.Errorf("buy and hold benchmark: %w", err)
	}
	excess := report.Result.TotalReturn - benchmark.Result.TotalReturn
	if math.IsNaN(excess) || math.IsInf(excess, 0) {
		return Comparison{}, errors.New("excess return exceeds float64 range")
	}
	return Comparison{Strategy: report, BuyAndHold: benchmark, ExcessReturn: excess}, nil
}
