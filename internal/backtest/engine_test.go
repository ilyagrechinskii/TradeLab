package backtest_test

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

func closeEnough(t *testing.T, name string, got, want float64) {
	t.Helper()
	const epsilon = 1e-10
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > epsilon*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %.15g, want %.15g", name, got, want)
	}
}

func fixture() []backtest.Candle {
	start := time.Date(2026, 1, 1, 3, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	return []backtest.Candle{
		{OpenTime: start, Open: 10, High: 50, Low: 10, Close: 50, Volume: 1},
		{OpenTime: start.Add(time.Hour), Open: 100, High: 120, Low: 100, Close: 120, Volume: 2},
		{OpenTime: start.Add(2 * time.Hour), Open: 150, High: 180, Low: 150, Close: 180, Volume: 3},
	}
}

func TestRunTradingScenarios(t *testing.T) {
	for _, tt := range []struct {
		name         string
		candleCount  int
		cfg          backtest.Config
		signals      []backtest.Signal
		prices       []float64
		notionals    []float64
		fees         []float64
		cashAfter    []float64
		equity       []float64
		wantCash     float64
		wantQuantity float64
		wantFees     float64
		wantReturn   float64
	}{
		{
			name: "buy without fee", candleCount: 2,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
			prices:  []float64{100}, notionals: []float64{1000}, fees: []float64{0}, cashAfter: []float64{0},
			equity: []float64{1000, 1200}, wantQuantity: 10, wantReturn: 20,
		},
		{
			name: "buy with fee", candleCount: 2,
			cfg:     backtest.Config{InitialCapital: 1010, FeePercent: 1},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
			prices:  []float64{100}, notionals: []float64{1000}, fees: []float64{10}, cashAfter: []float64{0},
			equity: []float64{1010, 1200}, wantQuantity: 10, wantFees: 10, wantReturn: 18.81188118811881,
		},
		{
			name: "sell with fee", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1010, FeePercent: 1},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Sell}},
			prices:  []float64{100, 150}, notionals: []float64{1000, 1500}, fees: []float64{10, 15}, cashAfter: []float64{0, 1485},
			equity: []float64{1010, 1200, 1485}, wantCash: 1485, wantFees: 25, wantReturn: 47.02970297029703,
		},
		{
			name: "buy and sell with slippage", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1100, SlippagePercent: 10},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Sell}},
			prices:  []float64{110, 135}, notionals: []float64{1100, 1350}, fees: []float64{0, 0}, cashAfter: []float64{0, 1350},
			equity: []float64{1100, 1200, 1350}, wantCash: 1350, wantReturn: 22.72727272727273,
		},
		{
			name: "repeated buy ignored", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Buy}},
			prices:  []float64{100}, notionals: []float64{1000}, fees: []float64{0}, cashAfter: []float64{0},
			equity: []float64{1000, 1200, 1800}, wantQuantity: 10, wantReturn: 80,
		},
		{
			name: "sell without position ignored", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Sell}, {CandleIndex: 1, Type: backtest.Sell}},
			equity:  []float64{1000, 1000, 1000}, wantCash: 1000,
		},
		{
			name: "no signals", candleCount: 3,
			cfg:    backtest.Config{InitialCapital: 1000},
			equity: []float64{1000, 1000, 1000}, wantCash: 1000,
		},
		{
			name: "explicit hold", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Hold}},
			equity:  []float64{1000, 1000, 1000}, wantCash: 1000,
		},
		{
			name: "open position not liquidated", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
			prices:  []float64{100}, notionals: []float64{1000}, fees: []float64{0}, cashAfter: []float64{0},
			equity: []float64{1000, 1200, 1800}, wantQuantity: 10, wantReturn: 80,
		},
		{
			name: "last candle buy does not execute", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 2, Type: backtest.Buy}},
			equity:  []float64{1000, 1000, 1000}, wantCash: 1000,
		},
		{
			name: "last candle sell does not close position", candleCount: 3,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 2, Type: backtest.Sell}},
			prices:  []float64{100}, notionals: []float64{1000}, fees: []float64{0}, cashAfter: []float64{0},
			equity: []float64{1000, 1200, 1800}, wantQuantity: 10, wantReturn: 80,
		},
		{
			name: "single candle", candleCount: 1,
			cfg:     backtest.Config{InitialCapital: 1000},
			signals: []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}},
			equity:  []float64{1000}, wantCash: 1000,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candles := fixture()[:tt.candleCount]
			result, err := backtest.Run(candles, tt.signals, tt.cfg)
			if err != nil {
				t.Fatal(err)
			}
			closeEnough(t, "initial capital", result.InitialCapital, tt.cfg.InitialCapital)
			closeEnough(t, "final equity", result.FinalEquity, tt.equity[len(tt.equity)-1])
			closeEnough(t, "cash", result.Cash, tt.wantCash)
			closeEnough(t, "asset quantity", result.AssetQuantity, tt.wantQuantity)
			closeEnough(t, "total fees", result.TotalFees, tt.wantFees)
			closeEnough(t, "total return", result.TotalReturn, tt.wantReturn)
			if len(result.Trades) != len(tt.prices) {
				t.Fatalf("trade count = %d, want %d", len(result.Trades), len(tt.prices))
			}
			for i, trade := range result.Trades {
				closeEnough(t, "execution price", trade.Price, tt.prices[i])
				closeEnough(t, "quantity", trade.Quantity, 10)
				closeEnough(t, "notional", trade.Notional, tt.notionals[i])
				closeEnough(t, "fee", trade.Fee, tt.fees[i])
				closeEnough(t, "cash after", trade.CashAfter, tt.cashAfter[i])
				wantType, wantAsset := backtest.Buy, 10.0
				if i == 1 {
					wantType, wantAsset = backtest.Sell, 0
				}
				if trade.Type != wantType {
					t.Errorf("trade type = %v, want %v", trade.Type, wantType)
				}
				closeEnough(t, "asset after", trade.AssetQuantityAfter, wantAsset)
				if !trade.Time.Equal(candles[i+1].OpenTime) || trade.Time.Location() != time.UTC {
					t.Errorf("trade time = %v, want next candle open in UTC", trade.Time)
				}
			}
			assertEquity(t, result, candles, tt.equity)
		})
	}
}

func assertEquity(t *testing.T, result backtest.BacktestResult, candles []backtest.Candle, want []float64) {
	t.Helper()
	if len(result.EquityCurve) != len(want) {
		t.Fatalf("equity point count = %d, want %d", len(result.EquityCurve), len(want))
	}
	for i, point := range result.EquityCurve {
		closeEnough(t, "equity point", point.Equity, want[i])
		if !point.Time.Equal(candles[i].OpenTime) || point.Time.Location() != time.UTC {
			t.Errorf("equity point %d time = %v, want candle label in UTC", i, point.Time)
		}
	}
}

func TestRunBuyHoldSell(t *testing.T) {
	start := fixture()[0].OpenTime
	candles := []backtest.Candle{
		{OpenTime: start, Open: 90, High: 90, Low: 90, Close: 90},
		{OpenTime: start.Add(time.Hour), Open: 100, High: 120, Low: 100, Close: 120},
		{OpenTime: start.Add(2 * time.Hour), Open: 130, High: 140, Low: 130, Close: 140},
		{OpenTime: start.Add(3 * time.Hour), Open: 150, High: 150, Low: 110, Close: 110},
		{OpenTime: start.Add(4 * time.Hour), Open: 100, High: 100, Low: 100, Close: 100},
	}
	// Unsorted, sparse signals are accepted. Sell on candle 2 executes on 3.
	signals := []backtest.Signal{
		{CandleIndex: 2, Type: backtest.Sell},
		{CandleIndex: 0, Type: backtest.Buy},
		{CandleIndex: 1, Type: backtest.Hold},
	}
	cfg := backtest.Config{InitialCapital: 1111, FeePercent: 1, SlippagePercent: 10}
	result, err := backtest.Run(candles, signals, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trades) != 2 {
		t.Fatalf("trade count = %d, want 2", len(result.Trades))
	}
	buy, sell := result.Trades[0], result.Trades[1]
	closeEnough(t, "buy price", buy.Price, 110)
	closeEnough(t, "buy quantity", buy.Quantity, 10)
	closeEnough(t, "buy notional", buy.Notional, 1100)
	closeEnough(t, "buy fee", buy.Fee, 11)
	closeEnough(t, "buy cash", buy.CashAfter, 0)
	closeEnough(t, "buy asset", buy.AssetQuantityAfter, 10)
	closeEnough(t, "sell price", sell.Price, 135)
	closeEnough(t, "sell quantity", sell.Quantity, 10)
	closeEnough(t, "sell notional", sell.Notional, 1350)
	closeEnough(t, "sell fee", sell.Fee, 13.5)
	closeEnough(t, "sell cash", sell.CashAfter, 1336.5)
	closeEnough(t, "sell asset", sell.AssetQuantityAfter, 0)
	closeEnough(t, "final equity", result.FinalEquity, 1336.5)
	closeEnough(t, "cash", result.Cash, 1336.5)
	closeEnough(t, "asset", result.AssetQuantity, 0)
	closeEnough(t, "total fees", result.TotalFees, 24.5)
	closeEnough(t, "return", result.TotalReturn, 20.2970297029703)
	if buy.Type != backtest.Buy || sell.Type != backtest.Sell ||
		!buy.Time.Equal(candles[1].OpenTime) || !sell.Time.Equal(candles[3].OpenTime) {
		t.Errorf("unexpected trade sides or execution times: %+v", result.Trades)
	}
	assertEquity(t, result, candles, []float64{1111, 1200, 1400, 1336.5, 1336.5})
}

func TestRunDoesNotExecuteAtSignalClose(t *testing.T) {
	candles := fixture()
	result, err := backtest.Run(candles, []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}}, backtest.Config{InitialCapital: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("trade count = %d, want 1", len(result.Trades))
	}
	// Candle 0 closes at 50; candle 1 opens at 100 and closes at 120.
	// A same-close fill would buy 20 units; a next-close fill buys 8.333... .
	closeEnough(t, "price", result.Trades[0].Price, 100)
	closeEnough(t, "quantity", result.Trades[0].Quantity, 10)
	assertEquity(t, result, candles, []float64{1000, 1200, 1800})
	if !result.Trades[0].Time.Equal(candles[1].OpenTime) {
		t.Error("signal did not execute at the next candle's open")
	}
}

func TestRunCanReenterAfterSale(t *testing.T) {
	candles := fixture()
	last := candles[2]
	last.OpenTime = last.OpenTime.Add(time.Hour)
	candles = append(candles, last)
	result, err := backtest.Run(candles, []backtest.Signal{
		{CandleIndex: 0, Type: backtest.Buy},
		{CandleIndex: 1, Type: backtest.Sell},
		{CandleIndex: 2, Type: backtest.Buy},
	}, backtest.Config{InitialCapital: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trades) != 3 {
		t.Fatalf("trade count = %d, want 3", len(result.Trades))
	}
	closeEnough(t, "reentry quantity", result.AssetQuantity, 10)
	closeEnough(t, "final equity", result.FinalEquity, 1800)
	assertEquity(t, result, candles, []float64{1000, 1200, 1500, 1800})
}

func TestRunDoesNotMutateInputs(t *testing.T) {
	candles := fixture()
	signals := []backtest.Signal{{CandleIndex: 1, Type: backtest.Sell}, {CandleIndex: 0, Type: backtest.Buy}}
	originalCandles := append([]backtest.Candle(nil), candles...)
	originalSignals := append([]backtest.Signal(nil), signals...)
	first, err := backtest.Run(candles, signals, backtest.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	second, err := backtest.Run(candles, signals, backtest.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(candles, originalCandles) || !reflect.DeepEqual(signals, originalSignals) {
		t.Error("Run modified its inputs")
	}
	if !reflect.DeepEqual(first, second) {
		t.Error("identical inputs produced different results")
	}
}

func TestRunRoundingNeverOverspends(t *testing.T) {
	for _, price := range []float64{0.1, 0.3, 3, 7, 99.99, 43210.12345678} {
		candles := fixture()
		for i := range candles {
			candles[i].Open, candles[i].High = price, price
			candles[i].Low, candles[i].Close = price, price
		}
		cfg := backtest.DefaultConfig()
		result, err := backtest.Run(candles, []backtest.Signal{
			{CandleIndex: 0, Type: backtest.Buy}, {CandleIndex: 1, Type: backtest.Sell},
		}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Trades) != 2 {
			t.Fatalf("trade count = %d, want 2", len(result.Trades))
		}
		buy, sell := result.Trades[0], result.Trades[1]
		if buy.CashAfter < 0 || buy.Notional+buy.Fee > cfg.InitialCapital || result.Cash < 0 {
			t.Errorf("overspent at price %g: %+v", price, result)
		}
		closeEnough(t, "buy cash conservation", buy.CashAfter+buy.Notional+buy.Fee, 10000)
		closeEnough(t, "sell cash conservation", sell.CashAfter, buy.CashAfter+sell.Notional-sell.Fee)
		closeEnough(t, "roundtrip final equity", result.FinalEquity, 9980.01998001998)
		closeEnough(t, "roundtrip fees", result.TotalFees, 19.98001998001998)
	}
}

func TestRunReducesQuantityWhenRoundingExceedsCash(t *testing.T) {
	candles := fixture()[:2]
	for i := range candles {
		candles[i].Open, candles[i].High = 0.01, 0.01
		candles[i].Low, candles[i].Close = 0.01, 0.01
	}
	cfg := backtest.Config{InitialCapital: 0.01, FeePercent: 0.1}
	feeRate := cfg.FeePercent / 100
	rawQuantity := (cfg.InitialCapital / (1 + feeRate)) / candles[1].Open
	rawNotional := rawQuantity * candles[1].Open
	if rawNotional+rawNotional*feeRate <= cfg.InitialCapital {
		t.Fatal("fixture must exercise a rounded cost greater than available cash")
	}
	result, err := backtest.Run(candles, []backtest.Signal{{CandleIndex: 0, Type: backtest.Buy}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("trade count = %d, want 1", len(result.Trades))
	}
	trade := result.Trades[0]
	if trade.Quantity >= rawQuantity || trade.Notional+trade.Fee > cfg.InitialCapital || trade.CashAfter < 0 {
		t.Errorf("quantity was not safely reduced: %+v", trade)
	}
	closeEnough(t, "final equity", result.FinalEquity, 0.00999000999000999)
	closeEnough(t, "fee", result.TotalFees, 0.00000999000999000999)
}
