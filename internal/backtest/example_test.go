package backtest_test

import (
	"fmt"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

func ExampleRun() {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []backtest.Candle{
		{OpenTime: start, Open: 100, High: 100, Low: 100, Close: 100},
		{OpenTime: start.Add(time.Hour), Open: 100, High: 110, Low: 100, Close: 110},
		{OpenTime: start.Add(2 * time.Hour), Open: 120, High: 120, Low: 120, Close: 120},
	}
	signals := []backtest.Signal{
		{CandleIndex: 0, Type: backtest.Buy},
		{CandleIndex: 1, Type: backtest.Sell},
	}
	cfg := backtest.DefaultConfig()
	cfg.InitialCapital = 1010
	cfg.FeePercent = 1
	result, err := backtest.Run(candles, signals, cfg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Equity: %.2f, fees: %.2f, return: %.2f%%\n",
		result.FinalEquity, result.TotalFees, result.TotalReturn)
	// Output: Equity: 1188.00, fees: 22.00, return: 17.62%
}
