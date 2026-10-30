import { readFileSync } from 'node:fs';
import type { Catalogs } from '../types/api.ts';
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultFormValues as initial, validateForm as validate } from './backtestForm.ts';
import type { BacktestFormValues } from '../types/backtest.ts';

const catalogs: Catalogs = JSON.parse(readFileSync(new URL('../test/fixtures/catalogs.json', import.meta.url), 'utf8'));
const defaultFormValues = (now: Date) => initial(catalogs, now);
const validateForm = (values: BacktestFormValues, now: Date) => validate(values, catalogs, now);
const now = new Date('2026-10-30T12:00:00Z');
const values = (changes: Partial<BacktestFormValues> = {}) => ({
  ...defaultFormValues(now), start: '2024-01-01', end: '2024-02-01', ...changes,
});

test('defaults produce a valid UTC request with explicit percentages and SMA warm-up', () => {
  const result = validateForm(defaultFormValues(now), now);
  assert.deepEqual(result.errors, {});
  assert.equal(result.candles, 720);
  assert.equal(result.warmup, 30);
  assert.equal(result.parameters?.start, '2026-09-30T00:00:00Z');
  assert.equal(result.parameters?.fee_percent, 0.1);
  assert.equal(result.parameters?.initial_capital, 10000);
  assert.deepEqual(result.parameters?.strategy, { type: 'sma_crossover', fast_period: 10, slow_period: 30 });
});

test('Buy & Hold ignores hidden SMA fields and omits them from the request', () => {
  const result = validateForm(values({ strategy: 'buy_and_hold', fastPeriod: '', slowPeriod: '-1' }), now);
  assert.deepEqual(result.errors, {});
  assert.equal(result.warmup, 0);
  assert.deepEqual(result.parameters?.strategy, { type: 'buy_and_hold' });
});

test('rejects missing, impossible, reversed and future dates', () => {
  for (const changes of [
    { start: '' }, { start: '2024-02-30' }, { end: '2024-01-01' },
    { start: '2024-03-01' }, { end: '2027-01-01' }, { start: '1969-12-31' },
  ]) {
    assert.equal(validateForm(values(changes), now).parameters, null, JSON.stringify(changes));
  }
});

test('capital must be positive and finite, with no coercion of blanks to zero', () => {
  for (const initialCapital of ['', '0', '-1', 'Infinity', 'NaN', '1e309'])
    assert.ok(validateForm(values({ initialCapital }), now).errors.initialCapital);
  assert.ok(validateForm(values({ initialCapital: '0.01' }), now).parameters);
});

test('fee and slippage accept zero and decimals, reject negative values and 100%', () => {
  for (const key of ['feePercent', 'slippagePercent'] as const) {
    for (const value of ['', '-0.1', '100', 'Infinity'])
      assert.ok(validateForm(values({ [key]: value }), now).errors[key]);
    for (const value of ['0', '0.1', '99.999'])
      assert.ok(validateForm(values({ [key]: value }), now).parameters);
  }
});

test('SMA requires positive whole periods with fast below slow', () => {
  for (const changes of [
    { fastPeriod: '0' }, { slowPeriod: '0' }, { fastPeriod: '1.5' },
    { fastPeriod: '30' }, { fastPeriod: '31' }, { slowPeriod: '2999' },
  ])
    assert.equal(validateForm(values(changes), now).parameters, null, JSON.stringify(changes));
});

test('90-day hourly limit includes SMA warm-up', () => {
  const range = { start: '2024-01-01', end: '2024-03-31' };
  assert.ok(validateForm(values({ ...range, strategy: 'buy_and_hold' }), now).parameters);
  assert.match(validateForm(values(range), now).errors.end!, /90 days/);
  assert.ok(validateForm(values({ ...range, end: '2024-03-29' }), now).parameters);
});

test('365-day limit includes warm-up for both 4h and 1d', () => {
  for (const interval of ['4h', '1d'] as const) {
    const range = { interval, start: '2024-01-01', end: '2024-12-31' };
    assert.ok(validateForm(values({ ...range, strategy: 'buy_and_hold' }), now).parameters);
    assert.match(validateForm(values(range), now).errors.end!, /365 days/);
  }
});

test('combined history cannot exceed 3000 candles', () => {
  // 48 trading candles + 2952 warm-up is exactly 3000, but exceeds 90 days.
  const result = validateForm(values({ end: '2024-01-03', slowPeriod: '2953' }), now);
  assert.equal(result.parameters, null);
  assert.match(result.errors.end!, /3,000 candles/);
});

test('rejects warm-up before epoch and fewer than two trading candles', () => {
  assert.ok(validateForm(values({ start: '1970-01-01', end: '1970-01-04' }), now).errors.start);
  assert.match(validateForm(values({ interval: '1d', end: '2024-01-02' }), now).errors.end!, /at least 2/);
});

test('serializes the historical sample exactly and adapts to server limits', () => {
  const sample = JSON.parse(readFileSync(new URL('../../../docs/examples/btcusdt-sma-request.json', import.meta.url), 'utf8'));
  assert.deepEqual(validateForm(values({ end: '2024-01-08' }), now).parameters, sample);
  const changed = structuredClone(catalogs);
  changed.markets.symbols = ['NEWUSDT'];
  const initialValues = initial(changed, now);
  assert.equal(initialValues.symbol, 'NEWUSDT');
  changed.markets.limits.max_days['1h'] = 2;
  assert.equal(validate(initialValues, changed, now).parameters, null);
  changed.strategies.configuration.fee_percent.maximum_exclusive = 0.05;
  assert.ok(validate(initialValues, changed, now).errors.feePercent);
});
