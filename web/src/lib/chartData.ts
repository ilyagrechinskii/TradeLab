import type { CandlestickData, LineData, SeriesMarker, UTCTimestamp } from 'lightweight-charts';
import type { BacktestResult, CandlesResult, IndicatorPoint, Market, Trade } from '../types/api.ts';

export function unixSeconds(iso: string): UTCTimestamp {
  const milliseconds = new Date(iso).getTime();
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(iso) || !Number.isFinite(milliseconds) || new Date(milliseconds).toISOString().slice(0,19) !== iso.slice(0,19)) throw new Error('Chart data contains an invalid UTC timestamp.');
  return Math.floor(milliseconds / 1000) as UTCTimestamp;
}
function finite(...values: number[]) {
  if (!values.every(Number.isFinite)) throw new Error('Chart data contains a non-finite value.');
}
function sortedUnique<T extends { time: UTCTimestamp }>(values: T[]): T[] {
  const sorted = [...values].sort((a, b) => a.time - b.time);
  return sorted.filter((value, index) => {
    const previous = sorted[index - 1];
    if (!previous || previous.time !== value.time) return true;
    if (JSON.stringify(previous) !== JSON.stringify(value)) throw new Error('Conflicting chart values share the same timestamp.');
    return false;
  });
}
export function candleKey(market: Pick<Market, 'symbol' | 'interval' | 'start' | 'end'>): string {
  return JSON.stringify([market.symbol, market.interval, market.start, market.end]);
}
export function toCandles(data: CandlesResult['candles']): CandlestickData<UTCTimestamp>[] {
  return sortedUnique(data.map(c => {
    finite(c.open, c.high, c.low, c.close, c.volume);
    if (Math.min(c.open, c.high, c.low, c.close) <= 0 || c.low > Math.min(c.open, c.close) || c.high < Math.max(c.open, c.close) || c.low > c.high || c.volume < 0)
      throw new Error('Chart data contains invalid OHLC values.');
    return { time: unixSeconds(c.open_time), open: c.open, high: c.high, low: c.low, close: c.close };
  }));
}
export function validateCandleRange(response: CandlesResult, expected: Market): CandlesResult {
  if (candleKey(response.market) !== candleKey(expected) || response.market.source !== expected.source)
    throw new Error('Candle metadata does not match the completed backtest.');
  const candles = toCandles(response.candles);
  const match = /^([1-9]\d*)(h|d)$/.exec(expected.interval);
  if (!match) throw new Error('Unsupported chart timeframe.');
  const step = Number(match[1]) * (match[2] === 'h' ? 3600 : 86400);
  const start = unixSeconds(expected.start), end = unixSeconds(expected.end);
  if (!candles.length || candles.length !== (end - start) / step || candles.some((c, i) => c.time !== start + i * step))
    throw new Error('Candle history is empty, incomplete or outside the backtest period.');
  // Volume also matters when deduplicating raw candles.
  const originals = new Map<number, CandlesResult['candles'][number]>();
  for (const candle of response.candles) {
    const time = unixSeconds(candle.open_time), previous = originals.get(time);
    if (Date.parse(candle.open_time) !== time * 1000) throw new Error('Candle time is not on an exact interval boundary.');
    if (previous && previous.volume !== candle.volume) throw new Error('Conflicting candle volumes share the same timestamp.');
    originals.set(time, candle);
  }
  return response;
}
export function toIndicator(points: IndicatorPoint[], candles: CandlestickData<UTCTimestamp>[]): LineData<UTCTimestamp>[] {
  return sortedUnique(points.map(p => {
    finite(p.value);
    const time = unixSeconds(p.time);
    if (!Number.isSafeInteger(p.candle_index) || candles[p.candle_index]?.time !== time)
      throw new Error('SMA timestamps or indices do not match the trading candles.');
    return { time, value: p.value };
  }));
}
export function toMarkers(trades: Trade[], candles: CandlestickData<UTCTimestamp>[]): SeriesMarker<UTCTimestamp>[] {
  const times = new Set(candles.map(c => c.time));
  return trades.map((trade, index): SeriesMarker<UTCTimestamp> => {
    const time = unixSeconds(trade.time);
    if (!times.has(time) || Date.parse(trade.time) !== time * 1000) throw new Error('An execution timestamp does not match a trading candle.');
    // The API already returns execution at Open N+1. Never shift this timestamp.
    return { time, id: 'trade-' + index, position: trade.type === 'buy' ? 'belowBar' : 'aboveBar',
      color: trade.type === 'buy' ? '#22C55E' : '#EF4444', shape: trade.type === 'buy' ? 'arrowUp' : 'arrowDown', text: trade.type.toUpperCase() };
  }).sort((a,b) => a.time - b.time);
}
export function toEquity(points: BacktestResult['equity_curve']): LineData<UTCTimestamp>[] {
  return sortedUnique(points.map(p => { finite(p.equity); return { time: unixSeconds(p.time), value: p.equity }; }));
}
export function marketChartData(result: BacktestResult, response: CandlesResult) {
  validateCandleRange(response, result.market);
  const candles = toCandles(response.candles);
  const sma = result.strategy.type === 'sma_crossover';
  if (sma && (!result.indicators.fast_sma.length || !result.indicators.slow_sma.length))
    throw new Error('The backend did not provide the SMA series required by this strategy.');
  return { candles, fast: sma ? toIndicator(result.indicators.fast_sma, candles) : [],
    slow: sma ? toIndicator(result.indicators.slow_sma, candles) : [], markers: toMarkers(result.trades, candles) };
}

export function equityChartData(result: BacktestResult): LineData<UTCTimestamp>[] {
  const points = toEquity(result.equity_curve);
  if (!points.length) return points;
  const interval = /^([1-9]\d*)(h|d)$/.exec(result.market.interval);
  if (!interval) throw new Error('Unsupported equity timeframe.');
  const step = Number(interval[1]) * (interval[2] === 'h' ? 3600 : 86400);
  const start = unixSeconds(result.market.start), end = unixSeconds(result.market.end);
  if (points.length !== (end-start)/step || points.some((p,i)=>p.time !== start+i*step))
    throw new Error('Equity history does not cover the completed backtest period.');
  return points;
}
