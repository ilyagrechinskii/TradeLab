package httpapi

import (
	"encoding/json"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/simulation"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExpandedMarketCatalogAndBacktests(t *testing.T) {
	recorder := httptest.NewRecorder()
	fixtureHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/markets", nil))
	if recorder.Code != 200 {
		t.Fatalf("status %d", recorder.Code)
	}
	var catalog struct {
		Symbols     []string            `json:"symbols"`
		Instruments []marketdata.Market `json:"instruments"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Symbols) != 12 || len(catalog.Instruments) != 12 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	for _, symbol := range marketdata.SupportedSymbols() {
		t.Run(symbol, func(t *testing.T) {
			provider, err := marketdata.NewFixtureProvider(symbol, "1h", preparedCandles())
			if err != nil {
				t.Fatal(err)
			}
			handler := handlerWithProvider(t, provider, simulation.Options{Source: "fixture"})
			query := url.Values{"symbol": {symbol}, "interval": {"1h"}, "start": {"2024-01-01T03:00:00Z"}, "end": {"2024-01-01T09:00:00Z"}}
			candles := httptest.NewRecorder()
			handler.ServeHTTP(candles, httptest.NewRequest(http.MethodGet, "/api/v1/candles?"+query.Encode(), nil))
			if candles.Code != 200 {
				t.Fatalf("candles: %d %s", candles.Code, candles.Body.String())
			}
			body := requestBody()
			body["symbol"] = symbol
			result := postJSON(t, handler, body)
			if result.Code != 200 {
				t.Fatalf("backtest: %d %s", result.Code, result.Body.String())
			}
			var response backtestResponse
			if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Market.Symbol != symbol || response.Metrics.CompletedTrades != 1 || response.Metrics.TotalReturn != 100 {
				t.Fatalf("unexpected result: %+v", response.Metrics)
			}
		})
	}
}
