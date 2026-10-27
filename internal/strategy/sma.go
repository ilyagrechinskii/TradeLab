package strategy

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

// SMAPoint exists only for a complete window; no zero/NaN warm-up placeholders.
type SMAPoint struct {
	CandleIndex int
	Time        time.Time
	Value       float64
}

// SMA calculates a trailing Close average in O(n) time with a rolling sum.
// There must be at least period candles. Results start at index period-1.
func SMA(candles []backtest.Candle, period int) ([]SMAPoint, error) {
	if period <= 0 {
		return nil, errors.New("SMA period must be positive")
	}
	if err := backtest.ValidateCandles(candles); err != nil {
		return nil, err
	}
	if len(candles) < period {
		return nil, fmt.Errorf("SMA requires at least %d candles", period)
	}
	return sma(context.Background(), candles, period)
}

// sma assumes validated candles and a period in [1, len(candles)].
func sma(ctx context.Context, candles []backtest.Candle, period int) ([]SMAPoint, error) {
	points := make([]SMAPoint, 0, len(candles)-period+1)
	var sum, correction float64
	// Compensated additions/subtractions reduce accumulated rolling-window error.
	add := func(value float64) {
		adjusted := value - correction
		next := sum + adjusted
		correction = (next - sum) - adjusted
		sum = next
	}
	for i, candle := range candles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if i >= period {
			add(-candles[i-period].Close)
		}
		add(candle.Close)
		if math.IsNaN(sum) || math.IsInf(sum, 0) {
			return nil, fmt.Errorf("SMA at candle %d exceeds float64 range", i)
		}
		if i >= period-1 {
			value := sum / float64(period)
			if value <= 0 {
				return nil, fmt.Errorf("SMA at candle %d is outside positive float64 range", i)
			}
			points = append(points, SMAPoint{CandleIndex: i, Time: candle.OpenTime.UTC(), Value: value})
		}
	}
	return points, nil
}
