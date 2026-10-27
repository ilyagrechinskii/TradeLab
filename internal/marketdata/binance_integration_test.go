package marketdata_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
)

func TestBinanceIntegration(t *testing.T) {
	if os.Getenv("TRADELAB_BINANCE_INTEGRATION") != "1" {
		t.Skip("set TRADELAB_BINANCE_INTEGRATION=1 to explicitly enable real Binance requests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := marketdata.NewBinance(marketdata.BinanceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles, err := client.GetCandles(ctx, "BTCUSDT", "1h", start, start.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 6 {
		t.Fatalf("received %d candles, want 6", len(candles))
	}
	t.Logf("received %d real candles: %s through %s", len(candles), candles[0].OpenTime, candles[len(candles)-1].OpenTime)
}
