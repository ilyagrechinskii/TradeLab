package backtest

import "fmt"

// Portfolio is the current account state. AssetValue and Equity are marked at
// CurrentPrice, which Run sets to the close of each processed candle.
type Portfolio struct {
	Cash          float64
	AssetQuantity float64
	CurrentPrice  float64
	AssetValue    float64
	Equity        float64
	TotalFees     float64
}

func (p *Portfolio) mark(price float64) error {
	assetValue := p.AssetQuantity * price
	equity := p.Cash + assetValue
	if !finite(assetValue) || !finite(equity) || (p.AssetQuantity > 0 && assetValue == 0) {
		return fmt.Errorf("portfolio valuation exceeds float64 range")
	}
	p.CurrentPrice = price
	p.AssetValue = assetValue
	p.Equity = equity
	return nil
}
