import { useTranslation } from 'react-i18next';
import { tr } from '../../i18n/config';
import { useEffect, useMemo, useRef, useState } from 'react';
import { createChart, LineSeries, LineStyle } from 'lightweight-charts';
import type { IChartApi, ISeriesApi, LineData, MouseEventParams, IPriceLine, UTCTimestamp } from 'lightweight-charts';
import type { BacktestResult } from '../../types/api';
import { equityChartData } from '../../lib/chartData';
import { money, signed, tone, utcTime } from '../../lib/format';
import { chartOptions } from './chartOptions';
import ChartEmptyState from './ChartEmptyState';

export default function EquityChart({ result, running }: { result: BacktestResult | null; running: boolean }) {
  const prepared = useMemo(() => {
    try { return { points: result ? equityChartData(result) : [], error: '' }; }
    catch (error) { return { points: [], error: error instanceof Error ? error.message : tr("Equity data is invalid.") }; }
  }, [result]);
  if (prepared.error) return <div className="chart-error" role="alert">{tr(prepared.error)}</div>;
  if (!result || !prepared.points.length) return <ChartEmptyState loading={running} title={running ? tr("Calculating portfolio history...") : result ? tr("No equity history was returned") : tr("See how your capital evolves")} description={tr("Portfolio values are calculated by the Go backend. No synthetic values are added.")} />;
  return <EquityPlot result={result} points={prepared.points} />;
}
function EquityPlot({ result, points }: { result: BacktestResult; points: LineData<UTCTimestamp>[] }) {
  const { i18n } = useTranslation();
  const initialPriceLine = useRef<IPriceLine | null>(null);
  const container = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi | null>(null);
  const series = useRef<ISeriesApi<'Line'> | null>(null);
  const pointRef = useRef(points);
  const [selected, setSelected] = useState<LineData<UTCTimestamp> | null>(null);
  const interval = result.market.interval;
  useEffect(() => {
    chart.current?.applyOptions({ localization: chartOptions(0, interval).localization });
    initialPriceLine.current?.applyOptions({title:tr("Initial")});
  }, [i18n.resolvedLanguage]);
  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const instance = createChart(element, { ...chartOptions(element.clientHeight, interval), width: element.clientWidth });
    chart.current = instance;
    series.current = instance.addSeries(LineSeries, {
      color: '#D4D4D4', lineWidth: 2, priceLineVisible: false,
      priceFormat: { type: 'custom', minMove: .01, formatter: (value: number) => money(value) },
    });
    const onMove = (event: MouseEventParams) => setSelected(pointRef.current.find(p => p.time === event.time) ?? null);
    instance.subscribeCrosshairMove(onMove);
    const resize = new ResizeObserver(() => { instance.resize(element.clientWidth, element.clientHeight); instance.timeScale().fitContent(); });
    resize.observe(element);
    return () => { resize.disconnect(); instance.unsubscribeCrosshairMove(onMove); instance.remove(); chart.current = null; series.current = null; };
  }, []);
  useEffect(() => {
    pointRef.current = points; series.current?.setData(points);
    const initialLine = series.current?.createPriceLine({ price: result.configuration.initial_capital, color: '#777777', lineWidth: 1, lineStyle: LineStyle.Dashed, axisLabelVisible: true, title: tr("Initial") });
    initialPriceLine.current = initialLine ?? null;
    chart.current?.applyOptions({ timeScale: { timeVisible: !interval.endsWith('d') } });
    chart.current?.timeScale().fitContent(); setSelected(null);
    const currentSeries = series.current;
    return () => { if (initialLine && currentSeries && series.current === currentSeries) currentSeries.removePriceLine(initialLine); };
  }, [points, interval, result.configuration.initial_capital]);
  const shown = selected ?? points.at(-1)!;
  return <div>
    <div className="equity-summary">
      <dl><div><dt>{tr("Initial capital")}</dt><dd>{money(result.configuration.initial_capital)}</dd></div><div><dt>{tr("Final equity")}</dt><dd>{money(result.portfolio.final_equity)}</dd></div><div><dt>{tr("Total Return")}</dt><dd className={tone(result.metrics.total_return)}>{signed(result.metrics.total_return)}</dd></div></dl>
      <button type="button" className="secondary-button" onClick={() => chart.current?.timeScale().fitContent()}>{tr("Reset equity view")}</button>
    </div>
    <p className="equity-readout">{utcTime(new Date(shown.time * 1000).toISOString())} UTC <strong>{money(shown.value)}</strong></p>
    <div ref={container} className="chart-canvas equity-canvas" role="img" aria-label={tr("Interactive portfolio equity chart in USDT")} />
    <div className="panel-footnote">{tr("Equity at candle close · timestamps use candle open (UTC)")}</div>
  </div>;
}
