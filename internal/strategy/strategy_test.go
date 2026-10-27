package strategy_test

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

func candlesWithCloses(closes ...float64) []backtest.Candle {
	start := time.Date(2026, 1, 1, 3, 0, 0, 0, time.FixedZone("UTC+3", 3*3600))
	candles := make([]backtest.Candle, len(closes))
	for i, price := range closes {
		candles[i] = backtest.Candle{
			OpenTime: start.Add(time.Duration(i) * time.Hour),
			Open:     price, High: price, Low: price, Close: price,
		}
	}
	return candles
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-10*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %.15g, want %.15g", name, got, want)
	}
}

func TestSMA(t *testing.T) {
	candles := candlesWithCloses(1, 2, 3, 4, 5)
	for _, tt := range []struct {
		period int
		want   []float64
	}{
		{1, []float64{1, 2, 3, 4, 5}},
		{3, []float64{2, 3, 4}},
		{5, []float64{3}},
	} {
		points, err := strategy.SMA(candles, tt.period)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != len(tt.want) {
			t.Fatalf("point count = %d, want %d", len(points), len(tt.want))
		}
		for i, point := range points {
			index := i + tt.period - 1
			near(t, "SMA", point.Value, tt.want[i])
			if point.CandleIndex != index || !point.Time.Equal(candles[index].OpenTime) || point.Time.Location() != time.UTC {
				t.Errorf("unexpected point index/time: %+v", point)
			}
		}
	}
}

func TestSMAInvalidInput(t *testing.T) {
	for _, period := range []int{-1, 0, 4, math.MaxInt} {
		if _, err := strategy.SMA(candlesWithCloses(1, 2, 3), period); err == nil {
			t.Errorf("accepted invalid/insufficient period %d", period)
		}
	}
	if _, err := strategy.SMA(nil, 1); err == nil {
		t.Error("accepted empty candles")
	}
	if _, err := strategy.SMA(candlesWithCloses(1, math.NaN()), 1); err == nil {
		t.Error("accepted invalid candles")
	}
	if _, err := strategy.SMA(candlesWithCloses(math.MaxFloat64, math.MaxFloat64), 2); err == nil {
		t.Error("accepted rolling sum overflow")
	}
}

func TestCrossoverSignals(t *testing.T) {
	for _, tt := range []struct {
		name   string
		closes []float64
		fast   int
		slow   int
		want   []backtest.Signal
	}{
		{
			name: "up and down crossings without repeat", closes: []float64{3, 2, 1, 2, 3, 4, 3, 2, 1}, fast: 2, slow: 3,
			want: []backtest.Signal{{CandleIndex: 4, Type: backtest.Buy}, {CandleIndex: 7, Type: backtest.Sell}},
		},
		{
			name: "equal windows and same-side touches", closes: []float64{1, 1, 2, 2, 3, 3, 2, 2, 1}, fast: 1, slow: 2,
			want: []backtest.Signal{{CandleIndex: 2, Type: backtest.Buy}, {CandleIndex: 6, Type: backtest.Sell}},
		},
		{
			name: "crossing through equality", closes: []float64{3, 2, 2, 3}, fast: 1, slow: 2,
			want: []backtest.Signal{{CandleIndex: 3, Type: backtest.Buy}},
		},
		{
			name: "downward crossing through equality", closes: []float64{1, 2, 2, 1}, fast: 1, slow: 2,
			want: []backtest.Signal{{CandleIndex: 3, Type: backtest.Sell}},
		},
		{name: "always above is not a crossing", closes: []float64{1, 2, 3, 4, 5}, fast: 1, slow: 2, want: []backtest.Signal{}},
		{name: "always below is not a crossing", closes: []float64{5, 4, 3, 2, 1}, fast: 1, slow: 2, want: []backtest.Signal{}},
		{name: "always equal", closes: []float64{2, 2, 2, 2}, fast: 1, slow: 2, want: []backtest.Signal{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := strategy.SMACrossover{FastPeriod: tt.fast, SlowPeriod: tt.slow}
			got, err := s.Generate(candlesWithCloses(tt.closes...))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("signals = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCrossoverSeries(t *testing.T) {
	candles := candlesWithCloses(3, 2, 1, 2, 3, 4, 3, 2, 1)
	analysis, err := (strategy.SMACrossover{FastPeriod: 2, SlowPeriod: 3}).Analyze(candles)
	if err != nil {
		t.Fatal(err)
	}
	for _, series := range []struct {
		points []strategy.SMAPoint
		want   []float64
	}{
		{analysis.FastSMA, []float64{2.5, 1.5, 1.5, 2.5, 3.5, 3.5, 2.5, 1.5}},
		{analysis.SlowSMA, []float64{2, 1.6666666666666667, 2, 3, 3.3333333333333333, 3, 2}},
	} {
		if len(series.points) != len(series.want) {
			t.Fatalf("unexpected series length %d", len(series.points))
		}
		for i, point := range series.points {
			near(t, "series value", point.Value, series.want[i])
		}
	}
}

func TestConstantPricesDoNotGenerateCrossovers(t *testing.T) {
	for _, price := range []float64{0.01, 0.1, 0.3, 99.99, 123.45} {
		closes := make([]float64, 64)
		for i := range closes {
			closes[i] = price
		}
		for _, selected := range []strategy.SMACrossover{
			{FastPeriod: 1, SlowPeriod: 3},
			{FastPeriod: 2, SlowPeriod: 3},
			strategy.DefaultSMACrossover(),
		} {
			signals, err := selected.Generate(candlesWithCloses(closes...))
			if err != nil {
				t.Fatal(err)
			}
			if len(signals) != 0 {
				t.Errorf("constant price %g with %+v generated %v", price, selected, signals)
			}
		}
	}
}

func TestCrossoverValidation(t *testing.T) {
	defaults := strategy.DefaultSMACrossover()
	if defaults.FastPeriod != 10 || defaults.SlowPeriod != 30 {
		t.Errorf("unexpected defaults: %+v", defaults)
	}
	for _, s := range []strategy.SMACrossover{
		{}, {FastPeriod: -1, SlowPeriod: 3}, {FastPeriod: 1, SlowPeriod: 0},
		{FastPeriod: 3, SlowPeriod: 3}, {FastPeriod: 4, SlowPeriod: 3},
	} {
		if err := s.Validate(); err == nil {
			t.Errorf("accepted invalid periods: %+v", s)
		}
		if _, err := s.Generate(candlesWithCloses(1, 2, 3, 4)); err == nil {
			t.Errorf("generated signals with invalid periods: %+v", s)
		}
	}
	s := strategy.SMACrossover{FastPeriod: 1, SlowPeriod: 3}
	for _, candles := range [][]backtest.Candle{nil, candlesWithCloses(1, 2), candlesWithCloses(1, 2, 3)} {
		if _, err := s.Generate(candles); err == nil {
			t.Error("accepted insufficient candles for detecting a crossing")
		}
	}
	if _, err := (strategy.SMACrossover{FastPeriod: 1, SlowPeriod: math.MaxInt}).Generate(candlesWithCloses(1, 2, 3)); err == nil {
		t.Error("accepted a slow period larger than history")
	}
}

func TestCrossoverIsCausalAndDoesNotMutateCandles(t *testing.T) {
	candles := candlesWithCloses(3, 2, 1, 2, 3, 4, 100, 1, 200)
	original := append([]backtest.Candle(nil), candles...)
	s := strategy.SMACrossover{FastPeriod: 2, SlowPeriod: 3}
	full, err := s.Analyze(candles)
	if err != nil {
		t.Fatal(err)
	}
	for length := 4; length < len(candles); length++ {
		prefix, err := s.Analyze(candles[:length])
		if err != nil {
			t.Fatal(err)
		}
		want := make([]backtest.Signal, 0)
		for _, signal := range full.Signals {
			if signal.CandleIndex < length {
				want = append(want, signal)
			}
		}
		if !reflect.DeepEqual(prefix.Signals, want) ||
			!reflect.DeepEqual(prefix.FastSMA, full.FastSMA[:len(prefix.FastSMA)]) ||
			!reflect.DeepEqual(prefix.SlowSMA, full.SlowSMA[:len(prefix.SlowSMA)]) {
			t.Errorf("future candles changed prefix of length %d", length)
		}
	}
	if !reflect.DeepEqual(candles, original) {
		t.Error("strategy modified the candles")
	}
}

func TestBuyAndHold(t *testing.T) {
	candles := candlesWithCloses(100, 100, 120)
	s := strategy.BuyAndHold{}
	signals, err := s.Generate(candles)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(signals, []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}}) {
		t.Fatalf("unexpected signals: %v", signals)
	}
	cfg := backtest.Config{InitialCapital: 1111, FeePercent: 1, SlippagePercent: 10}
	report, err := strategy.Run(candles, s, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Result.Trades) != 1 || !report.Result.Trades[0].Time.Equal(candles[1].OpenTime) {
		t.Fatalf("unexpected executed trades: %v", report.Result.Trades)
	}
	near(t, "execution price", report.Result.Trades[0].Price, 110)
	near(t, "open asset quantity", report.Result.AssetQuantity, 10)
	near(t, "fee", report.Result.TotalFees, 11)
	near(t, "final equity", report.Result.FinalEquity, 1200)
	if report.Metrics.CompletedTrades != 0 || report.Metrics.WinRate != nil || report.Metrics.ProfitFactor != nil {
		t.Errorf("open position counted as completed: %+v", report.Metrics)
	}
	for _, input := range [][]backtest.Candle{nil, candles[:1], candlesWithCloses(100, -1)} {
		if _, err := s.Generate(input); err == nil {
			t.Error("accepted insufficient or invalid Buy & Hold input")
		}
	}
}

func comparisonCandles() []backtest.Candle {
	candles := candlesWithCloses(3, 2, 1, 2, 3, 4, 3, 2, 1)
	// Deliberately different next opens distinguish them from signal closes.
	candles[5].Open, candles[5].High = 10, 10
	candles[8].Open, candles[8].High = 20, 20
	return candles
}

func TestCrossoverExecutionAndComparison(t *testing.T) {
	candles := comparisonCandles()
	s := strategy.SMACrossover{FastPeriod: 2, SlowPeriod: 3}
	comparison, err := strategy.CompareWithBuyAndHold(candles, s, backtest.Config{InitialCapital: 1000})
	if err != nil {
		t.Fatal(err)
	}
	report := comparison.Strategy
	if len(report.Result.Trades) != 2 {
		t.Fatalf("trade count = %d, want 2", len(report.Result.Trades))
	}
	buy, sell := report.Result.Trades[0], report.Result.Trades[1]
	if !buy.Time.Equal(candles[5].OpenTime) || !sell.Time.Equal(candles[8].OpenTime) {
		t.Error("SMA signals did not execute on next candle")
	}
	near(t, "buy price", buy.Price, 10)
	near(t, "sell price", sell.Price, 20)
	near(t, "final equity", report.Result.FinalEquity, 2000)
	near(t, "strategy return", report.Metrics.TotalReturn, 100)
	near(t, "drawdown", report.Metrics.MaxDrawdown, 80)
	if report.Metrics.CompletedTrades != 1 || report.Metrics.WinRate == nil {
		t.Fatalf("unexpected completed trades: %+v", report.Metrics)
	}
	near(t, "win rate", *report.Metrics.WinRate, 100)
	near(t, "benchmark return", comparison.BuyAndHold.Result.TotalReturn, -50)
	near(t, "excess return", comparison.ExcessReturn, 150)
}

func TestComparisonUsesSameIntervalAndExecutionSettings(t *testing.T) {
	candles := comparisonCandles()
	cfg := backtest.Config{InitialCapital: 1111, FeePercent: 1, SlippagePercent: 10}
	s := strategy.SMACrossover{FastPeriod: 2, SlowPeriod: 3}
	comparison, err := strategy.CompareWithBuyAndHold(candles, s, cfg)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "strategy final equity", comparison.Strategy.Result.FinalEquity, 1782)
	near(t, "benchmark final equity", comparison.BuyAndHold.Result.FinalEquity, 500)
	near(t, "strategy fees", comparison.Strategy.Result.TotalFees, 29)
	near(t, "benchmark fees", comparison.BuyAndHold.Result.TotalFees, 11)
	for _, selected := range []strategy.Strategy{s, strategy.BuyAndHold{}} {
		signals, err := selected.Generate(candles)
		if err != nil {
			t.Fatal(err)
		}
		direct, err := backtest.Run(candles, signals, cfg)
		if err != nil {
			t.Fatal(err)
		}
		report := comparison.Strategy
		if _, ok := selected.(strategy.BuyAndHold); ok {
			report = comparison.BuyAndHold
		}
		if !reflect.DeepEqual(report.Result, direct) {
			t.Error("integration differs from the existing backtest.Run")
		}
		if len(report.Result.EquityCurve) != len(candles) {
			t.Error("comparison shifted or truncated the supplied interval")
		}
	}
}

func TestRunValidation(t *testing.T) {
	candles := candlesWithCloses(1, 2, 3)
	for _, tt := range []struct {
		selected strategy.Strategy
		cfg      backtest.Config
	}{
		{nil, backtest.DefaultConfig()},
		{strategy.BuyAndHold{}, backtest.Config{}},
		{strategy.SMACrossover{FastPeriod: 0, SlowPeriod: 2}, backtest.DefaultConfig()},
	} {
		if report, err := strategy.Run(candles, tt.selected, tt.cfg); err == nil || !reflect.DeepEqual(report, strategy.Report{}) {
			t.Errorf("expected zero report and error, got %+v, %v", report, err)
		}
		if comparison, err := strategy.CompareWithBuyAndHold(candles, tt.selected, tt.cfg); err == nil || !reflect.DeepEqual(comparison, strategy.Comparison{}) {
			t.Errorf("expected zero comparison and error, got %+v, %v", comparison, err)
		}
	}
}
