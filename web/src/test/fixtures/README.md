# Test data

- catalogs.json: deterministic mock of the existing Go catalog contract.
- btcusdt-1h-2024-01-01-08.json: 168 real BTCUSDT hourly OHLCV rows downloaded on 2026-10-30 from Binance Public Market Data (data-api.binance.vision/api/v3/klines), startTime=1704067200000, endTime=1704671999999, limit=1000. Rows are mapped to the existing Go candle response DTO; this file is not a captured Render response. No candles or prices were generated.
- Backtest metrics, indicators, orders and equity in tests use the real saved Go response in docs/examples/btcusdt-sma-response.json.

These fixtures are imported by tests only and never used as production fallbacks.
