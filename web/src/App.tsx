import { useEffect, useRef, useState } from 'react';
import Header from './components/Header';
import BacktestForm from './components/BacktestForm';
import ResultsPanel from './components/ResultsPanel';
import RequestStatus from './components/RequestStatus';
import { defaultFormValues, validateForm } from './lib/backtestForm';
import { ApiError, createApiClient } from './api/client';
import type { BacktestFormValues, BacktestParameters } from './types/backtest';
import type { BacktestResult, Catalogs } from './types/api';

const api = createApiClient(import.meta.env.VITE_API_BASE_URL);
const message = (error: unknown) => error instanceof Error ? error.message : 'The request failed. Please retry manually.';

export default function App() {
  const [attempt, setAttempt] = useState(0);
  const [catalogs, setCatalogs] = useState<Catalogs | null>(null);
  const [values, setValues] = useState<BacktestFormValues | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(true);
  const [catalogError, setCatalogError] = useState('');
  const catalogRequest = useRef<AbortController | null>(null);
  const request = useRef<AbortController | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [cancelled, setCancelled] = useState(false);
  const [result, setResult] = useState<BacktestResult | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    catalogRequest.current = controller;
    setCatalogLoading(true); setCatalogError('');
    // Deferral lets StrictMode clean up its first effect before any network I/O.
    queueMicrotask(async () => {
      if (controller.signal.aborted) return;
      try {
        const [markets, strategies] = await Promise.all([api.markets(controller.signal), api.strategies(controller.signal)]);
        if (controller.signal.aborted) return;
        const next = { markets, strategies };
        setCatalogs(next); setValues(defaultFormValues(next));
        setCatalogLoading(false);
      } catch (error) {
        if (catalogRequest.current !== controller) return;
        setCatalogError(message(error)); setCatalogLoading(false); controller.abort();
      }
    });
    return () => { controller.abort(); catalogRequest.current = null; };
  }, [attempt]);

  useEffect(() => () => { request.current?.abort(); request.current = null; }, []);

  async function run(parameters: BacktestParameters) {
    // Synchronous guard also protects against a second click before React renders.
    if (request.current || !values || !catalogs || !validateForm(values, catalogs).parameters) return;
    const controller = new AbortController();
    request.current = controller;
    setRunning(true); setError(''); setCancelled(false); setResult(null);
    try {
      const next = await api.backtest(parameters, controller.signal);
      if (request.current === controller) setResult(next);
    } catch (error) {
      if (request.current === controller) {
        setError(message(error)); setCancelled(error instanceof ApiError && error.kind === 'cancelled');
      }
    } finally {
      if (request.current === controller) { request.current = null; setRunning(false); }
    }
  }

  return <>
    <a className="skip-link" href="#main">Skip to workspace</a>
    <Header />
    <main id="main" className="workspace">
      <div className="workspace-intro">
        <div><p className="eyebrow">RESEARCH WORKSPACE</p><h1>Test ideas. Understand performance.</h1><p className="workspace-subtitle">Explore crypto strategies with historical market data.</p></div>
        <span className="stage-badge">Historical backtesting</span>
      </div>
      {catalogLoading && <RequestStatus label="Loading markets & strategies..." onCancel={() => catalogRequest.current?.abort()} />}
      {catalogError && <div className="api-feedback" role="alert"><p>{catalogError}</p><button className="secondary-button" onClick={() => setAttempt(n => n + 1)}>Retry catalogs</button></div>}
      {running && <RequestStatus label="Running Backtest..." onCancel={() => request.current?.abort()} />}
      {error && <div className={'api-feedback' + (cancelled ? ' feedback-neutral' : '')} role="alert"><p>{error}</p><p>No result from this run is displayed. Adjust the form or run again manually.</p></div>}
      {catalogs && values && <div className="dashboard">
        <BacktestForm values={values} onChange={setValues} catalogs={catalogs} running={running} onRun={run} />
        <ResultsPanel symbol={values.symbol} interval={values.interval} result={result} />
      </div>}
      <footer className="footer"><span>TradeLab <span className="footer-divider">/</span> Strategy research, made clear.</span><span>Past performance does not guarantee future results.</span></footer>
    </main>
  </>;
}
