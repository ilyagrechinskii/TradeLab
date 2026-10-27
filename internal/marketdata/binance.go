package marketdata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

const (
	BinanceBaseURL = "https://data-api.binance.vision"
	pageLimit      = 1000
	maxPages       = 10
	maxBodyBytes   = 1 << 20
)

type BinanceOptions struct {
	BaseURL    string
	HTTPClient *http.Client
	Now        func() time.Time // Defaults to time.Now; injectable for deterministic tests.
}

type Binance struct {
	baseURL string
	client  *http.Client
	now     func() time.Time
}

var _ Provider = (*Binance)(nil)

// NewBinance builds a reusable client. Empty options select the public market
// data host and a 10-second per-request timeout. No API key is used.
func NewBinance(options BinanceOptions) (*Binance, error) {
	base := options.BaseURL
	if base == "" {
		base = BinanceBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed == nil || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("Binance base URL must be an HTTP(S) origin without credentials, query or path")
	}
	client := http.Client{Timeout: 10 * time.Second}
	if options.HTTPClient != nil {
		client = *options.HTTPClient
		if client.Timeout <= 0 {
			client.Timeout = 10 * time.Second
		}
	}
	// Do not follow redirects: each page must stay on the configured origin and
	// consume one HTTP request from the bounded pagination budget.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Binance{baseURL: strings.TrimRight(base, "/"), client: &client, now: now}, nil
}

// HTTPError reports upstream status without exposing the upstream response body.
// RetryAfter is the advised delay for 429/418, when a valid header was present.
// No automatic retry is made; callers must honor this delay before trying again.
type HTTPError struct {
	StatusCode int
	RetryAfter time.Duration
}

func (err *HTTPError) Error() string {
	reason := "unexpected response"
	switch {
	case err.StatusCode == http.StatusTooManyRequests:
		reason = "rate limit exceeded"
	case err.StatusCode == http.StatusTeapot:
		reason = "IP temporarily banned"
	case err.StatusCode == http.StatusForbidden || err.StatusCode == http.StatusUnavailableForLegalReasons:
		reason = "access restricted"
	case err.StatusCode >= 500:
		reason = "upstream server failure"
	}
	message := fmt.Sprintf("Binance HTTP %d: %s", err.StatusCode, reason)
	if err.RetryAfter > 0 {
		message += fmt.Sprintf("; retry after %s", err.RetryAfter)
	}
	return message
}

func retryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((1<<63-1)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 0
}

func (b *Binance) GetCandles(ctx context.Context, symbol, interval string, start, end time.Time) ([]backtest.Candle, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := b.now().UTC()
	request, err := prepareRequest(symbol, interval, start, end, now)
	if err != nil {
		return nil, err
	}
	candles := make([]backtest.Candle, 0, request.count)
	cursor := request.start
	for page := 0; cursor.Before(request.end); page++ {
		if page >= maxPages {
			return nil, fmt.Errorf("Binance pagination exceeds %d HTTP requests", maxPages)
		}
		batch, err := b.loadPage(ctx, request, cursor, now)
		if err != nil {
			return nil, fmt.Errorf("Binance page %d: %w", page+1, err)
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("insufficient Binance data: empty page at %s", cursor.Format(time.RFC3339))
		}
		if len(candles)+len(batch) > request.count || len(candles)+len(batch) > MaxCandles {
			return nil, errors.New("Binance returned more candles than requested")
		}
		for _, candle := range batch {
			if !candle.OpenTime.Equal(cursor) {
				return nil, fmt.Errorf("Binance candle gap or duplicate: want %s, got %s", cursor.Format(time.RFC3339), candle.OpenTime.Format(time.RFC3339))
			}
			cursor = cursor.Add(request.step)
		}
		candles = append(candles, batch...)
	}
	if err := validateCoverage(candles, request); err != nil {
		return nil, fmt.Errorf("Binance candles: %w", err)
	}
	return candles, nil
}

func (b *Binance) loadPage(ctx context.Context, request candleRequest, cursor, now time.Time) ([]backtest.Candle, error) {
	query := url.Values{
		"symbol": {request.symbol}, "interval": {request.interval}, "timeZone": {"0"},
		"startTime": {strconv.FormatInt(cursor.UnixMilli(), 10)},
		// Binance's endTime is inclusive; our end is exclusive.
		"endTime": {strconv.FormatInt(request.end.UnixMilli()-1, 10)},
		"limit":   {strconv.Itoa(pageLimit)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+"/api/v3/klines?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create kline request: %w", err)
	}
	// Binance responses default to milliseconds. Intentionally do not opt in
	// to its X-MBX-TIME-UNIT: MICROSECOND header. Parameters use UnixMilli above.
	req.Header.Set("Accept", "application/json")
	response, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request klines: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		statusErr := &HTTPError{StatusCode: response.StatusCode}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusTeapot {
			statusErr.RetryAfter = retryAfter(response.Header.Get("Retry-After"), now)
		}
		return nil, statusErr
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read klines: %w", err)
	}
	if len(body) > maxBodyBytes {
		return nil, errors.New("Binance response exceeds 1 MiB")
	}
	return parsePage(body, request, now)
}
