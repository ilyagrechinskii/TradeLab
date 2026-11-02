import { tr } from '../i18n/config';
import { GitCompareArrows, Layers } from 'lucide-react';
import type { BacktestResult } from '../types/api';
import { money, signed, strategyName, tone } from '../lib/format';
import EmptyState from './EmptyState';

export default function BenchmarkComparison({ result }: { result: BacktestResult | null }) {
  return <section className="panel comparison-panel" aria-labelledby="comparison-heading">
    <div className="panel-heading"><h3 id="comparison-heading"><GitCompareArrows size={16} aria-hidden="true" />{tr("Strategy vs Buy & Hold")}</h3></div>
    {!result ? <EmptyState compact icon={Layers} title={tr("Put performance in perspective")} description={tr("Compare your strategy with holding the same asset over the same period.")} />
      : result.strategy.type === 'buy_and_hold' ? <EmptyState compact icon={Layers} title={tr("Buy & Hold selected")} description={tr("This run is the benchmark itself. A comparison with the same strategy is not shown.")} />
      : <dl className="benchmark-values">
        <div><dt>{strategyName(result.strategy.type)}</dt><dd><strong className={tone(result.metrics.total_return)}>{signed(result.metrics.total_return)}</strong><span>{money(result.portfolio.final_equity)}</span></dd></div>
        <div><dt>{tr("Buy & Hold")}</dt><dd><strong className={tone(result.benchmark.metrics.total_return)}>{signed(result.benchmark.metrics.total_return)}</strong><span>{money(result.benchmark.portfolio.final_equity)}</span></dd></div>
        <div className="benchmark-difference"><dt>{tr("Excess Return")}{" "}<span>{tr("(percentage points)")}</span></dt><dd className={tone(result.benchmark.excess_return)}>{signed(result.benchmark.excess_return, tr(" pp"))}</dd></div>
      </dl>}
  </section>;
}
