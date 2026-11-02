package marketdata_test

import (
	"context"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"net/http"
	"testing"
	"time"
)

func TestAllSupportedMarkets(t *testing.T) {
	expected := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "ADAUSDT", "DOGEUSDT", "LINKUSDT", "AVAXUSDT", "LTCUSDT", "TRXUSDT", "DOTUSDT"}
	markets := marketdata.Markets()
	if len(markets) != len(expected) {
		t.Fatalf("got %d markets", len(markets))
	}
	for i, symbol := range expected {
		if markets[i].Symbol != symbol || markets[i].Name == "" {
			t.Fatalf("invalid market at %d: %+v", i, markets[i])
		}
		t.Run(symbol, func(t *testing.T) {
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("symbol") != symbol {
					t.Errorf("wrong symbol")
				}
				writeRows(t, w, rowsAt(testStart, 2, time.Hour))
			}, testNow)
			candles, err := client.GetCandles(context.Background(), symbol, "1h", testStart, testStart.Add(2*time.Hour))
			if err != nil || len(candles) != 2 {
				t.Fatalf("candles=%d err=%v", len(candles), err)
			}
		})
	}
	markets[0].Symbol = "MUTATED"
	if marketdata.Markets()[0].Symbol != "BTCUSDT" {
		t.Fatal("catalog mutated")
	}
	if _, err := marketdata.ValidateRange("UNKNOWNUSDT", "1h", testStart, testStart.Add(time.Hour), testNow); err == nil {
		t.Fatal("unknown symbol accepted")
	}
}
