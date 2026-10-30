import type { BacktestResult } from '../types/api';
import { money, number, percent, signed, strategyName, tone, utcTime } from '../lib/format';
import BenchmarkComparison from './BenchmarkComparison';
import TradeHistory from './TradeHistory';
import { ArrowDownRight, ArrowLeftRight, CandlestickChart, ChartNoAxesCombined, Crosshair, Percent, TrendingUp, Wallet } from 'lucide-react';
import type { Interval, Symbol } from '../types/backtest';
import MetricCard from './MetricCard';
import EmptyState from './EmptyState';

const metrics = [
  { label: 'Total Return', icon: TrendingUp },
  { label: 'Final Equity', icon: Wallet },
  { label: 'Max Drawdown', icon: ArrowDownRight },
  { label: 'Win Rate', icon: Crosshair },
  { label: 'Profit Factor', icon: Percent },
  { label: 'Completed Trades', icon: ArrowLeftRight },
];

export default function ResultsPanel({ symbol, interval, result }: { symbol: Symbol; interval: Interval; result: BacktestResult | null }) {
  const market = result?.market;
  const pair = (market?.symbol ?? symbol).replace(/USDT$/, '/USDT');
  const values = result ? [signed(result.metrics.total_return), money(result.portfolio.final_equity), percent(result.metrics.max_drawdown), percent(result.metrics.win_rate), number(result.metrics.profit_factor), number(result.metrics.completed_trades, 0)] : [];

  return (
    <section className="results" aria-labelledby="results-heading">
      <div className="results-heading">
        <div><h2 id="results-heading">Results overview</h2><p>Your strategy, measured against the market.</p></div>
        <span className="subtle-badge"><span className="status-dot" />{result ? 'Last completed run' : 'No results yet'}</span>
      </div>
      <dl className="metrics-grid">{metrics.map((metric, index) => <MetricCard key={metric.label} {...metric} value={values[index]} valueClass={index === 0 && result ? tone(result.metrics.total_return) : ''} />)}</dl>
      {result && <dl className="result-context" aria-label="Completed backtest details">
        <div><dt>Strategy</dt><dd>{strategyName(result.strategy.type)}{result.strategy.type === 'sma_crossover' ? ' · ' + result.strategy.fast_period + '/' + result.strategy.slow_period : ''}</dd></div>
        <div><dt>Market</dt><dd>{result.market.symbol} · {result.market.interval}</dd></div>
        <div><dt>Period · UTC · end excluded</dt><dd>{utcTime(result.market.start)} — {utcTime(result.market.end)}</dd></div>
        <div><dt>Initial Capital</dt><dd>{money(result.configuration.initial_capital)}</dd></div>
        <div><dt>Total Fees</dt><dd>{money(result.portfolio.total_fees)}</dd></div>
        <div><dt>Data source</dt><dd>{result.market.source}</dd></div>
      </dl>}
      <section className="panel price-panel" aria-labelledby="price-heading">
        <div className="panel-heading price-heading">
          <div className="chart-heading-copy">
            <h3 id="price-heading"><CandlestickChart size={17} aria-hidden="true" />Market overview</h3>
            <p>Historical price action & strategy execution</p>
          </div>
          <span className="instrument">{pair}<span className="instrument-interval">{market?.interval ?? interval}</span></span>
        </div>
        <div className="chart-toolbar">
          <dl className="chart-values" aria-label="Market values, not loaded">
            {['Open', 'High', 'Low', 'Close'].map(label => <div key={label}><dt>{label}</dt><dd>—</dd></div>)}
          </dl>
          <div className="chart-legend"><span><i className="legend-price" />Price</span><span><i className="legend-signal" />Signals</span></div>
        </div>
        <div className="chart-placeholder" aria-describedby="chart-preview-note">
          <div className="chart-scale-label" aria-hidden="true">RELATIVE SCALE · %</div>
          <div className="chart-plot">
            <div className="chart-grid" aria-hidden="true" />
            <div className="chart-y-axis" aria-hidden="true">{[100, 75, 50, 25, 0].map(tick => <span key={tick}>{tick}</span>)}</div>
            <div className="chart-empty-overlay">
              <span className="chart-preview-badge">{result ? 'DATA READY' : 'CHART PREVIEW'}</span>
              <EmptyState icon={CandlestickChart} title={result ? "Backtest data is ready" : "Your market. Your perspective."} description={result ? "Chart visualization will be available in the next stage." : "Historical candles and strategy signals will appear here. No market data has been loaded."} />
            </div>
          </div>
          <div className="chart-x-axis" aria-hidden="true">{['Start', '25%', '50%', '75%', 'End'].map(tick => <span key={tick}>{tick}</span>)}</div>
          <p className="chart-axis-caption">Position within the selected period</p>
        </div>
        <div className="panel-footnote"><span id="chart-preview-note">Illustrative axes only · not market prices</span><span>Time · UTC</span></div>
      </section>
      <section className="panel equity-panel" aria-labelledby="equity-heading">
        <div className="panel-heading"><h3 id="equity-heading"><ChartNoAxesCombined size={16} aria-hidden="true" />Equity Curve</h3><span className="panel-meta">Portfolio value · USDT</span></div>
        <EmptyState compact icon={TrendingUp} title="See how your capital evolves" description={result ? "Chart visualization will be available in the next stage. " + result.equity_curve.length + " equity points received." : "Portfolio performance will appear after a backtest is available."} />
      </section>
      <div className="results-bottom">
        <BenchmarkComparison result={result} />
        <TradeHistory result={result} />
      </div>
      <p className="results-note">{result ? 'Displayed values belong to the last completed run shown above. All calculations are performed by the Go backend.' : 'Configure your strategy and run a backtest to see results.'}</p>
    </section>
  );
}
