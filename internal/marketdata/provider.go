// Package marketdata loads closed, continuous OHLCV candles for backtesting.
package marketdata

import (
	"context"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

const MaxCandles = 3000

// Provider returns the complete closed portion of [start, end), or an error
// without partial data. Bounds must align with the interval in UTC.
type Provider interface {
	GetCandles(ctx context.Context, symbol, interval string, start, end time.Time) ([]backtest.Candle, error)
}
