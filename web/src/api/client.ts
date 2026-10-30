import type { BacktestParameters } from '../types/backtest.ts';
import { parseBacktest, parseCandles, parseMarkets, parseStrategies } from './schema.ts';

export class ApiError extends Error {
  readonly status: number;
  readonly kind: string;
  readonly retryAfter: number | null;
  constructor(message: string, kind = 'http', status = 0, retryAfter: number | null = null) {
    super(message); this.name = 'ApiError'; this.kind = kind; this.status = status; this.retryAfter = retryAfter;
  }
}
export function retryAfterSeconds(value: string | null, now = Date.now()): number | null {
  if (!value) return null;
  if (/^\d+$/.test(value)) return Number(value);
  const time = Date.parse(value);
  return Number.isFinite(time) ? Math.max(0, Math.ceil((time - now) / 1000)) : null;
}
export function httpError(status: number, retry: number | null): ApiError {
  const messages: Record<number, string> = {
    400: 'The server rejected these parameters. Check the dates, strategy and available history.',
    403: 'Access was refused. Check the frontend origin in CORS_ALLOWED_ORIGINS.',
    408: 'The server cancelled the request. You can run it again manually.',
    429: 'Too many requests or the server is busy.' + (retry !== null ? ' Wait ' + retry + ' seconds before trying again.' : ' Please wait before trying again.'),
    502: 'Binance history is unavailable, incomplete or invalid. Try a different period or retry manually.',
    503: 'The market data service is temporarily unavailable. Please retry manually later.',
    504: 'The backend or market data request timed out. This is a server timeout; try a shorter period or retry manually.',
  };
  return new ApiError(messages[status] ?? 'The server could not complete the request (HTTP ' + status + '). Please retry manually.', 'http', status, retry);
}
export function createApiClient(base: string | undefined, fetcher: typeof fetch = fetch, timeoutOverride?: number) {
  let baseUrl = '';
  try {
    const url = new URL(base ?? '');
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || (url.pathname !== '/' && url.pathname !== '')) throw new Error();
    baseUrl = url.origin;
  } catch { /* Report configuration errors through the normal request state. */ }
  async function request<T>(path: string, parse: (value: unknown) => T, signal: AbortSignal, body?: BacktestParameters): Promise<T> {
    if (!baseUrl) throw new ApiError('Set VITE_API_BASE_URL in web/.env.local and restart Vite.', 'configuration');
    const timeout = new AbortController();
    const timer = setTimeout(() => timeout.abort(), timeoutOverride ?? (body ? 90_000 : 120_000));
    const combined = AbortSignal.any([signal, timeout.signal]);
    try {
      const response = await fetcher(baseUrl + path, {
        method: body ? 'POST' : 'GET', signal: combined, credentials: 'omit',
        headers: body ? { 'Content-Type': 'application/json', Accept: 'application/json' } : { Accept: 'application/json' },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
      if (!response.ok) throw httpError(response.status, retryAfterSeconds(response.headers.get('Retry-After')));
      try { return parse(await response.json()); }
      catch (error) {
        if (combined.aborted) throw error;
        throw new ApiError('The API returned an unexpected response. No results were displayed. Please retry manually.', 'response');
      }
    } catch (error) {
      if (signal.aborted) throw new ApiError('Request cancelled. No new result was saved.', 'cancelled');
      if (timeout.signal.aborted) throw new ApiError('The waiting limit was reached. Render may still be waking up. Please retry manually.', 'timeout');
      if (error instanceof ApiError) throw error;
      throw new ApiError('Cannot reach the API. Check your internet connection and the server CORS configuration, then retry manually.', 'network');
    } finally { clearTimeout(timer); }
  }
  return {
    markets: (signal: AbortSignal) => request('/api/v1/markets', parseMarkets, signal),
    strategies: (signal: AbortSignal) => request('/api/v1/strategies', parseStrategies, signal),
    backtest: (body: BacktestParameters, signal: AbortSignal) => request('/api/v1/backtests', parseBacktest, signal, body),
    candles: (query: { symbol: string; interval: string; start: string; end: string }, signal: AbortSignal) => request('/api/v1/candles?' + new URLSearchParams(query), parseCandles, signal),
  };
}
