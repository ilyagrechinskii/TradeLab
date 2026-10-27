package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
	"github.com/ilyagrechinskii/TradeLab/internal/simulation"
	"github.com/ilyagrechinskii/TradeLab/internal/strategy"
)

type optional[T any] struct {
	value T
	set   bool
}

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("null is not allowed")
	}
	o.set = true
	return json.Unmarshal(data, &o.value)
}
func (o optional[T]) or(value T) T {
	if o.set {
		return o.value
	}
	return value
}

type strategyRequest struct {
	Type       string        `json:"type"`
	FastPeriod optional[int] `json:"fast_period"`
	SlowPeriod optional[int] `json:"slow_period"`
}
type backtestRequest struct {
	Symbol          string            `json:"symbol"`
	Interval        string            `json:"interval"`
	Start           string            `json:"start"`
	End             string            `json:"end"`
	InitialCapital  optional[float64] `json:"initial_capital"`
	FeePercent      optional[float64] `json:"fee_percent"`
	SlippagePercent optional[float64] `json:"slippage_percent"`
	Strategy        *strategyRequest  `json:"strategy"`
}

func invalid(message string) error { return &simulation.ValidationError{Message: message} }

func parseMarket(symbol, interval, startValue, endValue string) (simulation.MarketRequest, error) {
	start, err := parseUTC(startValue)
	if err != nil {
		return simulation.MarketRequest{}, invalid("start must be an RFC3339 UTC timestamp")
	}
	end, err := parseUTC(endValue)
	if err != nil {
		return simulation.MarketRequest{}, invalid("end must be an RFC3339 UTC timestamp")
	}
	return simulation.MarketRequest{Symbol: symbol, Interval: interval, Start: start, End: end}, nil
}

func parseUTC(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	_, offset := t.Zone()
	if offset != 0 {
		return time.Time{}, errors.New("non-UTC offset")
	}
	return t.UTC(), nil
}

func parseMarketQuery(r *http.Request) (simulation.MarketRequest, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return simulation.MarketRequest{}, invalid("invalid query string")
	}
	for key, values := range query {
		if (key != "symbol" && key != "interval" && key != "start" && key != "end") || len(values) != 1 {
			return simulation.MarketRequest{}, invalid("unknown or repeated query parameter")
		}
	}
	return parseMarket(query.Get("symbol"), query.Get("interval"), query.Get("start"), query.Get("end"))
}

func decodeBacktest(w http.ResponseWriter, r *http.Request) (simulation.Request, error) {
	if r.ContentLength > MaxRequestBytes {
		return simulation.Request{}, &http.MaxBytesError{Limit: MaxRequestBytes}
	}
	if r.URL.RawQuery != "" {
		return simulation.Request{}, invalid("backtest does not accept query parameters")
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return simulation.Request{}, invalid("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request *backtestRequest
	if err := decoder.Decode(&request); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return simulation.Request{}, err
		}
		return simulation.Request{}, invalid("body must match the JSON request schema; unknown fields and null parameters are not allowed")
	}
	if request == nil || request.Strategy == nil {
		return simulation.Request{}, invalid("strategy and request object are required")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return simulation.Request{}, err
		}
		return simulation.Request{}, invalid("body must contain exactly one JSON object")
	}
	market, err := parseMarket(request.Symbol, request.Interval, request.Start, request.End)
	if err != nil {
		return simulation.Request{}, err
	}
	defaults := backtest.DefaultConfig()
	spec := simulation.StrategySpec{Type: request.Strategy.Type}
	if spec.Type == "sma_crossover" {
		sma := strategy.DefaultSMACrossover()
		spec.FastPeriod, spec.SlowPeriod = request.Strategy.FastPeriod.or(sma.FastPeriod), request.Strategy.SlowPeriod.or(sma.SlowPeriod)
	} else if request.Strategy.FastPeriod.set || request.Strategy.SlowPeriod.set {
		return simulation.Request{}, invalid("this strategy does not accept SMA parameters")
	}
	return simulation.Request{Market: market, Strategy: spec, Config: backtest.Config{
		InitialCapital: request.InitialCapital.or(defaults.InitialCapital), FeePercent: request.FeePercent.or(defaults.FeePercent), SlippagePercent: request.SlippagePercent.or(defaults.SlippagePercent),
	}}, nil
}
