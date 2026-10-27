package backtest_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

func resultWithEquity(initial float64, equity ...float64) backtest.BacktestResult {
	result := backtest.BacktestResult{InitialCapital: initial}
	for i, value := range equity {
		result.EquityCurve = append(result.EquityCurve, backtest.EquityPoint{Time: time.Unix(int64(i), 0), Equity: value})
	}
	if len(equity) > 0 {
		result.FinalEquity = equity[len(equity)-1]
		result.TotalReturn = (result.FinalEquity/initial - 1) * 100
	}
	return result
}

func TestMaxDrawdown(t *testing.T) {
	for _, tt := range []struct {
		name   string
		equity []float64
		want   float64
	}{
		{"initial capital is the first peak", []float64{80, 90}, 20},
		{"prior maximum", []float64{90, 120, 60, 100}, 50},
		{"new high resets peak", []float64{110, 99, 150, 90}, 40},
		{"recovery below initial peak", []float64{80, 90, 70}, 30},
		{"monotonic growth", []float64{100, 110, 120}, 0},
		{"flat", []float64{100, 100, 100}, 0},
		{"complete loss", []float64{0}, 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := resultWithEquity(100, tt.equity...)
			metrics, err := backtest.CalculateMetrics(input)
			if err != nil {
				t.Fatal(err)
			}
			closeEnough(t, "drawdown", metrics.MaxDrawdown, tt.want)
			closeEnough(t, "total return", metrics.TotalReturn, input.TotalReturn)
		})
	}
}

// Every row is one executed order for one unit of asset; notionals therefore
// equal prices. Cash/equity are separate from the realized-pair calculation.
func metricTrades(amounts ...[2]float64) []backtest.Trade {
	trades := make([]backtest.Trade, len(amounts))
	for i, amount := range amounts {
		side := backtest.Buy
		if i%2 == 1 {
			side = backtest.Sell
		}
		trades[i] = backtest.Trade{
			Time: time.Unix(int64(i), 0), Type: side, Price: amount[0],
			Quantity: 1, Notional: amount[0], Fee: amount[1],
		}
	}
	return trades
}

func TestRealizedMetrics(t *testing.T) {
	ptr := func(value float64) *float64 { return &value }
	for _, tt := range []struct {
		name      string
		trades    []backtest.Trade
		completed int
		winRate   *float64
		factor    *float64
	}{
		{name: "no trades"},
		{name: "open position", trades: metricTrades([2]float64{100, 1})},
		{
			name: "no losses", trades: metricTrades([2]float64{100, 1}, [2]float64{121, 1}),
			completed: 1, winRate: ptr(100),
		},
		{
			name: "all losses", trades: metricTrades([2]float64{100, 1}, [2]float64{91, 1}),
			completed: 1, winRate: ptr(0), factor: ptr(0),
		},
		{
			name: "break even after fees", trades: metricTrades([2]float64{100, 1}, [2]float64{102, 1}),
			completed: 1, winRate: ptr(0),
		},
		{
			name: "fees turn gross profit into a loss", trades: metricTrades([2]float64{100, 2}, [2]float64{103, 2}),
			completed: 1, winRate: ptr(0), factor: ptr(0),
		},
		{
			// Net PnLs: +19, -11, 0. The remaining entry is not a closed pair.
			name: "mixed closed positions and an open position",
			trades: metricTrades(
				[2]float64{100, 1}, [2]float64{121, 1},
				[2]float64{100, 1}, [2]float64{91, 1},
				[2]float64{100, 1}, [2]float64{102, 1},
				[2]float64{100, 1},
			),
			completed: 3, winRate: ptr(33.33333333333333), factor: ptr(1.7272727272727273),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := resultWithEquity(1000, 1000, 1050)
			input.Trades = tt.trades
			original, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			metrics, err := backtest.CalculateMetrics(input)
			if err != nil {
				t.Fatal(err)
			}
			if metrics.CompletedTrades != tt.completed {
				t.Errorf("completed trades = %d, want %d", metrics.CompletedTrades, tt.completed)
			}
			assertOptionalMetric(t, "win rate", metrics.WinRate, tt.winRate)
			assertOptionalMetric(t, "profit factor", metrics.ProfitFactor, tt.factor)
			closeEnough(t, "total return", metrics.TotalReturn, 5)
			if _, err := json.Marshal(metrics); err != nil {
				t.Errorf("metrics are not valid JSON: %v", err)
			}
			after, err := json.Marshal(input)
			if err != nil || string(after) != string(original) {
				t.Error("metrics modified the input result")
			}
		})
	}
}

func assertOptionalMetric(t *testing.T, name string, got, want *float64) {
	t.Helper()
	if got == nil || want == nil {
		if (got == nil) != (want == nil) {
			t.Errorf("%s: unexpected defined/undefined state", name)
		}
		return
	}
	closeEnough(t, name, *got, *want)
}

func TestUndefinedMetricsSerializeAsNull(t *testing.T) {
	metrics, err := backtest.CalculateMetrics(resultWithEquity(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(metrics)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"WinRate":null`) || !strings.Contains(string(encoded), `"ProfitFactor":null`) {
		t.Errorf("undefined metrics are not null: %s", encoded)
	}
}

func TestMetricsUseActualExecutionWithSlippageAndFees(t *testing.T) {
	candles := fixture()
	for i := range candles {
		candles[i].Open, candles[i].High = 100, 100
		candles[i].Low, candles[i].Close = 100, 100
	}
	result, err := backtest.Run(candles, []backtest.Signal{
		{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Sell},
	}, backtest.Config{InitialCapital: 1111, FeePercent: 1, SlippagePercent: 10})
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := backtest.CalculateMetrics(result)
	if err != nil {
		t.Fatal(err)
	}
	// Buy: 10 units at 110 plus 11 fee. Sell: 900 minus 9 fee.
	closeEnough(t, "final equity", result.FinalEquity, 891)
	closeEnough(t, "total return", metrics.TotalReturn, -19.801980198019802)
	if metrics.CompletedTrades != 1 || metrics.WinRate == nil || metrics.ProfitFactor == nil {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	closeEnough(t, "win rate", *metrics.WinRate, 0)
	closeEnough(t, "profit factor", *metrics.ProfitFactor, 0)
}

func TestMetricsRejectInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*backtest.BacktestResult)
	}{
		{"zero initial capital", func(r *backtest.BacktestResult) { r.InitialCapital = 0 }},
		{"NaN initial capital", func(r *backtest.BacktestResult) { r.InitialCapital = math.NaN() }},
		{"negative final equity", func(r *backtest.BacktestResult) { r.FinalEquity = -1 }},
		{"infinite total return", func(r *backtest.BacktestResult) { r.TotalReturn = math.Inf(1) }},
		{"empty curve", func(r *backtest.BacktestResult) { r.EquityCurve = nil }},
		{"negative equity", func(r *backtest.BacktestResult) { r.EquityCurve[0].Equity = -1 }},
		{"infinite equity", func(r *backtest.BacktestResult) { r.EquityCurve[0].Equity = math.Inf(1) }},
		{"unordered curve", func(r *backtest.BacktestResult) { r.EquityCurve[1].Time = r.EquityCurve[0].Time }},
		{"sell without buy", func(r *backtest.BacktestResult) { r.Trades = r.Trades[1:] }},
		{"repeated buy", func(r *backtest.BacktestResult) { r.Trades[1].Type = backtest.Buy }},
		{"hold in executions", func(r *backtest.BacktestResult) { r.Trades[0].Type = backtest.Hold }},
		{"partial sale", func(r *backtest.BacktestResult) { r.Trades[1].Quantity = 0.5 }},
		{"NaN notional", func(r *backtest.BacktestResult) { r.Trades[0].Notional = math.NaN() }},
		{"negative fee", func(r *backtest.BacktestResult) { r.Trades[0].Fee = -1 }},
		{"unordered trades", func(r *backtest.BacktestResult) { r.Trades[1].Time = r.Trades[0].Time }},
		{"entry cost overflow", func(r *backtest.BacktestResult) {
			r.Trades = metricTrades([2]float64{math.MaxFloat64, math.MaxFloat64})
		}},
		{"sum of profits overflow", func(r *backtest.BacktestResult) {
			r.Trades = metricTrades([2]float64{1, 0}, [2]float64{math.MaxFloat64, 0}, [2]float64{1, 0}, [2]float64{math.MaxFloat64, 0})
		}},
		{"profit factor overflow", func(r *backtest.BacktestResult) {
			r.Trades = metricTrades([2]float64{1, 0}, [2]float64{1e300, 0}, [2]float64{2e-300, 0}, [2]float64{1e-300, 0})
		}},
		{"profit factor underflow", func(r *backtest.BacktestResult) {
			r.Trades = metricTrades([2]float64{1e-300, 0}, [2]float64{2e-300, 0}, [2]float64{1e300, 0}, [2]float64{1, 0})
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := resultWithEquity(1000, 1000, 1050)
			input.Trades = metricTrades([2]float64{100, 1}, [2]float64{121, 1})
			tt.mutate(&input)
			metrics, err := backtest.CalculateMetrics(input)
			if err == nil || !reflect.DeepEqual(metrics, backtest.Metrics{}) {
				t.Errorf("expected zero metrics and error, got %+v, %v", metrics, err)
			}
		})
	}
}
