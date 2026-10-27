package backtest

import "time"

// Trade is an executed order, not a completed entry/exit pair. Notional is
// Price * Quantity, excluding Fee. Fee is always paid in the quote currency.
type Trade struct {
	Time               time.Time
	Type               SignalType
	Price              float64
	Quantity           float64
	Notional           float64
	Fee                float64
	CashAfter          float64
	AssetQuantityAfter float64
}

// EquityPoint is valued at the candle's close. Time is its OpenTime in UTC,
// used as a label because the input does not include candle duration/close time.
type EquityPoint struct {
	Time   time.Time
	Equity float64
}

type BacktestResult struct {
	InitialCapital float64
	FinalEquity    float64
	TotalReturn    float64 // Percentage points, not a fraction.
	TotalFees      float64
	Trades         []Trade
	EquityCurve    []EquityPoint
	Cash           float64
	AssetQuantity  float64
}
