package strategy

import (
	"errors"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

type BuyAndHold struct{}

func (BuyAndHold) Generate(candles []backtest.Candle) ([]backtest.Signal, error) {
	if err := backtest.ValidateCandles(candles); err != nil {
		return nil, err
	}
	if len(candles) < 2 {
		return nil, errors.New("buy and hold requires at least two candles to execute a buy")
	}
	return []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}}, nil
}
