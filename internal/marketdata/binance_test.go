package marketdata_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
)

var testStart = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
var testNow = testStart.Add(400 * 24 * time.Hour)

func rowAt(open time.Time, step time.Duration) []any {
	return []any{open.UnixMilli(), "100.25", "110.5", "90.125", "105.75", "12.5",
		open.Add(step).UnixMilli() - 1, "0", 10, "0", "0", "0"}
}

func rowsAt(start time.Time, count int, step time.Duration) [][]any {
	rows := make([][]any, count)
	for i := range rows {
		rows[i] = rowAt(start.Add(time.Duration(i)*step), step)
	}
	return rows
}

func writeRows(t *testing.T, w http.ResponseWriter, rows [][]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rows); err != nil {
		t.Errorf("encode rows: %v", err)
	}
}

func testBinance(t *testing.T, handler http.HandlerFunc, now time.Time) *marketdata.Binance {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := marketdata.NewBinance(marketdata.BinanceOptions{
		BaseURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestBinanceOHLCVAndUTC(t *testing.T) {
	var requests atomic.Int32
	client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/api/v3/klines" ||
			query.Get("symbol") != "BTCUSDT" || query.Get("interval") != "1h" ||
			query.Get("timeZone") != "0" || query.Get("limit") != "1000" ||
			query.Get("startTime") != strconv.FormatInt(testStart.UnixMilli(), 10) ||
			query.Get("endTime") != strconv.FormatInt(testStart.Add(2*time.Hour).UnixMilli()-1, 10) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("X-MBX-TIME-UNIT") != "" || r.Header.Get("X-MBX-APIKEY") != "" {
			t.Error("must use default milliseconds and no API key")
		}
		writeRows(t, w, rowsAt(testStart, 2, time.Hour))
	}, testNow)
	zone := time.FixedZone("UTC+3", 3*3600)
	candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart.In(zone), testStart.Add(2*time.Hour).In(zone))
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 || requests.Load() != 1 {
		t.Fatalf("candles=%d requests=%d", len(candles), requests.Load())
	}
	for i, candle := range candles {
		want := backtest.Candle{OpenTime: testStart.Add(time.Duration(i) * time.Hour), Open: 100.25, High: 110.5, Low: 90.125, Close: 105.75, Volume: 12.5}
		if !reflect.DeepEqual(candle, want) || candle.OpenTime.Location() != time.UTC {
			t.Errorf("candle = %+v, want %+v", candle, want)
		}
	}
}

func TestBinancePagination(t *testing.T) {
	for _, tt := range []struct {
		name     string
		count    int
		pageSize int
		pages    int32
	}{
		{"full pages", 2001, 1000, 3},
		{"short pages still advance", 5, 2, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			end := testStart.Add(time.Duration(tt.count) * time.Hour)
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
				page := int(requests.Add(1)) - 1
				wantStart := testStart.Add(time.Duration(page*tt.pageSize) * time.Hour)
				if r.URL.Query().Get("startTime") != strconv.FormatInt(wantStart.UnixMilli(), 10) {
					t.Errorf("page %d repeats/skips boundary: %s", page, r.URL)
				}
				if r.URL.Query().Get("endTime") != strconv.FormatInt(end.UnixMilli()-1, 10) {
					t.Error("pagination changed end boundary")
				}
				remaining := tt.count - page*tt.pageSize
				writeRows(t, w, rowsAt(wantStart, min(tt.pageSize, remaining), time.Hour))
			}, testNow)
			candles, err := client.GetCandles(context.Background(), "ETHUSDT", "1h", testStart, end)
			if err != nil {
				t.Fatal(err)
			}
			if len(candles) != tt.count || requests.Load() != tt.pages {
				t.Fatalf("candles=%d pages=%d", len(candles), requests.Load())
			}
			for i, candle := range candles {
				if !candle.OpenTime.Equal(testStart.Add(time.Duration(i) * time.Hour)) {
					t.Fatalf("gap or duplicate at index %d", i)
				}
			}
		})
	}
}

func TestBinanceOnlyClosedCandles(t *testing.T) {
	for _, now := range []time.Time{testStart.Add(150 * time.Minute), testStart.Add(2 * time.Hour)} {
		client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("endTime") != strconv.FormatInt(testStart.Add(2*time.Hour).UnixMilli()-1, 10) {
				t.Error("request includes unfinished candle")
			}
			// Defend against an unexpected current candle in an upstream response.
			writeRows(t, w, rowsAt(testStart, 3, time.Hour))
		}, now)
		candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(3*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if len(candles) != 2 {
			t.Errorf("got %d candles, want 2 closed candles", len(candles))
		}
	}
}

func TestBinanceOtherIntervals(t *testing.T) {
	for interval, step := range map[string]time.Duration{"4h": 4 * time.Hour, "1d": 24 * time.Hour} {
		t.Run(interval, func(t *testing.T) {
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
				writeRows(t, w, rowsAt(testStart, 2, step))
			}, testNow)
			candles, err := client.GetCandles(context.Background(), "SOLUSDT", interval, testStart, testStart.Add(2*step))
			if err != nil || len(candles) != 2 {
				t.Fatalf("candles=%d err=%v", len(candles), err)
			}
		})
	}
}

func TestBinanceRejectsBadResponses(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"invalid JSON", `[`}, {"wrong top level", `{}`}, {"trailing JSON", `[] []`},
		{"empty", `[]`}, {"null", `null`}, {"null row", `[null]`}, {"short row", `[[1,"2"]]`},
		{"oversized", strings.Repeat(" ", (1<<20)+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tt.body) }, testNow)
			candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour))
			if err == nil || candles != nil {
				t.Errorf("expected nil candles and error, got %v, %v", candles, err)
			}
		})
	}
}

func TestBinanceRejectsBadKlines(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func([]any)
	}{
		{"zero price", func(row []any) { row[1] = "0" }},
		{"negative price", func(row []any) { row[4] = "-1" }},
		{"NaN", func(row []any) { row[1] = "NaN" }},
		{"Infinity", func(row []any) { row[2] = "+Inf" }},
		{"low above high", func(row []any) { row[3] = "200" }},
		{"invalid high", func(row []any) { row[2] = "95" }},
		{"negative volume", func(row []any) { row[5] = "-1" }},
		{"NaN volume", func(row []any) { row[5] = "NaN" }},
		{"float overflow", func(row []any) { row[4] = "1e999" }},
		{"non-string price", func(row []any) { row[1] = 100 }},
		{"invalid decimal", func(row []any) { row[1] = "not a price" }},
		{"null price", func(row []any) { row[1] = nil }},
		{"seconds instead of millis", func(row []any) { row[0] = testStart.Unix(); row[6] = testStart.Add(time.Hour).Unix() - 1 }},
		{"microseconds instead of millis", func(row []any) { row[0] = testStart.UnixMicro(); row[6] = testStart.Add(time.Hour).UnixMicro() - 1 }},
		{"null open time", func(row []any) { row[0] = nil }},
		{"fractional open time", func(row []any) { row[0] = 1.5 }},
		{"negative open time", func(row []any) { row[0] = -1 }},
		{"invalid close time", func(row []any) { row[6] = "invalid" }},
		{"close time mismatch", func(row []any) { row[6] = testStart.Add(2*time.Hour).UnixMilli() - 1 }},
		{"misaligned open", func(row []any) { row[0] = testStart.Add(time.Minute).UnixMilli() }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := rowAt(testStart, time.Hour)
			tt.mutate(row)
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) { writeRows(t, w, [][]any{row}) }, testNow)
			if candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour)); err == nil || candles != nil {
				t.Errorf("accepted malformed kline: %v, %v", candles, err)
			}
		})
	}
}

func TestBinanceStatusErrors(t *testing.T) {
	for _, tt := range []struct {
		code   int
		header string
		delay  time.Duration
	}{
		{429, "120", 120 * time.Second}, {418, testNow.Add(30 * time.Second).Format(http.TimeFormat), 30 * time.Second},
		{429, "invalid", 0}, {418, "-1", 0}, {429, "9223372036854775807", 0},
		{403, "", 0}, {451, "", 0}, {500, "", 0}, {503, "", 0}, {400, "", 0},
	} {
		t.Run(fmt.Sprintf("%d/%s", tt.code, tt.header), func(t *testing.T) {
			var calls atomic.Int32
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", tt.header)
				w.WriteHeader(tt.code)
				_, _ = io.WriteString(w, "untrusted upstream body")
			}, testNow)
			candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour))
			var status *marketdata.HTTPError
			if candles != nil || !errors.As(err, &status) {
				t.Fatalf("expected HTTPError, got %v", err)
			}
			if status.StatusCode != tt.code || status.RetryAfter != tt.delay || calls.Load() != 1 {
				t.Errorf("unexpected status/delay/retries: %+v, calls=%d", status, calls.Load())
			}
			if strings.Contains(err.Error(), "untrusted upstream body") {
				t.Error("upstream body leaked into error")
			}
		})
	}
}

func TestBinanceTimeoutAndContext(t *testing.T) {
	t.Run("client timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		httpClient := server.Client()
		httpClient.Timeout = 30 * time.Millisecond
		client, err := marketdata.NewBinance(marketdata.BinanceOptions{BaseURL: server.URL, HTTPClient: httpClient, Now: func() time.Time { return testNow }})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected timeout, got %v", err)
		}
	})
	t.Run("cancel in flight", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := testBinance(t, func(w http.ResponseWriter, r *http.Request) { cancel(); <-r.Context().Done() }, testNow)
		_, err := client.GetCandles(ctx, "BTCUSDT", "1h", testStart, testStart.Add(time.Hour))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected cancellation, got %v", err)
		}
	})
	t.Run("canceled before call", func(t *testing.T) {
		client := testBinance(t, func(w http.ResponseWriter, r *http.Request) { t.Error("canceled request reached server") }, testNow)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := client.GetCandles(ctx, "BTCUSDT", "1h", testStart, testStart.Add(time.Hour)); !errors.Is(err, context.Canceled) {
			t.Errorf("expected cancellation, got %v", err)
		}
	})
}

func TestBinanceGapsDuplicatesAndPageLimit(t *testing.T) {
	for _, tt := range []struct {
		name string
		rows [][]any
	}{
		{"gap", [][]any{rowAt(testStart, time.Hour), rowAt(testStart.Add(2*time.Hour), time.Hour)}},
		{"missing first", [][]any{rowAt(testStart.Add(time.Hour), time.Hour)}},
		{"duplicate", [][]any{rowAt(testStart, time.Hour), rowAt(testStart, time.Hour)}},
		{"unordered", [][]any{rowAt(testStart.Add(time.Hour), time.Hour), rowAt(testStart, time.Hour)}},
		{"outside range", [][]any{rowAt(testStart.Add(3*time.Hour), time.Hour)}},
		{"oversized page", rowsAt(testStart, 1001, time.Hour)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := testBinance(t, func(w http.ResponseWriter, r *http.Request) { writeRows(t, w, tt.rows) }, testNow)
			if candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(3*time.Hour)); err == nil || candles != nil {
				t.Errorf("expected error without partial candles: %v", err)
			}
		})
	}
	t.Run("duplicate between pages", func(t *testing.T) {
		var calls atomic.Int32
		client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			writeRows(t, w, rowsAt(testStart, 1, time.Hour))
		}, testNow)
		if candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(2*time.Hour)); err == nil || candles != nil || calls.Load() != 2 {
			t.Errorf("duplicate page not stopped: candles=%v calls=%d err=%v", candles, calls.Load(), err)
		}
	})
	t.Run("bounded request count", func(t *testing.T) {
		var calls atomic.Int32
		client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
			index := int(calls.Add(1)) - 1
			writeRows(t, w, rowsAt(testStart.Add(time.Duration(index)*time.Hour), 1, time.Hour))
		}, testNow)
		if candles, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(11*time.Hour)); err == nil || candles != nil || calls.Load() != 10 {
			t.Errorf("unbounded pagination: candles=%v calls=%d err=%v", candles, calls.Load(), err)
		}
	})
}

func TestBinanceRequestValidation(t *testing.T) {
	client := testBinance(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid request reached server") }, testNow)
	for _, tt := range []struct {
		name     string
		symbol   string
		interval string
		start    time.Time
		end      time.Time
	}{
		{"unsupported symbol", "XRPUSDT", "1h", testStart, testStart.Add(time.Hour)},
		{"unsupported interval", "BTCUSDT", "1m", testStart, testStart.Add(time.Hour)},
		{"start off grid", "BTCUSDT", "1h", testStart.Add(time.Second), testStart.Add(time.Hour)},
		{"end off grid", "BTCUSDT", "1h", testStart, testStart.Add(time.Hour + time.Nanosecond)},
		{"4h off grid", "BTCUSDT", "4h", testStart.Add(time.Hour), testStart.Add(4 * time.Hour)},
		{"daily off grid", "BTCUSDT", "1d", testStart, testStart.Add(25 * time.Hour)},
		{"empty range", "BTCUSDT", "1h", testStart, testStart},
		{"reversed", "BTCUSDT", "1h", testStart.Add(time.Hour), testStart},
		{"zero date", "BTCUSDT", "1h", time.Time{}, testStart},
		{"too many candles", "BTCUSDT", "1h", testStart, testStart.Add(3001 * time.Hour)},
		{"over 90 days", "BTCUSDT", "1h", testStart, testStart.Add(90*24*time.Hour + time.Hour)},
		{"over 365 days 4h", "BTCUSDT", "4h", testStart, testStart.Add(365*24*time.Hour + 4*time.Hour)},
		{"over 365 days 1d", "BTCUSDT", "1d", testStart, testStart.Add(366 * 24 * time.Hour)},
		{"only future candles", "BTCUSDT", "1h", testNow, testNow.Add(time.Hour)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if candles, err := client.GetCandles(context.Background(), tt.symbol, tt.interval, tt.start, tt.end); err == nil || candles != nil {
				t.Errorf("accepted invalid request: %v", err)
			}
		})
	}
	if _, err := client.GetCandles(nil, "BTCUSDT", "1h", testStart, testStart.Add(time.Hour)); err == nil {
		t.Error("accepted nil context")
	}
}

func TestNewBinance(t *testing.T) {
	for _, base := range []string{"://bad", "file:///tmp/klines", "https://", "https://example.com/path", "https://user:secret@example.com", "https://example.com?query=1"} {
		if _, err := marketdata.NewBinance(marketdata.BinanceOptions{BaseURL: base}); err == nil {
			t.Errorf("accepted base URL %q", base)
		}
	}
	if _, err := marketdata.NewBinance(marketdata.BinanceOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestBinanceDoesNotFollowRedirects(t *testing.T) {
	var calls atomic.Int32
	client := testBinance(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}, testNow)
	_, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour))
	var status *marketdata.HTTPError
	if !errors.As(err, &status) || status.StatusCode != http.StatusFound || calls.Load() != 1 {
		t.Errorf("unexpected redirect handling: calls=%d err=%v", calls.Load(), err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (body *trackedBody) Close() error { body.closed = true; return nil }

func TestBodyClosedAndNetworkErrorsWrapped(t *testing.T) {
	for _, status := range []int{200, 429} {
		body := &trackedBody{Reader: strings.NewReader(`invalid JSON`)}
		httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: body}, nil
		})}
		client, err := marketdata.NewBinance(marketdata.BinanceOptions{HTTPClient: httpClient, Now: func() time.Time { return testNow }})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour))
		if !body.closed {
			t.Errorf("HTTP %d body was not closed", status)
		}
		if httpClient.Timeout != 0 {
			t.Error("constructor mutated caller's HTTP client")
		}
	}
	sentinel := errors.New("network unavailable")
	client, err := marketdata.NewBinance(marketdata.BinanceOptions{
		Now:        func() time.Time { return testNow },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, sentinel })},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetCandles(context.Background(), "BTCUSDT", "1h", testStart, testStart.Add(time.Hour)); !errors.Is(err, sentinel) {
		t.Errorf("network error lost its cause: %v", err)
	}
}
