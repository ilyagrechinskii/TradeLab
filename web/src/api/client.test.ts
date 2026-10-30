import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';
import { ApiError, createApiClient, httpError, retryAfterSeconds } from './client.ts';
import { parseBacktest, parseMarkets, parseStrategies } from './schema.ts';
import type { BacktestParameters } from '../types/backtest.ts';

const example = JSON.parse(readFileSync(new URL('../../../docs/examples/btcusdt-sma-response.json', import.meta.url), 'utf8'));
const parameters: BacktestParameters = JSON.parse(readFileSync(new URL('../../../docs/examples/btcusdt-sma-request.json', import.meta.url), 'utf8'));
const catalogs = JSON.parse(readFileSync(new URL('../test/fixtures/catalogs.json', import.meta.url), 'utf8'));
const signal = () => new AbortController().signal;

test('validates actual saved Go response and nullable metrics without changing data', () => {
  assert.equal(parseBacktest(example), example);
  assert.equal(example.metrics.completed_trades, 4);
  assert.equal(example.benchmark.metrics.win_rate, null);
  assert.equal(parseMarkets(catalogs.markets), catalogs.markets);
  assert.equal(parseStrategies(catalogs.strategies), catalogs.strategies);
  for (const data of [{}, { ...example, metrics: { ...example.metrics, total_return: '3.29' } }, { ...example, trades: [{}] }])
    assert.throws(() => parseBacktest(data));
});

test('sends exact Go JSON contract once, with supplied abort signal and no credentials', async () => {
  let calls = 0;
  const fetcher: typeof fetch = async (url, init) => {
    calls++;
    assert.equal(url, 'https://api.example/api/v1/backtests');
    assert.equal(init?.method, 'POST');
    assert.equal(init?.credentials, 'omit');
    assert.deepEqual(JSON.parse(init?.body as string), parameters);
    assert.ok(init?.signal);
    return Response.json(example);
  };
  assert.equal((await createApiClient('https://api.example/', fetcher).backtest(parameters, signal())).portfolio.final_equity, example.portfolio.final_equity);
  assert.equal(calls, 1);
});

test('maps HTTP errors and respects Retry-After without retrying POST', async () => {
  for (const status of [400,403,408,429,500,502,503,504]) {
    let calls=0;
    const fetcher: typeof fetch = async () => { calls++; return new Response('', { status, headers: { 'Retry-After': '7' } }); };
    await assert.rejects(createApiClient('https://api.example', fetcher).backtest(parameters, signal()), (e: unknown) => e instanceof ApiError && e.status === status && e.retryAfter === 7);
    assert.equal(calls, 1);
    assert.ok(httpError(status, 7).message.length > 20);
  }
  assert.equal(retryAfterSeconds(null), null);
  assert.equal(retryAfterSeconds('invalid'), null);
  assert.equal(retryAfterSeconds('Thu, 01 Jan 2026 00:00:05 GMT', Date.parse('2026-01-01T00:00:00Z')), 5);
});

test('distinguishes configuration, network and malformed response errors', async () => {
  await assert.rejects(createApiClient(undefined).markets(signal()), (e: unknown) => e instanceof ApiError && e.kind === 'configuration');
  await assert.rejects(createApiClient('https://api.example', async () => { throw new TypeError('offline'); }).markets(signal()), (e: unknown) => e instanceof ApiError && e.kind === 'network');
  for (const response of [Response.json({}), new Response('<html>not JSON</html>')])
    await assert.rejects(createApiClient('https://api.example', async () => response).backtest(parameters, signal()), (e: unknown) => e instanceof ApiError && e.kind === 'response');
});

test('supports user cancellation and a bounded client waiting timeout', async () => {
  const fetcher: typeof fetch = (_url, init) => new Promise((_resolve, reject) => {
    const fail=() => reject(new DOMException('cancelled', 'AbortError'));
    if(init?.signal?.aborted) fail(); else init?.signal?.addEventListener('abort', fail, { once:true });
  });
  const controller = new AbortController();
  const pending = createApiClient('https://api.example', fetcher).markets(controller.signal);
  controller.abort();
  await assert.rejects(pending, (e: unknown) => e instanceof ApiError && e.kind === 'cancelled');
  await assert.rejects(createApiClient('https://api.example', fetcher, 5).markets(signal()), (e: unknown) => e instanceof ApiError && e.kind === 'timeout');
});
