package httpapi

import (
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/simulation"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

type marketResponse struct {
	Symbol   string    `json:"symbol"`
	Interval string    `json:"interval"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Source   string    `json:"source"`
}
type candle struct {
	OpenTime time.Time `json:"open_time"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
}
type candleResponse struct {
	Market  marketResponse `json:"market"`
	Candles []candle       `json:"candles"`
}
type strategyResponse struct {
	Type       string `json:"type"`
	FastPeriod int    `json:"fast_period,omitempty"`
	SlowPeriod int    `json:"slow_period,omitempty"`
}
type configurationResponse struct {
	InitialCapital  float64 `json:"initial_capital"`
	FeePercent      float64 `json:"fee_percent"`
	SlippagePercent float64 `json:"slippage_percent"`
}
type metricResponse struct {
	TotalReturn     float64  `json:"total_return"`
	MaxDrawdown     float64  `json:"max_drawdown"`
	CompletedTrades int      `json:"completed_trades"`
	WinRate         *float64 `json:"win_rate"`
	ProfitFactor    *float64 `json:"profit_factor"`
}
type portfolioResponse struct {
	FinalEquity   float64 `json:"final_equity"`
	Cash          float64 `json:"cash"`
	AssetQuantity float64 `json:"asset_quantity"`
	TotalFees     float64 `json:"total_fees"`
}
type benchmarkResponse struct {
	Strategy     string            `json:"strategy"`
	Metrics      metricResponse    `json:"metrics"`
	Portfolio    portfolioResponse `json:"portfolio"`
	ExcessReturn float64           `json:"excess_return"`
}
type tradeResponse struct {
	Time               time.Time `json:"time"`
	Type               string    `json:"type"`
	Price              float64   `json:"price"`
	Quantity           float64   `json:"quantity"`
	Notional           float64   `json:"notional"`
	Fee                float64   `json:"fee"`
	CashAfter          float64   `json:"cash_after"`
	AssetQuantityAfter float64   `json:"asset_quantity_after"`
}
type equityResponse struct {
	Time   time.Time `json:"time"`
	Equity float64   `json:"equity"`
}
type indicatorPoint struct {
	CandleIndex int       `json:"candle_index"`
	Time        time.Time `json:"time"`
	Value       float64   `json:"value"`
}
type indicatorsResponse struct {
	FastSMA []indicatorPoint `json:"fast_sma"`
	SlowSMA []indicatorPoint `json:"slow_sma"`
}
type backtestResponse struct {
	Market        marketResponse        `json:"market"`
	Strategy      strategyResponse      `json:"strategy"`
	Configuration configurationResponse `json:"configuration"`
	Metrics       metricResponse        `json:"metrics"`
	Portfolio     portfolioResponse     `json:"portfolio"`
	Benchmark     benchmarkResponse     `json:"benchmark"`
	Trades        []tradeResponse       `json:"trades"`
	EquityCurve   []equityResponse      `json:"equity_curve"`
	Indicators    indicatorsResponse    `json:"indicators"`
}

func marketDTO(r simulation.MarketRequest, source string) marketResponse {
	return marketResponse{Symbol: r.Symbol, Interval: r.Interval, Start: r.Start.UTC(), End: r.End.UTC(), Source: source}
}
func candleDTOs(candles []backtest.Candle) []candle {
	result := make([]candle, 0, len(candles))
	for _, c := range candles {
		result = append(result, candle{OpenTime: c.OpenTime.UTC(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume})
	}
	return result
}
func metricsDTO(m backtest.Metrics) metricResponse {
	return metricResponse{TotalReturn: m.TotalReturn, MaxDrawdown: m.MaxDrawdown, CompletedTrades: m.CompletedTrades, WinRate: m.WinRate, ProfitFactor: m.ProfitFactor}
}
func portfolioDTO(r backtest.BacktestResult) portfolioResponse {
	return portfolioResponse{FinalEquity: r.FinalEquity, Cash: r.Cash, AssetQuantity: r.AssetQuantity, TotalFees: r.TotalFees}
}
func indicatorDTOs(points []strategy.SMAPoint) []indicatorPoint {
	result := make([]indicatorPoint, 0, len(points))
	for _, p := range points {
		result = append(result, indicatorPoint{CandleIndex: p.CandleIndex, Time: p.Time.UTC(), Value: p.Value})
	}
	return result
}
func backtestDTO(out simulation.Outcome) backtestResponse {
	r := backtestResponse{
		Market: marketDTO(out.Market, out.Source), Strategy: strategyResponse{Type: out.Strategy.Type, FastPeriod: out.Strategy.FastPeriod, SlowPeriod: out.Strategy.SlowPeriod},
		Configuration: configurationResponse{InitialCapital: out.Config.InitialCapital, FeePercent: out.Config.FeePercent, SlippagePercent: out.Config.SlippagePercent},
		Metrics:       metricsDTO(out.Report.Metrics), Portfolio: portfolioDTO(out.Report.Result),
		Benchmark: benchmarkResponse{Strategy: "buy_and_hold", Metrics: metricsDTO(out.Benchmark.Metrics), Portfolio: portfolioDTO(out.Benchmark.Result), ExcessReturn: out.ExcessReturn},
		Trades:    make([]tradeResponse, 0, len(out.Report.Result.Trades)), EquityCurve: make([]equityResponse, 0, len(out.Report.Result.EquityCurve)),
		Indicators: indicatorsResponse{FastSMA: indicatorDTOs(out.FastSMA), SlowSMA: indicatorDTOs(out.SlowSMA)},
	}
	for _, t := range out.Report.Result.Trades {
		side := "buy"
		if t.Type == backtest.Sell {
			side = "sell"
		}
		r.Trades = append(r.Trades, tradeResponse{Time: t.Time.UTC(), Type: side, Price: t.Price, Quantity: t.Quantity, Notional: t.Notional, Fee: t.Fee, CashAfter: t.CashAfter, AssetQuantityAfter: t.AssetQuantityAfter})
	}
	for _, p := range out.Report.Result.EquityCurve {
		r.EquityCurve = append(r.EquityCurve, equityResponse{Time: p.Time.UTC(), Equity: p.Equity})
	}
	return r
}

func markets() any {
	return map[string]any{
		"symbols": marketdata.SupportedSymbols(), "instruments": marketdata.Markets(), "intervals": []string{"1h", "4h", "1d"},
		"limits": map[string]any{"max_candles": marketdata.MaxCandles, "max_days": map[string]int{"1h": 90, "4h": 365, "1d": 365}, "range": "[start, end)", "time_zone": "UTC", "warmup_included_in_limits": true},
	}
}
func strategies() any {
	return map[string]any{
		"strategies": []any{
			map[string]any{"type": "sma_crossover", "parameters": map[string]any{
				"fast_period": map[string]int{"default": 10, "minimum": 1, "maximum": simulation.MaxSMAPeriod - 1},
				"slow_period": map[string]int{"default": 30, "minimum": 2, "maximum": simulation.MaxSMAPeriod},
			}, "constraints": []string{"fast_period < slow_period", "slow_period warmup candles count toward market limits", "at least two trading candles"}},
			map[string]any{"type": "buy_and_hold", "parameters": map[string]any{}, "constraints": []string{"at least two trading candles"}},
		},
		"configuration": map[string]any{
			"initial_capital":  map[string]any{"default": 10000, "minimum_exclusive": 0},
			"fee_percent":      map[string]any{"default": 0.1, "minimum": 0, "maximum_exclusive": 100},
			"slippage_percent": map[string]any{"default": 0, "minimum": 0, "maximum_exclusive": 100},
		},
	}
}
