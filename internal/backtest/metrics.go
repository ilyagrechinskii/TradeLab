package backtest

import (
	"context"
	"errors"
	"fmt"
)

type Metrics struct {
	TotalReturn     float64
	MaxDrawdown     float64 // Non-negative percentage of the preceding equity peak.
	CompletedTrades int
	WinRate         *float64 // nil when no positions have been closed.
	ProfitFactor    *float64 // nil when there are no realized losses.
}

// CalculateMetrics consumes an engine result without changing it. Only closed
// buy/sell pairs contribute to WinRate and ProfitFactor. Both entry and exit
// fees are included; slippage is already reflected in the executed notionals.
// Break-even positions count as completed, but neither as wins nor losses.
// This supports the engine's one-position, all-in spot model, not partial exits.
func CalculateMetrics(result BacktestResult) (Metrics, error) {
	return CalculateMetricsContext(context.Background(), result)
}

func CalculateMetricsContext(ctx context.Context, result BacktestResult) (Metrics, error) {
	if err := ctx.Err(); err != nil {
		return Metrics{}, err
	}
	if !finite(result.InitialCapital) || result.InitialCapital <= 0 ||
		!finite(result.FinalEquity) || result.FinalEquity < 0 || !finite(result.TotalReturn) {
		return Metrics{}, errors.New("metrics require finite returns and valid capital/equity")
	}
	if len(result.EquityCurve) == 0 {
		return Metrics{}, errors.New("metrics require a non-empty equity curve")
	}
	metrics := Metrics{TotalReturn: result.TotalReturn}
	peak := result.InitialCapital
	for i, point := range result.EquityCurve {
		if err := ctx.Err(); err != nil {
			return Metrics{}, err
		}
		if !finite(point.Equity) || point.Equity < 0 {
			return Metrics{}, fmt.Errorf("equity point %d must be non-negative and finite", i)
		}
		if i > 0 && !point.Time.After(result.EquityCurve[i-1].Time) {
			return Metrics{}, fmt.Errorf("equity point %d is not in chronological order", i)
		}
		if point.Equity > peak {
			peak = point.Equity
		}
		drawdown := (peak - point.Equity) / peak * 100
		if drawdown > metrics.MaxDrawdown {
			metrics.MaxDrawdown = drawdown
		}
	}

	var entry *Trade
	var wins int
	var grossProfit, grossLoss float64
	for i := range result.Trades {
		if err := ctx.Err(); err != nil {
			return Metrics{}, err
		}
		trade := &result.Trades[i]
		if !finite(trade.Price) || trade.Price <= 0 || !finite(trade.Quantity) || trade.Quantity <= 0 ||
			!finite(trade.Notional) || trade.Notional <= 0 || !finite(trade.Fee) || trade.Fee < 0 || trade.Fee > trade.Notional {
			return Metrics{}, fmt.Errorf("trade %d contains invalid amounts", i)
		}
		if i > 0 && !trade.Time.After(result.Trades[i-1].Time) {
			return Metrics{}, fmt.Errorf("trade %d is not in chronological order", i)
		}
		switch trade.Type {
		case Buy:
			if entry != nil {
				return Metrics{}, fmt.Errorf("trade %d buys while a position is open", i)
			}
			if !finite(trade.Notional + trade.Fee) {
				return Metrics{}, fmt.Errorf("trade %d entry cost exceeds float64 range", i)
			}
			entry = trade
		case Sell:
			if entry == nil || trade.Quantity != entry.Quantity {
				return Metrics{}, fmt.Errorf("trade %d must close the entire open position", i)
			}
			pnl := (trade.Notional - trade.Fee) - (entry.Notional + entry.Fee)
			metrics.CompletedTrades++
			if pnl > 0 {
				wins++
				grossProfit += pnl
			} else if pnl < 0 {
				grossLoss -= pnl
			}
			entry = nil
		default:
			return Metrics{}, fmt.Errorf("trade %d is not an executed buy or sell", i)
		}
		if !finite(grossProfit) || !finite(grossLoss) {
			return Metrics{}, errors.New("realized PnL exceeds float64 range")
		}
	}
	if metrics.CompletedTrades > 0 {
		rate := float64(wins) / float64(metrics.CompletedTrades) * 100
		metrics.WinRate = &rate
	}
	if grossLoss > 0 {
		factor := grossProfit / grossLoss
		if !finite(factor) || (grossProfit > 0 && factor == 0) {
			return Metrics{}, errors.New("profit factor exceeds float64 range")
		}
		metrics.ProfitFactor = &factor
	}
	return metrics, nil
}
