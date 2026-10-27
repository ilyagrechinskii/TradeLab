package marketdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

// FixtureProvider serves explicitly supplied test/demo data, never real quotes.
// It owns a copy of its data and returns independent copies to callers.
type FixtureProvider struct {
	symbol   string
	interval string
	candles  []backtest.Candle
}

var _ Provider = (*FixtureProvider)(nil)

func NewFixtureProvider(symbol, interval string, candles []backtest.Candle) (*FixtureProvider, error) {
	step, _, err := instrument(symbol, interval)
	if err != nil {
		return nil, err
	}
	if err := backtest.ValidateCandles(candles); err != nil {
		return nil, fmt.Errorf("fixture: %w", err)
	}
	owned := append([]backtest.Candle(nil), candles...)
	for i := range owned {
		owned[i].OpenTime = owned[i].OpenTime.UTC()
		if !owned[i].OpenTime.Equal(owned[i].OpenTime.Truncate(step)) ||
			(i > 0 && !owned[i].OpenTime.Equal(owned[i-1].OpenTime.Add(step))) {
			return nil, errors.New("fixture timestamps must align with the interval and have no gaps")
		}
	}
	return &FixtureProvider{symbol: symbol, interval: interval, candles: owned}, nil
}

func (f *FixtureProvider) GetCandles(ctx context.Context, symbol, interval string, start, end time.Time) ([]backtest.Candle, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	request, err := prepareRequest(symbol, interval, start, end, time.Now())
	if err != nil {
		return nil, err
	}
	if symbol != f.symbol || interval != f.interval {
		return nil, errors.New("fixture has no data for this symbol/interval")
	}
	candles := make([]backtest.Candle, 0, request.count)
	for _, candle := range f.candles {
		if !candle.OpenTime.Before(request.start) && candle.OpenTime.Before(request.end) {
			candles = append(candles, candle)
		}
	}
	if err := validateCoverage(candles, request); err != nil {
		return nil, fmt.Errorf("fixture: %w", err)
	}
	return candles, nil
}
