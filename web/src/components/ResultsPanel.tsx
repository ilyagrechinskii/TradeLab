import { tr } from '../i18n/config';
import type { Dispatch, SetStateAction } from 'react';
import type { DisplaySettings } from '../types/display';
import type { CandleState } from '../hooks/useCandles';
import MarketChart from './charts/MarketChart';
import EquityChart from './charts/EquityChart';
import type { BacktestResult } from '../types/api';
import { money, number, percent, signed, tone } from '../lib/format';
import BenchmarkComparison from './BenchmarkComparison';
import TradeHistory from './TradeHistory';
import { CandlestickChart, ChartNoAxesCombined } from 'lucide-react';
import type { Interval, Symbol } from '../types/backtest';
import MetricCard from './MetricCard';

export type WorkspaceView = 'overview' | 'charts' | 'trades';

const metrics = ['Total return', 'Final equity', 'Max drawdown', 'Win rate', 'Profit factor', 'Completed trades'];

export default function ResultsPanel({ symbol, interval, result, candles, running, retryCandles, cancelCandles, view, display, setDisplay }: { symbol: Symbol; interval: Interval; result: BacktestResult | null; candles: CandleState; running: boolean; retryCandles: () => void; cancelCandles: () => void; view: WorkspaceView; display: DisplaySettings; setDisplay: Dispatch<SetStateAction<DisplaySettings>> }) {
  const market = result?.market;
  const pair = (market?.symbol ?? symbol).replace(/USDT$/, '/USDT');
  const values = result ? [signed(result.metrics.total_return), money(result.portfolio.final_equity), percent(result.metrics.max_drawdown), percent(result.metrics.win_rate), number(result.metrics.profit_factor), number(result.metrics.completed_trades, 0)] : [];

  return (
    <section className="results" role="tabpanel" id={'view-'+view} aria-labelledby={'tab-'+view}>
      <h2 className="sr-only">{view==='overview'?tr("Performance overview"):view==='charts'?tr("Market analysis"):tr("Executed trades")}</h2>
      {view==='overview' && <div className="overview-metrics">
        <dl className="metrics-grid primary-metrics">{metrics.slice(0,3).map((metric,index)=><MetricCard key={metric} label={metric} value={values[index]} valueClass={index===0&&result?tone(result.metrics.total_return):''} />)}</dl>
        <dl className="metrics-grid secondary-metrics">{metrics.slice(3).map((metric,index)=><MetricCard key={metric} label={metric} value={values[index+3]} />)}</dl>
      </div>}
      {view==='overview' && result && <dl className="result-context" aria-label={tr("Completed backtest details")}>
        <div><dt>{tr("Initial capital")}</dt><dd>{money(result.configuration.initial_capital)}</dd></div>
        <div><dt>{tr("Total fees")}</dt><dd>{money(result.portfolio.total_fees)}</dd></div>
        <div><dt>{tr("Market data")}</dt><dd>{result.market.source}</dd></div>
      </dl>}
      {view==='charts' && <section className="panel price-panel" aria-labelledby="price-heading">
        <div className="panel-heading price-heading">
          <div className="chart-heading-copy">
            <h3 id="price-heading"><CandlestickChart size={16} strokeWidth={1.5} aria-hidden="true" />{tr("Market chart")}</h3>

          </div>
          <span className="sr-only">{pair} · {market?.interval ?? interval}</span>
        </div>
        <MarketChart result={result} candles={candles} running={running} retry={retryCandles} cancel={cancelCandles} display={display} setDisplay={setDisplay} />
        <div className="panel-footnote"><span>{tr("Historical candles · execution at Open N+1")}</span><span>{tr("Time · UTC")}</span></div>
      </section>}
      {view==='overview' && <section className="panel equity-panel" aria-labelledby="equity-heading">
        <div className="panel-heading"><h3 id="equity-heading"><ChartNoAxesCombined size={16} aria-hidden="true" />{tr("Equity curve")}</h3><span className="panel-meta">{tr("Portfolio value · USDT")}</span></div>
        <EquityChart result={result} running={running} />
      </section>}
      {view==='overview' && <div className="overview-comparison"><BenchmarkComparison result={result} /></div>}
      {view==='trades' && <TradeHistory result={result} display={display} setDisplay={setDisplay} />}
    </section>
  );
}
