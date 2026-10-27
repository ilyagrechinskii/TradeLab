package backtest_test

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

func TestDefaultConfig(t *testing.T) {
	cfg := backtest.DefaultConfig()
	if cfg.InitialCapital != 10000 || cfg.FeePercent != 0.1 || cfg.SlippagePercent != 0 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRunInvalidConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		cfg  backtest.Config
	}{
		{"zero capital", backtest.Config{}},
		{"negative capital", backtest.Config{InitialCapital: -1}},
		{"NaN capital", backtest.Config{InitialCapital: math.NaN()}},
		{"infinite capital", backtest.Config{InitialCapital: math.Inf(1)}},
		{"negative infinite capital", backtest.Config{InitialCapital: math.Inf(-1)}},
		{"negative fee", backtest.Config{InitialCapital: 1000, FeePercent: -0.1}},
		{"100 percent fee", backtest.Config{InitialCapital: 1000, FeePercent: 100}},
		{"excessive fee", backtest.Config{InitialCapital: 1000, FeePercent: 101}},
		{"NaN fee", backtest.Config{InitialCapital: 1000, FeePercent: math.NaN()}},
		{"infinite fee", backtest.Config{InitialCapital: 1000, FeePercent: math.Inf(1)}},
		{"negative slippage", backtest.Config{InitialCapital: 1000, SlippagePercent: -1}},
		{"100 percent slippage", backtest.Config{InitialCapital: 1000, SlippagePercent: 100}},
		{"excessive slippage", backtest.Config{InitialCapital: 1000, SlippagePercent: 101}},
		{"NaN slippage", backtest.Config{InitialCapital: 1000, SlippagePercent: math.NaN()}},
		{"infinite slippage", backtest.Config{InitialCapital: 1000, SlippagePercent: math.Inf(1)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Error("Validate accepted invalid config")
			}
			result, err := backtest.Run(fixture(), nil, tt.cfg)
			assertFailedRun(t, result, err)
		})
	}
}

func TestRunInvalidCandles(t *testing.T) {
	for _, field := range []string{"open", "high", "low", "close"} {
		for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
			t.Run(field+"/"+floatLabel(value), func(t *testing.T) {
				candles := fixture()
				switch field {
				case "open":
					candles[1].Open = value
				case "high":
					candles[1].High = value
				case "low":
					candles[1].Low = value
				case "close":
					candles[1].Close = value
				}
				result, err := backtest.Run(candles, nil, backtest.DefaultConfig())
				assertFailedRun(t, result, err)
			})
		}
	}
	for _, tt := range []struct {
		name   string
		mutate func([]backtest.Candle) []backtest.Candle
	}{
		{"empty", func(c []backtest.Candle) []backtest.Candle { return nil }},
		{"high below open", func(c []backtest.Candle) []backtest.Candle { c[1].Open = 121; return c }},
		{"high below close", func(c []backtest.Candle) []backtest.Candle { c[1].High = 110; return c }},
		{"high below low", func(c []backtest.Candle) []backtest.Candle { c[1].Low = 121; return c }},
		{"low above open", func(c []backtest.Candle) []backtest.Candle { c[1].Low = 101; return c }},
		{"low above close", func(c []backtest.Candle) []backtest.Candle { c[1].Close = 99; return c }},
		{"negative volume", func(c []backtest.Candle) []backtest.Candle { c[1].Volume = -1; return c }},
		{"NaN volume", func(c []backtest.Candle) []backtest.Candle { c[1].Volume = math.NaN(); return c }},
		{"infinite volume", func(c []backtest.Candle) []backtest.Candle { c[1].Volume = math.Inf(1); return c }},
		{"unsorted", func(c []backtest.Candle) []backtest.Candle { c[0], c[1] = c[1], c[0]; return c }},
		{"duplicate timestamp", func(c []backtest.Candle) []backtest.Candle { c[1].OpenTime = c[0].OpenTime; return c }},
		{"same instant in another zone", func(c []backtest.Candle) []backtest.Candle { c[1].OpenTime = c[0].OpenTime.UTC(); return c }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candles := tt.mutate(fixture())
			if err := backtest.ValidateCandles(candles); err == nil {
				t.Error("ValidateCandles accepted invalid candles")
			}
			result, err := backtest.Run(candles, nil, backtest.DefaultConfig())
			assertFailedRun(t, result, err)
		})
	}
}

func floatLabel(value float64) string {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "+Inf"
	case math.IsInf(value, -1):
		return "-Inf"
	case value == 0:
		return "zero"
	default:
		return "negative"
	}
}

func TestRunInvalidSignals(t *testing.T) {
	for _, tt := range []struct {
		name    string
		signals []backtest.Signal
	}{
		{"negative index", []backtest.Signal{{CandleIndex: -1, Type: backtest.Buy}}},
		{"out of range", []backtest.Signal{{CandleIndex: 3, Type: backtest.Buy}}},
		{"unknown type", []backtest.Signal{{CandleIndex: 0, Type: 255}}},
		{"invalid last candle type", []backtest.Signal{{CandleIndex: 2, Type: 255}}},
		{"conflicting signals", []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 0, Type: backtest.Sell}}},
		{"duplicate hold", []backtest.Signal{{CandleIndex: 1, Type: backtest.Hold}, {CandleIndex: 1, Type: backtest.Hold}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := backtest.Run(fixture(), tt.signals, backtest.DefaultConfig())
			assertFailedRun(t, result, err)
			if err != nil && !strings.Contains(err.Error(), "signal") {
				t.Errorf("error lacks signal context: %v", err)
			}
		})
	}
}

func assertFailedRun(t *testing.T, result backtest.BacktestResult, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	if !reflect.DeepEqual(result, backtest.BacktestResult{}) {
		t.Errorf("error returned a partial result: %+v", result)
	}
}

func TestRunUnrepresentableArithmetic(t *testing.T) {
	for _, tt := range []struct {
		name    string
		capital float64
		fee     float64
		slip    float64
		prices  [][2]float64 // Open and Close; derive a valid High and Low.
		signals []backtest.Signal
	}{
		{
			name: "execution price overflow", capital: 1000, slip: 10,
			prices:  [][2]float64{{1, 1}, {math.MaxFloat64, math.MaxFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
		},
		{
			name: "execution price underflow", capital: 1000, slip: 99,
			prices:  [][2]float64{{1, 1}, {1, 1}, {math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Sell}},
		},
		{
			name: "quantity overflow", capital: math.MaxFloat64,
			prices:  [][2]float64{{1, 1}, {math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
		},
		{
			name: "quantity underflow", capital: math.SmallestNonzeroFloat64,
			prices:  [][2]float64{{1, 1}, {math.MaxFloat64, math.MaxFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
		},
		{
			name: "valuation overflow", capital: 1000,
			prices:  [][2]float64{{1, 1}, {1, math.MaxFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
		},
		{
			name: "valuation underflow", capital: 1,
			prices:  [][2]float64{{1, 1}, {math.MaxFloat64, math.SmallestNonzeroFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
		},
		{
			name: "sale notional overflow", capital: 1000,
			prices:  [][2]float64{{1, 1}, {1, 1}, {math.MaxFloat64, math.MaxFloat64}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Sell}},
		},
		{
			name: "return overflow", capital: 1e-300,
			prices:  [][2]float64{{1, 1}, {1e-300, 1e100}},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candles := make([]backtest.Candle, len(tt.prices))
			for i, price := range tt.prices {
				candles[i] = backtest.Candle{
					OpenTime: time.Unix(int64(i), 0), Open: price[0], Close: price[1],
					High: math.Max(price[0], price[1]), Low: math.Min(price[0], price[1]),
				}
			}
			result, err := backtest.Run(candles, tt.signals, backtest.Config{
				InitialCapital: tt.capital, FeePercent: tt.fee, SlippagePercent: tt.slip,
			})
			assertFailedRun(t, result, err)
		})
	}
}
