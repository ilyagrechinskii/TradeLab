// Package simulation coordinates bounded market-data loads and backtests.
// It has no HTTP dependency and leaves execution and metrics to backtest.
package simulation

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

const (
	DefaultTimeout     = 25 * time.Second
	DefaultConcurrency = 2
	MaxSMAPeriod       = marketdata.MaxCandles - 2
)

var ErrBusy = errors.New("heavy request capacity reached")

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type DataError struct{ Err error }

func (e *DataError) Error() string { return "market data unavailable" }
func (e *DataError) Unwrap() error { return e.Err }

type MarketRequest struct {
	Symbol   string
	Interval string
	Start    time.Time
	End      time.Time
}

type StrategySpec struct {
	Type       string
	FastPeriod int
	SlowPeriod int
}

type Request struct {
	Market   MarketRequest
	Strategy StrategySpec
	Config   backtest.Config
}

type CandleResult struct {
	Market  MarketRequest
	Source  string
	Candles []backtest.Candle
}

type Outcome struct {
	Market       MarketRequest
	Source       string
	Strategy     StrategySpec
	Config       backtest.Config
	Report       strategy.Report
	Benchmark    strategy.Report
	ExcessReturn float64
	FastSMA      []strategy.SMAPoint
	SlowSMA      []strategy.SMAPoint
}

type Options struct {
	Timeout       time.Duration
	MaxConcurrent int
	Source        string
	Now           func() time.Time
}

type Service struct {
	provider marketdata.Provider
	timeout  time.Duration
	slots    chan struct{}
	source   string
	now      func() time.Time
}

func NewService(provider marketdata.Provider, options Options) (*Service, error) {
	if provider == nil {
		return nil, errors.New("market data provider is required")
	}
	if options.Timeout == 0 {
		options.Timeout = DefaultTimeout
	}
	if options.MaxConcurrent == 0 {
		options.MaxConcurrent = DefaultConcurrency
	}
	if options.Timeout < 0 || options.MaxConcurrent < 1 || options.MaxConcurrent > 16 {
		return nil, errors.New("invalid resource limits")
	}
	if options.Source == "" {
		options.Source = "binance"
	}
	if options.Source != "binance" && options.Source != "fixture" {
		return nil, errors.New("unknown data source")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{provider: provider, timeout: options.Timeout, slots: make(chan struct{}, options.MaxConcurrent), source: options.Source, now: options.Now}, nil
}

func (s *Service) acquire(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	select {
	case s.slots <- struct{}{}:
		ctx, cancel := context.WithTimeout(ctx, s.timeout)
		return ctx, func() { cancel(); <-s.slots }, nil
	default:
		return nil, nil, ErrBusy
	}
}

func validateMarket(request MarketRequest, now time.Time) (marketdata.Range, error) {
	r, err := marketdata.ValidateRange(request.Symbol, request.Interval, request.Start, request.End, now)
	if err != nil {
		return marketdata.Range{}, &ValidationError{Message: err.Error()}
	}
	return r, nil
}

func (s *Service) load(ctx context.Context, request MarketRequest, r marketdata.Range) ([]backtest.Candle, error) {
	candles, err := s.provider.GetCandles(ctx, request.Symbol, request.Interval, r.Start, r.End)
	if err != nil {
		return nil, &DataError{Err: err}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.ValidateCandles(candles); err != nil {
		return nil, &DataError{Err: err}
	}
	return candles, nil
}

func actualMarket(request MarketRequest, r marketdata.Range) MarketRequest {
	request.Start, request.End = r.Start, r.End
	return request
}

func (s *Service) Candles(ctx context.Context, request MarketRequest) (CandleResult, error) {
	r, err := validateMarket(request, s.now())
	if err != nil {
		return CandleResult{}, err
	}
	ctx, release, err := s.acquire(ctx)
	if err != nil {
		return CandleResult{}, err
	}
	defer release()
	candles, err := s.load(ctx, request, r)
	if err != nil {
		return CandleResult{}, err
	}
	return CandleResult{Market: actualMarket(request, r), Source: s.source, Candles: candles}, nil
}

func validateStrategy(spec StrategySpec) (int, error) {
	switch spec.Type {
	case "buy_and_hold":
		if spec.FastPeriod != 0 || spec.SlowPeriod != 0 {
			return 0, &ValidationError{Message: "buy_and_hold does not accept SMA parameters"}
		}
		return 0, nil
	case "sma_crossover":
		sma := strategy.SMACrossover{FastPeriod: spec.FastPeriod, SlowPeriod: spec.SlowPeriod}
		if err := sma.Validate(); err != nil {
			return 0, &ValidationError{Message: err.Error()}
		}
		if spec.SlowPeriod > MaxSMAPeriod {
			return 0, &ValidationError{Message: "slow_period exceeds the candle budget"}
		}
		return spec.SlowPeriod, nil
	default:
		return 0, &ValidationError{Message: "unknown strategy type"}
	}
}

func report(ctx context.Context, candles []backtest.Candle, signals []backtest.Signal, cfg backtest.Config) (strategy.Report, error) {
	result, err := backtest.RunContext(ctx, candles, signals, cfg)
	if err != nil {
		return strategy.Report{}, err
	}
	metrics, err := backtest.CalculateMetricsContext(ctx, result)
	if err != nil {
		return strategy.Report{}, err
	}
	return strategy.Report{Result: result, Metrics: metrics}, nil
}

func (s *Service) Run(ctx context.Context, request Request) (Outcome, error) {
	now := s.now()
	r, err := validateMarket(request.Market, now)
	if err != nil {
		return Outcome{}, err
	}
	if r.Count < 2 {
		return Outcome{}, &ValidationError{Message: "backtest requires at least two closed candles"}
	}
	if err := request.Config.Validate(); err != nil {
		return Outcome{}, &ValidationError{Message: err.Error()}
	}
	warmup, err := validateStrategy(request.Strategy)
	if err != nil {
		return Outcome{}, err
	}
	loadRange, err := validateMarket(MarketRequest{Symbol: request.Market.Symbol, Interval: request.Market.Interval,
		Start: r.Start.Add(-time.Duration(warmup) * r.Step), End: request.Market.End}, now)
	if err != nil {
		return Outcome{}, err
	}
	ctx, release, err := s.acquire(ctx)
	if err != nil {
		return Outcome{}, err
	}
	defer release()
	all, err := s.load(ctx, request.Market, loadRange)
	if err != nil {
		return Outcome{}, err
	}
	trading := all[warmup:]
	out := Outcome{Market: actualMarket(request.Market, r), Source: s.source, Strategy: request.Strategy, Config: request.Config,
		FastSMA: make([]strategy.SMAPoint, 0), SlowSMA: make([]strategy.SMAPoint, 0)}
	var signals []backtest.Signal
	if warmup > 0 {
		analysis, err := (strategy.SMACrossover{FastPeriod: request.Strategy.FastPeriod, SlowPeriod: request.Strategy.SlowPeriod}).AnalyzeContext(ctx, all)
		if err != nil {
			return Outcome{}, calculationError(err)
		}
		// The warm-up initializes indicators and crossing state, never orders.
		// A signal at trading index 0 can execute only at trading index 1.
		for _, signal := range analysis.Signals {
			if signal.CandleIndex >= warmup {
				signal.CandleIndex -= warmup
				signals = append(signals, signal)
			}
		}
		out.FastSMA = tradingPoints(analysis.FastSMA, warmup)
		out.SlowSMA = tradingPoints(analysis.SlowSMA, warmup)
	} else {
		signals, err = (strategy.BuyAndHold{}).Generate(trading)
		if err != nil {
			return Outcome{}, calculationError(err)
		}
	}
	out.Report, err = report(ctx, trading, signals, request.Config)
	if err != nil {
		return Outcome{}, calculationError(err)
	}
	benchmarkSignals, err := (strategy.BuyAndHold{}).Generate(trading)
	if err != nil {
		return Outcome{}, calculationError(err)
	}
	out.Benchmark, err = report(ctx, trading, benchmarkSignals, request.Config)
	if err != nil {
		return Outcome{}, calculationError(err)
	}
	out.ExcessReturn = out.Report.Result.TotalReturn - out.Benchmark.Result.TotalReturn
	if math.IsNaN(out.ExcessReturn) || math.IsInf(out.ExcessReturn, 0) {
		return Outcome{}, calculationError(errors.New("numeric range"))
	}
	if err := ctx.Err(); err != nil {
		return Outcome{}, err
	}
	return out, nil
}

func tradingPoints(points []strategy.SMAPoint, warmup int) []strategy.SMAPoint {
	selected := make([]strategy.SMAPoint, 0)
	for _, point := range points {
		if point.CandleIndex >= warmup {
			point.CandleIndex -= warmup
			selected = append(selected, point)
		}
	}
	return selected
}

func calculationError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &ValidationError{Message: "simulation exceeds the supported numeric range"}
}
