package marketdata

import (
	"errors"
	"fmt"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

type candleRequest struct {
	symbol   string
	interval string
	start    time.Time
	end      time.Time
	step     time.Duration
	count    int
}

// Range is the validated, closed UTC portion of a requested market interval.
type Range struct {
	Start time.Time
	End   time.Time
	Step  time.Duration
	Count int
}

// ValidateRange shares provider limits with callers that must validate before
// doing I/O (including extra history required by a strategy).
func ValidateRange(symbol, interval string, start, end, now time.Time) (Range, error) {
	r, err := prepareRequest(symbol, interval, start, end, now)
	if err != nil {
		return Range{}, err
	}
	return Range{Start: r.start, End: r.end, Step: r.step, Count: r.count}, nil
}

func (r Range) ValidateCandles(candles []backtest.Candle) error {
	return validateCoverage(candles, candleRequest{start: r.Start, end: r.End, step: r.Step, count: r.Count})
}

func instrument(symbol, interval string) (time.Duration, time.Duration, error) {
	switch symbol {
	case "BTCUSDT", "ETHUSDT", "SOLUSDT":
	default:
		return 0, 0, fmt.Errorf("unsupported symbol %q: use BTCUSDT, ETHUSDT or SOLUSDT", symbol)
	}
	switch interval {
	case "1h":
		return time.Hour, 90 * 24 * time.Hour, nil
	case "4h":
		return 4 * time.Hour, 365 * 24 * time.Hour, nil
	case "1d":
		return 24 * time.Hour, 365 * 24 * time.Hour, nil
	default:
		return 0, 0, fmt.Errorf("unsupported interval %q: use 1h, 4h or 1d", interval)
	}
}

func prepareRequest(symbol, interval string, start, end, now time.Time) (candleRequest, error) {
	step, maxPeriod, err := instrument(symbol, interval)
	if err != nil {
		return candleRequest{}, err
	}
	start, end = start.UTC(), end.UTC()
	if start.Year() < 1970 || end.Year() > 9999 || !start.Before(end) {
		return candleRequest{}, errors.New("require 1970 <= start < end <= year 9999")
	}
	if !start.Equal(start.Truncate(step)) || !end.Equal(end.Truncate(step)) {
		return candleRequest{}, fmt.Errorf("start and end must align with %s candle boundaries in UTC", interval)
	}
	span := end.Sub(start)
	if span/step > MaxCandles {
		return candleRequest{}, fmt.Errorf("requested range exceeds %d candles", MaxCandles)
	}
	if span > maxPeriod {
		return candleRequest{}, fmt.Errorf("requested %s range exceeds %d days", interval, int(maxPeriod/(24*time.Hour)))
	}
	// Freeze the cutoff for the entire load. A candle is usable only after its
	// complete interval has elapsed, even if a page arrives after the next hour.
	cutoff := now.UTC().Truncate(step)
	if end.After(cutoff) {
		end = cutoff
	}
	if !start.Before(end) {
		return candleRequest{}, errors.New("requested range contains no closed candles")
	}
	return candleRequest{symbol: symbol, interval: interval, start: start, end: end, step: step, count: int(end.Sub(start) / step)}, nil
}

func validateCoverage(candles []backtest.Candle, request candleRequest) error {
	if err := backtest.ValidateCandles(candles); err != nil {
		return err
	}
	if len(candles) != request.count {
		return fmt.Errorf("insufficient data: got %d candles, want %d", len(candles), request.count)
	}
	for i, candle := range candles {
		expected := request.start.Add(time.Duration(i) * request.step)
		if !candle.OpenTime.Equal(expected) {
			return fmt.Errorf("candle gap or unexpected timestamp: want %s, got %s", expected.Format(time.RFC3339), candle.OpenTime.Format(time.RFC3339))
		}
	}
	return nil
}
