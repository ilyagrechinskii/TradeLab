package strategy_test

import (
	"fmt"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

func exampleCandles() []backtest.Candle {
	closes := []float64{3, 2, 1, 2, 3, 4, 3, 2, 1}
	candles := make([]backtest.Candle, len(closes))
	for i, price := range closes {
		candles[i] = backtest.Candle{
			OpenTime: time.Unix(int64(i)*3600, 0).UTC(),
			Open:     price, High: price, Low: price, Close: price,
		}
	}
	candles[5].Open, candles[5].High = 10, 10
	candles[8].Open, candles[8].High = 20, 20
	return candles
}

func ExampleSMACrossover_Generate() {
	candles := exampleCandles()
	selected := strategy.SMACrossover{FastPeriod: 2, SlowPeriod: 3}
	signals, err := selected.Generate(candles)
	if err != nil {
		panic(err)
	}
	result, err := backtest.Run(candles, signals, backtest.Config{InitialCapital: 1000})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Orders: %d, return: %.2f%%\n", len(result.Trades), result.TotalReturn)
	// Output: Orders: 2, return: 100.00%
}

func ExampleCompareWithBuyAndHold() {
	comparison, err := strategy.CompareWithBuyAndHold(
		exampleCandles(),
		strategy.SMACrossover{FastPeriod: 2, SlowPeriod: 3},
		backtest.Config{InitialCapital: 1000},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("SMA: %.2f%%, Buy & Hold: %.2f%%, difference: %.2f pp\n",
		comparison.Strategy.Result.TotalReturn,
		comparison.BuyAndHold.Result.TotalReturn,
		comparison.ExcessReturn)
	// Output: SMA: 100.00%, Buy & Hold: -50.00%, difference: 150.00 pp
}
