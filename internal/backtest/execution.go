package backtest

import (
	"fmt"
	"math"
)

// execute applies a signal to the next candle's open. It returns nil for Hold,
// Buy while holding an asset, and Sell without an asset.
func execute(p *Portfolio, signal SignalType, candle Candle, cfg Config) (*Trade, error) {
	if signal == Hold || (signal == Buy && p.AssetQuantity > 0) || (signal == Sell && p.AssetQuantity == 0) {
		return nil, nil
	}

	feeRate := cfg.FeePercent / 100
	slippage := cfg.SlippagePercent / 100
	price := candle.Open * (1 + slippage)
	if signal == Sell {
		price = candle.Open * (1 - slippage)
	}
	if !finite(price) || price <= 0 {
		return nil, fmt.Errorf("execution price exceeds float64 range")
	}

	quantity := p.AssetQuantity
	if signal == Buy {
		if p.Cash <= 0 {
			return nil, fmt.Errorf("insufficient cash for buy")
		}
		// Dividing in stages avoids overflowing price * (1 + feeRate).
		quantity = (p.Cash / (1 + feeRate)) / price
	}
	if !finite(quantity) || quantity <= 0 {
		return nil, fmt.Errorf("asset quantity exceeds float64 range")
	}

	notional := quantity * price
	fee := notional * feeRate
	if signal == Buy {
		// Rounding can put the all-in cost a few ulps above available cash.
		// Reduce quantity instead of allowing debt or discarding an unpaid fee.
		for notional+fee > p.Cash {
			quantity = math.Nextafter(quantity, 0)
			notional = quantity * price
			fee = notional * feeRate
		}
	}
	if !finite(notional) || notional <= 0 || !finite(fee) {
		return nil, fmt.Errorf("trade value exceeds float64 range")
	}

	cash := p.Cash + (notional - fee)
	assetQuantity := 0.0
	if signal == Buy {
		cash = p.Cash - (notional + fee)
		assetQuantity = quantity
	}
	totalFees := p.TotalFees + fee
	if !finite(cash) || cash < 0 || !finite(totalFees) {
		return nil, fmt.Errorf("trade balance exceeds float64 range")
	}
	p.Cash = cash
	p.AssetQuantity = assetQuantity
	p.TotalFees = totalFees
	return &Trade{
		Time:               candle.OpenTime.UTC(),
		Type:               signal,
		Price:              price,
		Quantity:           quantity,
		Notional:           notional,
		Fee:                fee,
		CashAfter:          cash,
		AssetQuantityAfter: assetQuantity,
	}, nil
}
