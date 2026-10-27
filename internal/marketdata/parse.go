package marketdata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ilyagrechinskii/TradeLab/internal/backtest"
)

func parsePage(body []byte, request candleRequest, now time.Time) ([]backtest.Candle, error) {
	var rows [][]json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode kline JSON: %w", err)
	}
	if len(rows) > pageLimit {
		return nil, errors.New("Binance page exceeds 1000 candles")
	}
	candles := make([]backtest.Candle, 0, len(rows))
	for i, row := range rows {
		if len(row) < 7 {
			return nil, fmt.Errorf("kline %d: expected at least seven fields", i)
		}
		openTime, err := parseMillis(row[0])
		if err != nil {
			return nil, fmt.Errorf("kline %d open time: %w", i, err)
		}
		closeTime, err := parseMillis(row[6])
		if err != nil {
			return nil, fmt.Errorf("kline %d close time: %w", i, err)
		}
		if !openTime.Equal(openTime.Truncate(request.step)) ||
			!closeTime.Equal(openTime.Add(request.step).Add(-time.Millisecond)) {
			return nil, fmt.Errorf("kline %d: invalid millisecond timestamps or candle boundaries", i)
		}
		var values [5]float64
		for j := range values {
			var value string
			if err := json.Unmarshal(row[j+1], &value); err != nil {
				return nil, fmt.Errorf("kline %d field %d must be a decimal string", i, j+1)
			}
			values[j], err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("kline %d field %d is not a valid number", i, j+1)
			}
		}
		candle := backtest.Candle{
			OpenTime: openTime, Open: values[0], High: values[1], Low: values[2], Close: values[3], Volume: values[4],
		}
		if err := backtest.ValidateCandles([]backtest.Candle{candle}); err != nil {
			return nil, fmt.Errorf("kline %d: %w", i, err)
		}
		if !closeTime.Before(now) || openTime.Add(request.step).After(now) {
			continue // Never include a current/incomplete candle, even if returned unexpectedly.
		}
		if openTime.Before(request.start) || !openTime.Before(request.end) {
			return nil, fmt.Errorf("kline %d: closed candle outside requested range", i)
		}
		candles = append(candles, candle)
	}
	if len(candles) > 0 {
		if err := backtest.ValidateCandles(candles); err != nil {
			return nil, fmt.Errorf("invalid kline page: %w", err)
		}
	}
	return candles, nil
}

func parseMillis(raw json.RawMessage) (time.Time, error) {
	var millis int64
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return time.Time{}, errors.New("timestamp cannot be null")
	}
	if err := json.Unmarshal(raw, &millis); err != nil || millis < 0 {
		return time.Time{}, errors.New("timestamp must be a non-negative integer in Unix milliseconds")
	}
	value := time.UnixMilli(millis).UTC()
	if value.Year() > 9999 {
		return time.Time{}, errors.New("millisecond timestamp exceeds year 9999")
	}
	return value, nil
}
