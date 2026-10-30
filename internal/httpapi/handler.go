package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/marketdata"
	"github.com/ilyagrechinskii/TradeLab/internal/simulation"
)

const (
	MaxRequestBytes  = 16 << 10
	MaxResponseBytes = 2 << 20
)

type Options struct {
	Service        *simulation.Service
	AllowedOrigins []string
	Logger         *slog.Logger
}

type api struct {
	service *simulation.Service
	origins map[string]bool
	logger  *slog.Logger
}

// NewHandler keeps the no-argument health-check setup compatible. Production
// supplies a Service explicitly; a missing service makes heavy routes return 503.
func NewHandler(options ...Options) http.Handler {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	a := &api{service: opts.Service, logger: opts.Logger, origins: make(map[string]bool)}
	for _, origin := range opts.AllowedOrigins {
		a.origins[origin] = true
	}
	return http.HandlerFunc(a.serveHTTP)
}

func routeMethod(path string) string {
	switch path {
	case "/healthz", "/api/v1/markets", "/api/v1/strategies", "/api/v1/candles":
		return http.MethodGet
	case "/api/v1/backtests":
		return http.MethodPost
	default:
		return ""
	}
}

func (a *api) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !a.origins[origin] {
			a.failure(w, 403, "origin_not_allowed", "origin is not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "Retry-After")
	}
	method := routeMethod(r.URL.Path)
	if method == "" {
		a.failure(w, 404, "not_found", "endpoint not found")
		return
	}
	if r.Method == http.MethodOptions && origin != "" && r.Header.Get("Access-Control-Request-Method") != "" {
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")
		if r.Header.Get("Access-Control-Request-Method") != method {
			w.Header().Set("Allow", method)
			a.failure(w, 405, "method_not_allowed", "method is not supported")
			return
		}
		for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
			header = strings.TrimSpace(header)
			if header != "" && !strings.EqualFold(header, "Content-Type") {
				a.failure(w, 400, "invalid_parameters", "preflight header is not allowed")
				return
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", method)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		a.failure(w, 405, "method_not_allowed", "method is not supported")
		return
	}
	switch r.URL.Path {
	case "/healthz":
		a.writeJSON(w, 200, map[string]string{"status": "ok"})
	case "/api/v1/markets":
		a.writeJSON(w, 200, markets())
	case "/api/v1/strategies":
		a.writeJSON(w, 200, strategies())
	case "/api/v1/candles":
		a.candles(w, r)
	case "/api/v1/backtests":
		a.backtest(w, r)
	}
}

func (a *api) candles(w http.ResponseWriter, r *http.Request) {
	request, err := parseMarketQuery(r)
	if err != nil {
		a.handleError(w, err)
		return
	}
	if a.service == nil {
		a.failure(w, 503, "service_unavailable", "market data service is unavailable")
		return
	}
	result, err := a.service.Candles(r.Context(), request)
	if err != nil {
		a.handleError(w, err)
		return
	}
	a.writeJSON(w, 200, candleResponse{Market: marketDTO(result.Market, result.Source), Candles: candleDTOs(result.Candles)})
}

func (a *api) backtest(w http.ResponseWriter, r *http.Request) {
	request, err := decodeBacktest(w, r)
	if err != nil {
		a.handleError(w, err)
		return
	}
	if a.service == nil {
		a.failure(w, 503, "service_unavailable", "market data service is unavailable")
		return
	}
	result, err := a.service.Run(r.Context(), request)
	if err != nil {
		a.handleError(w, err)
		return
	}
	a.writeJSON(w, 200, backtestDTO(result))
}

type errorEnvelope struct {
	Error errorMessage `json:"error"`
}
type errorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a *api) failure(w http.ResponseWriter, status int, code, message string) {
	a.writeJSON(w, status, errorEnvelope{Error: errorMessage{Code: code, Message: message}})
}

func (a *api) writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil || len(body) > MaxResponseBytes {
		a.logger.Error("API response could not be encoded within limits")
		status = 500
		body = []byte(`{"error":{"code":"internal_error","message":"response could not be produced"}}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (a *api) handleError(w http.ResponseWriter, err error) {
	var validation *simulation.ValidationError
	var tooLarge *http.MaxBytesError
	var upstream *marketdata.HTTPError
	var data *simulation.DataError
	var network net.Error
	switch {
	case errors.As(err, &tooLarge):
		a.failure(w, 413, "request_too_large", "JSON body exceeds 16 KiB")
	case errors.As(err, &validation):
		a.failure(w, 400, "invalid_parameters", validation.Message)
	case errors.Is(err, simulation.ErrBusy):
		w.Header().Set("Retry-After", "1")
		a.failure(w, 429, "capacity_exceeded", "too many concurrent heavy requests")
	case errors.Is(err, context.Canceled):
		a.failure(w, 408, "request_canceled", "request was canceled")
	case errors.Is(err, context.DeadlineExceeded):
		a.logger.Warn("request time limit reached")
		a.failure(w, 504, "request_timeout", "request exceeded its time limit")
	case errors.As(err, &upstream):
		a.logger.Warn("market data HTTP request failed", "upstream_status", upstream.StatusCode)
		if upstream.RetryAfter > 0 {
			seconds := upstream.RetryAfter / time.Second
			if upstream.RetryAfter%time.Second != 0 {
				seconds++
			}
			w.Header().Set("Retry-After", strconv.FormatInt(int64(seconds), 10))
		}
		switch upstream.StatusCode {
		case 429:
			a.failure(w, 429, "upstream_rate_limited", "market data request limit reached")
		case 504:
			a.failure(w, 504, "upstream_timeout", "market data request timed out")
		case 418, 403, 451, 500, 502, 503:
			a.failure(w, 503, "market_data_unavailable", "market data source is temporarily unavailable")
		default:
			if upstream.StatusCode >= 500 {
				a.failure(w, 503, "market_data_unavailable", "market data source is temporarily unavailable")
			} else {
				a.failure(w, 502, "market_data_error", "market data source returned an unexpected response")
			}
		}
	case errors.As(err, &network):
		a.logger.Warn("market data network request failed", "timeout", network.Timeout())
		if network.Timeout() {
			a.failure(w, 504, "upstream_timeout", "market data request timed out")
		} else {
			a.failure(w, 503, "market_data_unavailable", "market data source is unavailable")
		}
	case errors.As(err, &data):
		a.logger.Warn("market data history failed validation")
		a.failure(w, 502, "market_data_error", "market data source returned incomplete or invalid history")
	default:
		a.logger.Error("API request failed unexpectedly")
		a.failure(w, 500, "internal_error", "request could not be completed")
	}
}
