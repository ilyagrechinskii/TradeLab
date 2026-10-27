package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/simulation"
)

var apiStart = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func preparedCandles() []backtest.Candle {
	closes := []float64{3, 2, 1, 2, 3, 4, 3, 2, 1}
	candles := make([]backtest.Candle, len(closes))
	for i, price := range closes {
		candles[i] = backtest.Candle{OpenTime: apiStart.Add(time.Duration(i) * time.Hour), Open: price, High: price, Low: price, Close: price, Volume: 10}
	}
	candles[5].Open, candles[5].High = 10, 10
	candles[8].Open, candles[8].High = 20, 20
	return candles
}

func fixtureHandler(t *testing.T) http.Handler {
	t.Helper()
	provider, err := marketdata.NewFixtureProvider("BTCUSDT", "1h", preparedCandles())
	if err != nil {
		t.Fatal(err)
	}
	return handlerWithProvider(t, provider, simulation.Options{Source: "fixture"})
}

func handlerWithProvider(t *testing.T, provider marketdata.Provider, options simulation.Options) http.Handler {
	t.Helper()
	service, err := simulation.NewService(provider, options)
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(Options{Service: service, AllowedOrigins: []string{"http://localhost:5173"}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func requestBody() map[string]any {
	return map[string]any{"symbol": "BTCUSDT", "interval": "1h", "start": "2024-01-01T03:00:00Z", "end": "2024-01-01T09:00:00Z",
		"initial_capital": 1000, "fee_percent": 0, "slippage_percent": 0, "strategy": map[string]any{"type": "sma_crossover", "fast_period": 2, "slow_period": 3}}
}

func postJSON(t *testing.T, handler http.Handler, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/backtests", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func decodeSuccess(t *testing.T, w *httptest.ResponseRecorder) backtestResponse {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var result backtestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
	var result errorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Error.Code != code || result.Error.Message == "" {
		t.Errorf("unexpected error: %+v", result)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("error is not JSON")
	}
}

func almost(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-10*math.Max(1, math.Abs(want)) {
		t.Errorf("%s=%g want=%g", name, got, want)
	}
}

func TestDiscovery(t *testing.T) {
	handler := fixtureHandler(t)
	for _, path := range []string{"/api/v1/markets", "/api/v1/strategies"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if path == "/api/v1/markets" && (!strings.Contains(w.Body.String(), "SOLUSDT") || !strings.Contains(w.Body.String(), "3000")) {
			t.Error("market discovery incomplete")
		}
		if path == "/api/v1/strategies" && (!strings.Contains(w.Body.String(), "buy_and_hold") || !strings.Contains(w.Body.String(), "slow_period") || !strings.Contains(w.Body.String(), "30")) {
			t.Error("strategy discovery incomplete")
		}
	}
}

func candleURL() string {
	return "/api/v1/candles?" + url.Values{"symbol": {"BTCUSDT"}, "interval": {"1h"}, "start": {"2024-01-01T03:00:00Z"}, "end": {"2024-01-01T09:00:00Z"}}.Encode()
}

func TestGetCandles(t *testing.T) {
	w := httptest.NewRecorder()
	fixtureHandler(t).ServeHTTP(w, httptest.NewRequest("GET", candleURL(), nil))
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var result candleResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Candles) != 6 || result.Market.Source != "fixture" || result.Candles[0].Close != 2 || !result.Market.Start.Equal(apiStart.Add(3*time.Hour)) {
		t.Errorf("unexpected candles: %+v", result)
	}
	if !strings.Contains(w.Body.String(), `"open_time":"2024-01-01T03:00:00Z"`) {
		t.Error("timestamp is not ISO8601 UTC snake_case")
	}
}

func TestSMABacktestWarmupAndBenchmark(t *testing.T) {
	w := postJSON(t, fixtureHandler(t), requestBody())
	result := decodeSuccess(t, w)
	if result.Market.Source != "fixture" || !result.Market.Start.Equal(apiStart.Add(3*time.Hour)) || !result.Market.End.Equal(apiStart.Add(9*time.Hour)) {
		t.Errorf("wrong trading interval: %+v", result.Market)
	}
	if len(result.EquityCurve) != 6 || len(result.Trades) != 2 {
		t.Fatalf("wrong curve/order counts: %+v", result)
	}
	if !result.Trades[0].Time.Equal(apiStart.Add(5*time.Hour)) || !result.Trades[1].Time.Equal(apiStart.Add(8*time.Hour)) {
		t.Error("crossing did not execute at next trading candle open")
	}
	almost(t, "buy price", result.Trades[0].Price, 10)
	almost(t, "sell price", result.Trades[1].Price, 20)
	almost(t, "return", result.Metrics.TotalReturn, 100)
	almost(t, "drawdown", result.Metrics.MaxDrawdown, 80)
	almost(t, "final equity", result.Portfolio.FinalEquity, 2000)
	almost(t, "benchmark return", result.Benchmark.Metrics.TotalReturn, -66.66666666666667)
	almost(t, "excess return", result.Benchmark.ExcessReturn, 166.66666666666667)
	if result.Metrics.CompletedTrades != 1 || result.Metrics.WinRate == nil || result.Metrics.ProfitFactor != nil {
		t.Errorf("unexpected metrics: %+v", result.Metrics)
	}
	almost(t, "win rate", *result.Metrics.WinRate, 100)
	wantEquity := []float64{1000, 1000, 400, 300, 200, 2000}
	for i, p := range result.EquityCurve {
		almost(t, "equity", p.Equity, wantEquity[i])
		if !p.Time.Equal(apiStart.Add(time.Duration(i+3) * time.Hour)) {
			t.Error("warmup leaked into equity curve")
		}
	}
	if len(result.Indicators.FastSMA) != 6 || len(result.Indicators.SlowSMA) != 6 {
		t.Fatal("indicator series do not cover the trading period")
	}
	for i, point := range result.Indicators.SlowSMA {
		if point.CandleIndex != i || point.Time.Before(result.Market.Start) {
			t.Error("warmup leaked into indicators")
		}
	}
	almost(t, "first slow SMA", result.Indicators.SlowSMA[0].Value, 1.6666666666666667)
	if strings.Contains(w.Body.String(), `"id"`) || !strings.Contains(w.Body.String(), `"profit_factor":null`) {
		t.Error("response has fake ID or non-null undefined metric")
	}
}

func TestBuyAndHoldBacktest(t *testing.T) {
	body := requestBody()
	body["strategy"] = map[string]any{"type": "buy_and_hold"}
	result := decodeSuccess(t, postJSON(t, fixtureHandler(t), body))
	if len(result.Trades) != 1 || result.Trades[0].Type != "buy" || !result.Trades[0].Time.Equal(apiStart.Add(4*time.Hour)) {
		t.Errorf("unexpected buy and hold orders: %+v", result.Trades)
	}
	if result.Metrics.CompletedTrades != 0 || result.Metrics.WinRate != nil || result.Metrics.ProfitFactor != nil || len(result.Indicators.FastSMA) != 0 {
		t.Errorf("open position counted as closed or SMA invented: %+v", result)
	}
	almost(t, "benchmark difference", result.Benchmark.ExcessReturn, 0)
	almost(t, "asset", result.Portfolio.AssetQuantity, 333.3333333333333)
}

func TestRequestDefaultsAndExplicitZero(t *testing.T) {
	body := requestBody()
	body["strategy"] = map[string]any{"type": "buy_and_hold"}
	delete(body, "initial_capital")
	delete(body, "fee_percent")
	delete(body, "slippage_percent")
	result := decodeSuccess(t, postJSON(t, fixtureHandler(t), body))
	if result.Configuration.InitialCapital != 10000 || result.Configuration.FeePercent != 0.1 || result.Configuration.SlippagePercent != 0 {
		t.Errorf("wrong configuration defaults: %+v", result.Configuration)
	}
	body["strategy"] = map[string]any{"type": "sma_crossover"}
	encoded, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/v1/backtests", bytes.NewReader(encoded))
	r.Header.Set("Content-Type", "application/json")
	request, err := decodeBacktest(httptest.NewRecorder(), r)
	if err != nil || request.Strategy.FastPeriod != 10 || request.Strategy.SlowPeriod != 30 {
		t.Errorf("wrong SMA defaults: %+v %v", request.Strategy, err)
	}
}

func TestValidationBeforeProvider(t *testing.T) {
	var calls atomic.Int32
	handler := handlerWithProvider(t, providerFunc(func(context.Context, string, string, time.Time, time.Time) ([]backtest.Candle, error) {
		calls.Add(1)
		return nil, errors.New("must not be called")
	}), simulation.Options{})
	for _, tt := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown symbol", func(b map[string]any) { b["symbol"] = "XRPUSDT" }},
		{"unknown interval", func(b map[string]any) { b["interval"] = "1m" }},
		{"bad time", func(b map[string]any) { b["start"] = "today" }},
		{"not UTC", func(b map[string]any) { b["start"] = "2024-01-01T03:00:00+03:00" }},
		{"off grid", func(b map[string]any) { b["start"] = "2024-01-01T03:01:00Z" }},
		{"reversed", func(b map[string]any) { b["end"] = b["start"] }},
		{"zero capital", func(b map[string]any) { b["initial_capital"] = 0 }},
		{"negative capital", func(b map[string]any) { b["initial_capital"] = -1 }},
		{"null capital", func(b map[string]any) { b["initial_capital"] = nil }},
		{"negative fee", func(b map[string]any) { b["fee_percent"] = -1 }},
		{"excess fee", func(b map[string]any) { b["fee_percent"] = 100 }},
		{"excess slippage", func(b map[string]any) { b["slippage_percent"] = 100 }},
		{"unknown strategy", func(b map[string]any) { b["strategy"] = map[string]any{"type": "rsi"} }},
		{"missing strategy", func(b map[string]any) { delete(b, "strategy") }},
		{"bad SMA periods", func(b map[string]any) {
			b["strategy"] = map[string]any{"type": "sma_crossover", "fast_period": 3, "slow_period": 3}
		}},
		{"zero period", func(b map[string]any) { b["strategy"] = map[string]any{"type": "sma_crossover", "fast_period": 0} }},
		{"hold with SMA fields", func(b map[string]any) { b["strategy"] = map[string]any{"type": "buy_and_hold", "fast_period": 0} }},
		{"unknown nested field", func(b map[string]any) { b["strategy"] = map[string]any{"type": "sma_crossover", "secret": 1} }},
		{"unknown field", func(b map[string]any) { b["url"] = "http://localhost/private" }},
		{"too many candles", func(b map[string]any) { b["end"] = "2024-06-01T00:00:00Z" }},
		{"warmup exceeds 90 days", func(b map[string]any) { b["start"] = "2024-01-01T00:00:00Z"; b["end"] = "2024-03-31T00:00:00Z" }},
		{"one trading candle", func(b map[string]any) { b["end"] = "2024-01-01T04:00:00Z" }},
		{"SMA exceeds total budget", func(b map[string]any) {
			b["strategy"] = map[string]any{"type": "sma_crossover", "fast_period": 1, "slow_period": 2999}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := requestBody()
			tt.mutate(body)
			assertError(t, postJSON(t, handler, body), 400, "invalid_parameters")
		})
	}
	if calls.Load() != 0 {
		t.Errorf("invalid request reached provider %d times", calls.Load())
	}
}

func TestInvalidJSONAndBodyLimits(t *testing.T) {
	handler := fixtureHandler(t)
	for _, body := range []string{`null`, `[]`, `{`, `{"initial_capital":1e999}`, `{} {}`, `{"strategy":{"type":"buy_and_hold"},"fee_percent":NaN}`} {
		r := httptest.NewRequest("POST", "/api/v1/backtests", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		assertError(t, w, 400, "invalid_parameters")
	}
	body, _ := json.Marshal(requestBody())
	r := httptest.NewRequest("POST", "/api/v1/backtests", strings.NewReader(strings.Repeat(" ", MaxRequestBytes)+string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assertError(t, w, 413, "request_too_large")
	r = httptest.NewRequest("POST", "/api/v1/backtests", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assertError(t, w, 400, "invalid_parameters")
}

func TestMethodsAndQueries(t *testing.T) {
	handler := fixtureHandler(t)
	for _, path := range []string{"/healthz", "/api/v1/markets", "/api/v1/strategies", "/api/v1/candles", "/api/v1/backtests"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("DELETE", path, nil))
		assertError(t, w, 405, "method_not_allowed")
		if w.Header().Get("Allow") == "" {
			t.Error("405 has no Allow")
		}
	}
	for _, path := range []string{candleURL() + "&url=https://example.com", candleURL() + "&symbol=ETHUSDT", "/api/v1/candles?%xx=1", "/api/v1/candles"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		assertError(t, w, 400, "invalid_parameters")
	}
}

type providerFunc func(context.Context, string, string, time.Time, time.Time) ([]backtest.Candle, error)

func (f providerFunc) GetCandles(ctx context.Context, symbol, interval string, start, end time.Time) ([]backtest.Candle, error) {
	return f(ctx, symbol, interval, start, end)
}

func TestUpstreamErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   string
		retry  string
	}{
		{"empty history", nil, 502, "market_data_error", ""},
		{"private upstream details", errors.New("secret http://internal-user:password@host"), 502, "market_data_error", ""},
		{"timeout", context.DeadlineExceeded, 504, "request_timeout", ""},
		{"rate limit", &marketdata.HTTPError{StatusCode: 429, RetryAfter: 1500 * time.Millisecond}, 429, "upstream_rate_limited", "2"},
		{"ban", &marketdata.HTTPError{StatusCode: 418, RetryAfter: time.Minute}, 503, "market_data_unavailable", "60"},
		{"forbidden", &marketdata.HTTPError{StatusCode: 403}, 503, "market_data_unavailable", ""},
		{"restricted", &marketdata.HTTPError{StatusCode: 451}, 503, "market_data_unavailable", ""},
		{"server failure", &marketdata.HTTPError{StatusCode: 500}, 503, "market_data_unavailable", ""},
		{"upstream gateway timeout", &marketdata.HTTPError{StatusCode: 504}, 504, "upstream_timeout", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlerWithProvider(t, providerFunc(func(context.Context, string, string, time.Time, time.Time) ([]backtest.Candle, error) {
				return nil, tt.err
			}), simulation.Options{})
			w := postJSON(t, handler, requestBody())
			assertError(t, w, tt.status, tt.code)
			if w.Header().Get("Retry-After") != tt.retry || strings.Contains(w.Body.String(), "password") {
				t.Errorf("unsafe/wrong error response: %s %v", w.Body.String(), w.Header())
			}
		})
	}
}

func TestCORS(t *testing.T) {
	handler := fixtureHandler(t)
	for _, origin := range []string{"http://localhost:5173", "https://untrusted.example", "null", ""} {
		r := httptest.NewRequest("GET", "/api/v1/markets", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if origin == "http://localhost:5173" {
			if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Error("allowed origin rejected")
			}
		} else if origin != "" {
			assertError(t, w, 403, "origin_not_allowed")
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Error("untrusted origin allowed")
			}
		} else if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Code != 200 {
			t.Error("no-origin request was rejected or CORS wildcard enabled")
		}
	}
	r := httptest.NewRequest("OPTIONS", "/api/v1/backtests", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "content-type")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Methods") != "POST" {
		t.Errorf("preflight failed: %d %s", w.Code, w.Body.String())
	}
	r.Header.Set("Access-Control-Request-Headers", "Authorization")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assertError(t, w, 400, "invalid_parameters")
	w = httptest.NewRecorder()
	NewHandler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/markets", nil))
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS enabled by default")
	}
}

func TestCancellationTimeoutAndConcurrentLimit(t *testing.T) {
	t.Run("in-flight cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handler := handlerWithProvider(t, providerFunc(func(ctx context.Context, _ string, _ string, _, _ time.Time) ([]backtest.Candle, error) {
			cancel()
			<-ctx.Done()
			return nil, ctx.Err()
		}), simulation.Options{})
		r := httptest.NewRequest("GET", candleURL(), nil).WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		assertError(t, w, 408, "request_canceled")
	})
	t.Run("service deadline", func(t *testing.T) {
		handler := handlerWithProvider(t, providerFunc(func(ctx context.Context, _ string, _ string, _, _ time.Time) ([]backtest.Candle, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}), simulation.Options{Timeout: 20 * time.Millisecond})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", candleURL(), nil))
		assertError(t, w, 504, "request_timeout")
	})
	t.Run("shared capacity and release", func(t *testing.T) {
		entered := make(chan struct{}, 1)
		unblock := make(chan struct{})
		provider, err := marketdata.NewFixtureProvider("BTCUSDT", "1h", preparedCandles())
		if err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int32
		handler := handlerWithProvider(t, providerFunc(func(ctx context.Context, symbol, interval string, start, end time.Time) ([]backtest.Candle, error) {
			if calls.Add(1) == 1 {
				entered <- struct{}{}
				select {
				case <-unblock:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return provider.GetCandles(ctx, symbol, interval, start, end)
		}), simulation.Options{MaxConcurrent: 1})
		first := httptest.NewRecorder()
		done := make(chan struct{})
		go func() { defer close(done); handler.ServeHTTP(first, httptest.NewRequest("GET", candleURL(), nil)) }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("first request did not enter provider")
		}
		assertError(t, postJSON(t, handler, requestBody()), 429, "capacity_exceeded")
		metadata := httptest.NewRecorder()
		handler.ServeHTTP(metadata, httptest.NewRequest("GET", "/healthz", nil))
		if metadata.Code != 200 {
			t.Error("busy service blocks health")
		}
		close(unblock)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("request did not complete")
		}
		if first.Code != 200 {
			t.Errorf("first request failed: %s", first.Body.String())
		}
		decodeSuccess(t, postJSON(t, handler, requestBody()))
		if calls.Load() != 2 {
			t.Errorf("rejected request called provider: %d", calls.Load())
		}
	})
}
