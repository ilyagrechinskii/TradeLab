package backtest

import (
	"fmt"
	"math"
	"time"
)

// Candle contains OHLCV data. OpenTime identifies the candle, not its close.
type Candle struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// ValidateCandles requires at least one valid candle in strictly increasing time
// order. Timestamps are compared as instants regardless of their time zones.
func ValidateCandles(candles []Candle) error {
	if len(candles) == 0 {
		return fmt.Errorf("at least one candle is required")
	}
	for i, candle := range candles {
		for _, price := range []float64{candle.Open, candle.High, candle.Low, candle.Close} {
			if !finite(price) || price <= 0 {
				return fmt.Errorf("candle %d: prices must be positive and finite", i)
			}
		}
		if candle.High < candle.Open || candle.High < candle.Close || candle.High < candle.Low {
			return fmt.Errorf("candle %d: high must be at least open, close and low", i)
		}
		if candle.Low > candle.Open || candle.Low > candle.Close || candle.Low > candle.High {
			return fmt.Errorf("candle %d: low must not exceed open, close or high", i)
		}
		if !finite(candle.Volume) || candle.Volume < 0 {
			return fmt.Errorf("candle %d: volume must be non-negative and finite", i)
		}
		if i > 0 && !candle.OpenTime.After(candles[i-1].OpenTime) {
			return fmt.Errorf("candle %d: timestamps must be strictly increasing", i)
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
