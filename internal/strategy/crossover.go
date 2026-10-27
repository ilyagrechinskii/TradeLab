package strategy

import (
	"errors"
	"fmt"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

type SMACrossover struct {
	FastPeriod int
	SlowPeriod int
}

func DefaultSMACrossover() SMACrossover {
	return SMACrossover{FastPeriod: 10, SlowPeriod: 30}
}

type CrossoverResult struct {
	Signals []backtest.Signal
	FastSMA []SMAPoint
	SlowSMA []SMAPoint
}

func (s SMACrossover) Validate() error {
	if s.FastPeriod <= 0 || s.SlowPeriod <= 0 || s.FastPeriod >= s.SlowPeriod {
		return errors.New("SMA periods must satisfy 0 < FastPeriod < SlowPeriod")
	}
	return nil
}

func (s SMACrossover) Generate(candles []backtest.Candle) ([]backtest.Signal, error) {
	analysis, err := s.Analyze(candles)
	if err != nil {
		return nil, err
	}
	return analysis.Signals, nil
}

// Analyze exposes both SMA series for charting as well as crossover signals.
// Equality is neutral: returning to the same side after touching does not
// create another signal. The last strictly unequal side is remembered.
func (s SMACrossover) Analyze(candles []backtest.Candle) (CrossoverResult, error) {
	if err := s.Validate(); err != nil {
		return CrossoverResult{}, err
	}
	if err := backtest.ValidateCandles(candles); err != nil {
		return CrossoverResult{}, err
	}
	// Two consecutive complete slow windows are required for a crossover.
	if len(candles) <= s.SlowPeriod {
		return CrossoverResult{}, fmt.Errorf("SMA crossover requires more than %d candles", s.SlowPeriod)
	}
	fast, err := sma(candles, s.FastPeriod)
	if err != nil {
		return CrossoverResult{}, err
	}
	slow, err := sma(candles, s.SlowPeriod)
	if err != nil {
		return CrossoverResult{}, err
	}
	result := CrossoverResult{Signals: make([]backtest.Signal, 0), FastSMA: fast, SlowSMA: slow}
	lastSide := 0
	for j, point := range slow {
		fastValue := fast[point.CandleIndex-s.FastPeriod+1].Value
		side := 0
		if fastValue > point.Value {
			side = 1
		} else if fastValue < point.Value {
			side = -1
		}
		if j > 0 && side != 0 && side != lastSide {
			signal := backtest.Buy
			if side < 0 {
				signal = backtest.Sell
			}
			result.Signals = append(result.Signals, backtest.Signal{CandleIndex: point.CandleIndex, Type: signal})
		}
		if side != 0 {
			lastSide = side
		}
	}
	return result, nil
}
