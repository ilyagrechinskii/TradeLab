import { useEffect, useRef, useState } from 'react';
import type { BacktestResult, CandlesResult } from '../types/api';
import type { createApiClient } from '../api/client';
import { candleKey, validateCandleRange } from '../lib/chartData';

export type CandleState =
  | { status: 'idle'; key: string }
  | { status: 'loading'; key: string }
  | { status: 'error'; key: string; message: string }
  | { status: 'ready'; key: string; data: CandlesResult };

export function useCandles(result: BacktestResult | null, api: ReturnType<typeof createApiClient>) {
  const market = result?.market;
  const key = market ? candleKey(market) : '';
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<CandleState>({ status: 'idle', key: '' });
  const cached = useRef<{ key: string; data: CandlesResult } | null>(null);
  const active = useRef<AbortController | null>(null);
  useEffect(() => {
    if (!market) { setState({ status: 'idle', key }); return; }
    if (cached.current?.key === key && cached.current.data.market.source === market.source) {
      setState({ status: 'ready', key, data: cached.current.data }); return;
    }
    const controller = new AbortController();
    active.current = controller;
    setState({ status: 'loading', key });
    queueMicrotask(async () => {
      if (controller.signal.aborted) return;
      try {
        const data = validateCandleRange(await api.candles({ symbol: market.symbol, interval: market.interval, start: market.start, end: market.end }, controller.signal), market);
        if (controller.signal.aborted) return;
        cached.current = { key, data };
        setState({ status: 'ready', key, data });
      } catch (error) {
        if (!controller.signal.aborted) setState({ status: 'error', key, message: error instanceof Error ? error.message : 'Candle history could not be loaded.' });
      }
    });
    return () => { controller.abort(); if (active.current === controller) active.current = null; };
  }, [key, market?.source, attempt, api]); // market fields are represented by key.
  return {
    state: !key ? { status: 'idle' as const, key } : state.key === key ? state : { status: 'loading' as const, key },
    retry: () => setAttempt(n => n + 1),
    cancel: () => { active.current?.abort(); setState({ status: 'error', key, message: 'Candle loading cancelled. Backtest metrics are still available.' }); },
  };
}
