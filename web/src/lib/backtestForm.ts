import { locale } from '../i18n/config.ts';
import type { Catalogs } from '../types/api.ts';
import type { BacktestFormValues, BacktestParameters, FormErrors } from '../types/backtest.ts';

const DAY = 86_400_000;
export const intervalHours = (interval: string) => Number(interval.slice(0, -1)) * (interval.endsWith('d') ? 24 : 1);
export const formatCount = (value: number) => new Intl.NumberFormat(locale()).format(value);
export const utcDate = (date: Date) => date.toISOString().slice(0, 10);

export function defaultFormValues(catalogs: Catalogs, now = new Date()): BacktestFormValues {
  const end = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  return {
    symbol: catalogs.markets.symbols[0], interval: catalogs.markets.intervals[0],
    start: utcDate(new Date(end - 30 * DAY)), end: utcDate(new Date(end)),
    strategy: catalogs.strategies.strategies[0].type,
    fastPeriod: String(catalogs.strategies.strategies.find(s => s.type === 'sma_crossover')?.parameters.fast_period.default ?? ''),
    slowPeriod: String(catalogs.strategies.strategies.find(s => s.type === 'sma_crossover')?.parameters.slow_period.default ?? ''),
    initialCapital: String(catalogs.strategies.configuration.initial_capital.default),
    feePercent: String(catalogs.strategies.configuration.fee_percent.default),
    slippagePercent: String(catalogs.strategies.configuration.slippage_percent.default),
  };
}

function parseDate(value: string): number {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return NaN;
  const time = Date.parse(value + 'T00:00:00Z');
  return Number.isFinite(time) && utcDate(new Date(time)) === value ? time : NaN;
}

function numeric(value: string): number {
  return value.trim() === '' ? NaN : Number(value);
}

export function validateForm(values: BacktestFormValues, catalogs: Catalogs, now = new Date()): {
  errors: FormErrors;
  parameters: BacktestParameters | null;
  candles: number | null;
  warmup: number;
} {
  const errors: FormErrors = {};
  const start = parseDate(values.start);
  const end = parseDate(values.end);
  const capital = numeric(values.initialCapital);
  const fee = numeric(values.feePercent);
  const slippage = numeric(values.slippagePercent);
  const fast = numeric(values.fastPeriod);
  const slow = numeric(values.slowPeriod);
  const hours = intervalHours(values.interval);
  const maxDays = catalogs.markets.limits.max_days[values.interval];
  const MAX_CANDLES = catalogs.markets.limits.max_candles;
  const sma = catalogs.strategies.strategies.find(s => s.type === 'sma_crossover');
  const MAX_SMA_PERIOD = sma?.parameters.slow_period.maximum ?? MAX_CANDLES - 2;
  if (!catalogs.markets.symbols.includes(values.symbol)) errors.symbol = 'Choose an available market.';
  if (!catalogs.markets.intervals.includes(values.interval)) errors.interval = 'Choose an available timeframe.';
  if (!catalogs.strategies.strategies.some(s => s.type === values.strategy)) errors.strategy = 'Choose an available strategy.';
  const step = hours * 3_600_000;
  const cutoff = Math.floor(now.getTime() / step) * step;

  if (!Number.isFinite(start) || start < 0) errors.start = 'Choose a valid date on or after 1 Jan 1970.';
  if (!Number.isFinite(end) || end <= 0) errors.end = 'Choose a valid end date.';
  if (Number.isFinite(start) && Number.isFinite(end) && start >= end) errors.end = 'End date must be after start date.';
  if (Number.isFinite(start) && start % step !== 0) errors.start = 'Start date must align with UTC candle boundaries.';
  if (Number.isFinite(end) && end % step !== 0) errors.end = 'End date must align with UTC candle boundaries.';
  if (end > cutoff) errors.end = 'Use a completed period. End date cannot be in the future.';
  if (!Number.isFinite(capital) || capital <= 0) errors.initialCapital = 'Initial capital must be greater than 0.';
  if (!Number.isFinite(fee) || fee < 0 || fee >= 100) errors.feePercent = 'Trading fee must be at least 0% and below 100%.';
  if (!Number.isFinite(slippage) || slippage < 0 || slippage >= 100) errors.slippagePercent = 'Slippage must be at least 0% and below 100%.';

  const config = catalogs.strategies.configuration;
  for (const [field, value, rule] of [
    ['initialCapital', capital, config.initial_capital],
    ['feePercent', fee, config.fee_percent],
    ['slippagePercent', slippage, config.slippage_percent],
  ] as const) {
    if ((rule.minimum !== undefined && value < rule.minimum) || (rule.maximum !== undefined && value > rule.maximum) ||
        (rule.minimum_exclusive !== undefined && value <= rule.minimum_exclusive) || (rule.maximum_exclusive !== undefined && value >= rule.maximum_exclusive))
      errors[field] = errors[field] ?? 'Value is outside the allowed server limits.';
  }
  if (values.strategy === 'sma_crossover') {
    if (!Number.isSafeInteger(fast) || fast < (sma?.parameters.fast_period.minimum ?? 1) || fast > (sma?.parameters.fast_period.maximum ?? MAX_SMA_PERIOD - 1))
      errors.fastPeriod = 'Enter a positive whole number up to ' + formatCount(sma?.parameters.fast_period.maximum ?? MAX_SMA_PERIOD - 1) + '.';
    if (!Number.isSafeInteger(slow) || slow < (sma?.parameters.slow_period.minimum ?? 2) || slow > MAX_SMA_PERIOD)
      errors.slowPeriod = 'Enter a positive whole number up to ' + formatCount(MAX_SMA_PERIOD) + '.';
    if (!errors.fastPeriod && !errors.slowPeriod && fast >= slow)
      errors.fastPeriod = 'Fast Period must be less than Slow Period.';
  }

  const warmup = values.strategy === 'sma_crossover' && Number.isSafeInteger(slow) && slow > 0 ? slow : 0;
  const candles = Number.isFinite(start) && Number.isFinite(end) && end > start ? (end - start) / step : null;
  if (candles !== null && !errors.start && !errors.end && !errors.slowPeriod) {
    if (start - warmup * step < 0) errors.start = 'SMA warm-up would start before 1 Jan 1970.';
    if (candles + warmup > MAX_CANDLES)
      errors.end = 'Period plus SMA warm-up must not exceed ' + formatCount(MAX_CANDLES) + ' candles.';
    else if (end - start + warmup * step > maxDays * DAY)
      errors.end = 'Period plus warm-up must fit within ' + maxDays + ' days for ' + values.interval + '.';
    else if (candles < 2) errors.end = 'Choose a period containing at least 2 trading candles.';
  }

  if (Object.keys(errors).length > 0) return { errors, parameters: null, candles, warmup };
  return {
    errors, candles, warmup,
    parameters: {
      symbol: values.symbol, interval: values.interval,
      start: values.start + 'T00:00:00Z', end: values.end + 'T00:00:00Z',
      initial_capital: capital, fee_percent: fee, slippage_percent: slippage,
      strategy: values.strategy === 'sma_crossover'
        ? { type: 'sma_crossover', fast_period: fast, slow_period: slow }
        : { type: 'buy_and_hold' },
    },
  };
}
