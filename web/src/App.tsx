import { useTranslation } from 'react-i18next';
import { tr } from './i18n/config';
import { defaultDisplaySettings } from './types/display';
import { utcTime } from './lib/format';
import { useCandles } from './hooks/useCandles';
import { useEffect, useRef, useState } from 'react';
import Header from './components/Header';
import BacktestForm from './components/BacktestForm';
import ResultsPanel from './components/ResultsPanel';
import type { WorkspaceView } from './components/ResultsPanel';
import { strategyName } from './lib/format';
import RequestStatus from './components/RequestStatus';
import { defaultFormValues, validateForm } from './lib/backtestForm';
import { ApiError, createApiClient } from './api/client';
import type { BacktestFormValues, BacktestParameters } from './types/backtest';
import type { BacktestResult, Catalogs } from './types/api';

const api = createApiClient(import.meta.env.VITE_API_BASE_URL);
const message = (error: unknown) => error instanceof Error ? error.message : tr("The request failed. Please retry manually.");

export default function App() {
  useTranslation();
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
  const candles = useCandles(result, api);
  const [view, setView] = useState<WorkspaceView>('overview');
  const [collapsed, setCollapsed] = useState(false);
  const [display, setDisplay] = useState(defaultDisplaySettings);
  const [completedParameters, setCompletedParameters] = useState<BacktestParameters | null>(null);
  const draft = values && catalogs ? validateForm(values, catalogs).parameters : null;
  const draftChanged = !!result && (!draft || JSON.stringify(draft) !== JSON.stringify(completedParameters));

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
      if (request.current === controller) { setResult(next); setCompletedParameters(parameters); setDisplay(defaultDisplaySettings()); }
    } catch (error) {
      if (request.current === controller) {
        setError(message(error)); setCancelled(error instanceof ApiError && error.kind === 'cancelled');
      }
    } finally {
      if (request.current === controller) { request.current = null; setRunning(false); }
    }
  }

  return <>
    <a className="skip-link" href="#main">{tr("Skip to workspace")}</a>
    <Header />
    <main id="main" className="workspace">
      <h1 className="sr-only">{tr("TradeLab strategy backtesting")}</h1>
      {catalogLoading && <RequestStatus label={tr("Loading markets & strategies...")} onCancel={() => catalogRequest.current?.abort()} />}
      {catalogError && <div className="api-feedback" role="alert"><p>{tr(catalogError)}</p><button className="secondary-button" onClick={() => setAttempt(n => n + 1)}>{tr("Retry catalogs")}</button></div>}
      {running && <RequestStatus label={tr("Running Backtest...")} onCancel={() => request.current?.abort()} />}
      {error && <div className={'api-feedback' + (cancelled ? ' feedback-neutral' : '')} role="alert"><p>{tr(error)}</p><p>{tr("No result from this run is displayed. Adjust the form or run again manually.")}</p></div>}
      {catalogs && values && <>
        <nav className="workspace-tabs" aria-label={tr("Backtest sections")} role="tablist">
          {(['overview', 'charts', 'trades'] as const).map((item,index) => <button key={item} type="button" role="tab"
            id={'tab-'+item} aria-selected={view===item} aria-controls={'view-'+item} tabIndex={view===item?0:-1}
            onClick={()=>setView(item)} onKeyDown={event=>{
              const keys: Record<string,number>={ArrowRight:(index+1)%3,ArrowLeft:(index+2)%3,Home:0,End:2};
              if(keys[event.key]===undefined)return;
              event.preventDefault();
              const buttons=event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('button');
              buttons?.[keys[event.key]].click(); buttons?.[keys[event.key]].focus();
            }}>{item==='overview'?tr("Overview"):item==='charts'?tr("Charts"):tr("Trades")}</button>)}
        </nav>
        <div className="configuration-summary">
          <div className="run-context"><span className="summary-label">{result?tr("Completed backtest"):tr("Backtest configuration")}</span>
            {result?<p><strong>{result.market.symbol.replace(/USDT$/,'/USDT')}</strong><span>{result.market.interval}</span><span>{strategyName(result.strategy.type)}{result.strategy.type==='sma_crossover'?' '+result.strategy.fast_period+'/'+result.strategy.slow_period:''}</span><span>{utcTime(result.market.start)} — {utcTime(result.market.end)} UTC</span></p>
              :<p>{tr("Select a market, strategy and historical period")}</p>}
          </div>
          {(result || collapsed) && <button type="button" className="secondary-button" disabled={running} aria-expanded={!collapsed} aria-controls="configuration-region" onClick={()=>setCollapsed(value=>!value)}>{collapsed?tr("Edit settings"):tr("Collapse settings")}</button>}
        </div>
        {draftChanged && <p className="draft-notice" role="status">{tr("Settings changed — run a new backtest to update these results")}</p>}
        <div className={'dashboard'+(collapsed?' is-collapsed':'')+(view==='charts'?' charts-layout':'')}>
          <div id="configuration-region" className="configuration-region" hidden={collapsed}>
            <BacktestForm values={values} onChange={setValues} catalogs={catalogs} running={running} onRun={run} />
          </div>
          <ResultsPanel symbol={values.symbol} interval={values.interval} result={result} candles={candles.state} running={running} retryCandles={candles.retry} cancelCandles={candles.cancel} view={view} display={display} setDisplay={setDisplay} />
        </div>
      </>}

      <footer className="footer"><span>TradeLab <span className="footer-divider">/</span>{" "}{tr("Strategy research, made clear.")}</span><span>{tr("Past performance does not guarantee future results.")}</span></footer>
      <p className="chart-attribution">TradingView Lightweight Charts™ · Copyright (с) 2025 TradingView, Inc. <a href="https://www.tradingview.com/" target="_blank" rel="noreferrer">TradingView</a></p>
    </main>
  </>;
}
