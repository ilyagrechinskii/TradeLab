// Small runtime validators keep malformed responses out of financial UI state.
import type { BacktestResult, CandlesResult, Markets, Strategies } from '../types/api.ts';
type Check = (value: unknown) => boolean;
const object = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const number: Check = v => typeof v === 'number' && Number.isFinite(v);
const integer: Check = v => number(v) && Number.isSafeInteger(v) && (v as number) >= 0;
const positive: Check = v => number(v) && (v as number) > 0;
const text: Check = v => typeof v === 'string' && v.length > 0;
const time: Check = v => typeof v === 'string' && v.endsWith('Z') && Number.isFinite(Date.parse(v));
const nullable = (check: Check): Check => v => v === null || check(v);
const array = (check: Check): Check => v => Array.isArray(v) && v.every(check);
const shape = (fields: Record<string, Check>): Check => v => object(v) && Object.entries(fields).every(([key, check]) => check(v[key]));
const record = (check: Check): Check => v => object(v) && Object.values(v).every(check);
const parameter: Check = v => object(v) && number(v.default) && ['minimum','maximum','minimum_exclusive','maximum_exclusive'].every(k => v[k] === undefined || number(v[k]));
const strategy: Check = v => object(v) && (v.type === 'buy_and_hold' || (v.type === 'sma_crossover' && positive(v.fast_period) && positive(v.slow_period)));
const metrics = shape({ total_return: number, max_drawdown: number, completed_trades: integer, win_rate: nullable(number), profit_factor: nullable(number) });
const portfolio = shape({ final_equity: number, cash: number, asset_quantity: number, total_fees: number });
const market = shape({ symbol: text, interval: text, start: time, end: time, source: text });
const indicator = shape({ candle_index: integer, time, value: number });
const trade = shape({ time, type: v => v === 'buy' || v === 'sell', price: number, quantity: number, notional: number, fee: number, cash_after: number, asset_quantity_after: number });

export function parseMarkets(v: unknown): Markets {
  if (!shape({ symbols: array(text), intervals: array(text), limits: shape({ max_candles: positive, max_days: record(positive), range: v => v === '[start, end)', time_zone: v => v === 'UTC', warmup_included_in_limits: v => v === true }) })(v)) throw new Error('schema');
  const result = v as Markets;
  if (!result.symbols.length || !result.intervals.length || !result.intervals.every(i => /^[1-9]\d*[hd]$/.test(i) && result.limits.max_days[i] > 0)) throw new Error('schema');
  return result;
}
export function parseStrategies(v: unknown): Strategies {
  if (!shape({ strategies: array(shape({ type: v => v === 'sma_crossover' || v === 'buy_and_hold', parameters: record(parameter), constraints: array(text) })), configuration: shape({ initial_capital: parameter, fee_percent: parameter, slippage_percent: parameter }) })(v)) throw new Error('schema');
  const result = v as Strategies;
  if (!result.strategies.length) throw new Error('schema');
  for (const s of result.strategies) if (s.type === 'sma_crossover' && (!s.parameters.fast_period || !s.parameters.slow_period)) throw new Error('schema');
  return result;
}
export function parseBacktest(v: unknown): BacktestResult {
  if (!shape({ market, strategy, configuration: shape({ initial_capital: positive, fee_percent: number, slippage_percent: number }), metrics, portfolio, benchmark: shape({ strategy: text, metrics, portfolio, excess_return: number }), trades: array(trade), equity_curve: array(shape({ time, equity: number })), indicators: shape({ fast_sma: array(indicator), slow_sma: array(indicator) }) })(v)) throw new Error('schema');
  return v as BacktestResult;
}
export function parseCandles(v: unknown): CandlesResult {
  if (!shape({ market, candles: array(shape({ open_time: time, open: number, high: number, low: number, close: number, volume: number })) })(v)) throw new Error('schema');
  return v as CandlesResult;
}
