// Package backtest simulates all-in spot trades from precomputed candle signals.
// It has no dependency on HTTP, exchanges, storage or strategy implementations.
package backtest

import "fmt"

// Run validates all inputs before simulation and never modifies them. Signals
// may be sparse and in any order; each candle index must occur at most once.
// Signals execute one candle later, before that candle's close valuation.
// A last-candle signal is validated but cannot execute. Open positions are
// marked at the final close and are never automatically sold.
// On invalid input or unrepresentable arithmetic, Run returns a zero result
// and an error rather than a partial or non-finite result.
func Run(candles []Candle, signals []Signal, cfg Config) (BacktestResult, error) {
	if err := cfg.Validate(); err != nil {
		return BacktestResult{}, err
	}
	if err := ValidateCandles(candles); err != nil {
		return BacktestResult{}, err
	}
	indexed, err := indexSignals(signals, len(candles))
	if err != nil {
		return BacktestResult{}, err
	}

	portfolio := Portfolio{Cash: cfg.InitialCapital}
	result := BacktestResult{
		InitialCapital: cfg.InitialCapital,
		Trades:         make([]Trade, 0),
		EquityCurve:    make([]EquityPoint, 0, len(candles)),
	}
	for i, candle := range candles {
		if i > 0 {
			trade, err := execute(&portfolio, indexed[i-1], candle, cfg)
			if err != nil {
				return BacktestResult{}, fmt.Errorf("candle %d: %w", i, err)
			}
			if trade != nil {
				result.Trades = append(result.Trades, *trade)
			}
		}
		if err := portfolio.mark(candle.Close); err != nil {
			return BacktestResult{}, fmt.Errorf("candle %d: %w", i, err)
		}
		result.EquityCurve = append(result.EquityCurve, EquityPoint{
			Time: candle.OpenTime.UTC(), Equity: portfolio.Equity,
		})
	}
	result.FinalEquity = portfolio.Equity
	result.TotalReturn = (result.FinalEquity/cfg.InitialCapital - 1) * 100
	if !finite(result.TotalReturn) {
		return BacktestResult{}, fmt.Errorf("total return exceeds float64 range")
	}
	result.TotalFees = portfolio.TotalFees
	result.Cash = portfolio.Cash
	result.AssetQuantity = portfolio.AssetQuantity
	return result, nil
}
