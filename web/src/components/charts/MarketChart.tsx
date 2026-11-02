import { useTranslation } from 'react-i18next';
import { tr } from '../../i18n/config';
import type { Dispatch, SetStateAction } from 'react';
import type { DisplaySettings } from '../../types/display';
import { useEffect, useMemo, useRef, useState } from 'react';
import { CandlestickSeries, LineSeries, LineStyle, createChart, createSeriesMarkers } from 'lightweight-charts';
import type { IChartApi, ISeriesApi, ISeriesMarkersPluginApi, MouseEventParams, Time, UTCTimestamp } from 'lightweight-charts';
import type { BacktestResult, CandlesResult } from '../../types/api';
import type { CandleState } from '../../hooks/useCandles';
import { marketChartData, unixSeconds } from '../../lib/chartData';
import { number, utcTime } from '../../lib/format';
import { chartOptions } from './chartOptions';
import ChartEmptyState from './ChartEmptyState';

type PlotData = ReturnType<typeof marketChartData>;
export default function MarketChart({ result, candles, running, retry, cancel, display, setDisplay }: {
  result: BacktestResult | null; candles: CandleState; running: boolean; retry: () => void; cancel: () => void; display: DisplaySettings; setDisplay: Dispatch<SetStateAction<DisplaySettings>>;
}) {
  if (!result) return <ChartEmptyState loading={running} title={running ? tr("Preparing your backtest...") : tr("Your market. Your perspective.")} description={running ? tr("Candles will load automatically after the calculation completes.") : tr("Run a backtest to see historical candles, server SMA and executed orders.")} />;
  if (candles.status === 'loading' || candles.status === 'idle') return <div><ChartEmptyState loading title={tr("Loading trading candles...")} description={tr("One historical request for this completed backtest. No live market connection.")} /><div className="chart-state-actions"><button type="button" className="secondary-button" onClick={cancel}>{tr("Cancel candle loading")}</button></div></div>;
  if (candles.status === 'error') return <div><div className="chart-error" role="alert">{tr(candles.message)}</div><div className="chart-state-actions"><button type="button" className="secondary-button" onClick={retry}>{tr("Retry candles")}</button></div></div>;
  return <ValidatedMarketPlot result={result} response={candles.data} display={display} setDisplay={setDisplay} />;
}
function ValidatedMarketPlot({ result, response, display, setDisplay }: { result: BacktestResult; response: CandlesResult; display: DisplaySettings; setDisplay: Dispatch<SetStateAction<DisplaySettings>> }) {
  const prepared = useMemo(() => {
    try { return { data: marketChartData(result, response), error: '' }; }
    catch (error) { return { data: null, error: error instanceof Error ? error.message : tr("Chart data is invalid.") }; }
  }, [result, response]);
  return prepared.data ? <MarketPlot data={prepared.data} result={result} display={display} setDisplay={setDisplay} /> : <div className="chart-error" role="alert">{tr(prepared.error)}</div>;
}
function MarketPlot({ data, result, display, setDisplay }: { data: PlotData; result: BacktestResult; display: DisplaySettings; setDisplay: Dispatch<SetStateAction<DisplaySettings>> }) {
  const { i18n } = useTranslation();
  const container = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi | null>(null);
  const candleSeries = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const fastSeries = useRef<ISeriesApi<'Line'> | null>(null);
  const slowSeries = useRef<ISeriesApi<'Line'> | null>(null);
  const markers = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const [hoverTime, setHoverTime] = useState<UTCTimestamp | null>(null);
  const interval = result.market.interval;
  const {sma:showSma,trades:showTrades}=display;

  useEffect(() => {
    chart.current?.applyOptions({ localization: chartOptions(0, interval).localization });
    
  }, [i18n.resolvedLanguage]);
  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const instance = createChart(element, { ...chartOptions(element.clientHeight, interval), width: element.clientWidth });
    chart.current = instance;
    candleSeries.current = instance.addSeries(CandlestickSeries, {
      upColor: '#22C55E', downColor: '#EF4444', wickUpColor: '#22C55E', wickDownColor: '#EF4444', borderVisible: false,
      priceFormat: { type: 'price', precision: 2, minMove: .01 },
    });
    fastSeries.current = instance.addSeries(LineSeries, { color: '#D4D4D4', lineWidth: 2, lastValueVisible: false, priceLineVisible: false, crosshairMarkerVisible: false });
    slowSeries.current = instance.addSeries(LineSeries, { color: '#777777', lineWidth: 1, lineStyle: LineStyle.Dashed, lastValueVisible: false, priceLineVisible: false, crosshairMarkerVisible: false });
    markers.current = createSeriesMarkers(candleSeries.current, []);
    const onMove = (event: MouseEventParams) => setHoverTime(typeof event.time === 'number' ? event.time as UTCTimestamp : null);
    instance.subscribeCrosshairMove(onMove);
    instance.subscribeClick(onMove);
    const resize = new ResizeObserver(() => { instance.resize(element.clientWidth, element.clientHeight); instance.timeScale().fitContent(); });
    resize.observe(element);
    return () => {
      resize.disconnect();
      instance.unsubscribeCrosshairMove(onMove); instance.unsubscribeClick(onMove);
      markers.current?.detach(); markers.current = null;
      instance.remove(); chart.current = null; candleSeries.current = null; fastSeries.current = null; slowSeries.current = null;
    };
    // A result/hover update changes series data, not the chart instance.
  }, []);
  useEffect(() => {
    candleSeries.current?.setData(data.candles);
    fastSeries.current?.setData(data.fast); slowSeries.current?.setData(data.slow);
    fastSeries.current?.applyOptions({ visible: result.strategy.type === 'sma_crossover' });
    slowSeries.current?.applyOptions({ visible: result.strategy.type === 'sma_crossover' });
    markers.current?.setMarkers(data.markers.map(marker=>({...marker,text:undefined})));
    chart.current?.applyOptions({ timeScale: { timeVisible: !interval.endsWith('d') } });
    chart.current?.timeScale().fitContent(); setHoverTime(null);
  }, [data, interval, result.strategy.type]);

  useEffect(() => {
    fastSeries.current?.applyOptions({visible: showSma && result.strategy.type==='sma_crossover'});
    slowSeries.current?.applyOptions({visible: showSma && result.strategy.type==='sma_crossover'});
    markers.current?.setMarkers(showTrades ? data.markers.map(marker=>({...marker,text:undefined})) : []);
  },[showSma,showTrades,data,result.strategy.type]);

  const displayed = data.candles.find(c => c.time === hoverTime) ?? data.candles.at(-1)!;
  const trades = useMemo(() => result.trades.map(trade => ({ trade, time: unixSeconds(trade.time) })), [result.trades]);
  const selectedTrades = trades.filter(t => t.time === hoverTime).map(t => t.trade);
  return <div className="market-chart-content">
    <div className="chart-toolbar">
      <div className="chart-readout"><time dateTime={new Date(displayed.time * 1000).toISOString()}>{utcTime(new Date(displayed.time * 1000).toISOString())} UTC</time>
        <dl className="chart-values" aria-label={tr("OHLC values")}>{([['O', tr("Open"), displayed.open], ['H', tr("High"), displayed.high], ['L', tr("Low"), displayed.low], ['C', tr("Close"), displayed.close]] as const).map(([key,label,value]) => <div key={key}><dt><abbr title={label}>{key}</abbr></dt><dd>{number(value)}</dd></div>)}</dl>
      </div>
      <button type="button" className="secondary-button" onClick={() => chart.current?.timeScale().fitContent()}>{tr("Reset market view")}</button>
    </div>
    <div className="chart-legend chart-series-legend" aria-label={tr("Market chart legend")}>
      <span className="legend-candle">OHLC · USDT</span>
      {result.strategy.type==='sma_crossover' && <label className="chart-toggle"><input type="checkbox" aria-label="SMA" checked={showSma} onChange={event=>setDisplay(previous=>({...previous,sma:event.target.checked}))} />SMA <span className="sma-periods"><i className="legend-sma-fast" />{result.strategy.type==='sma_crossover'?result.strategy.fast_period:''}<i className="legend-sma-slow" />{result.strategy.type==='sma_crossover'?result.strategy.slow_period:''}</span></label>}
      <label className="chart-toggle"><input type="checkbox" checked={showTrades} onChange={event=>setDisplay(previous=>({...previous,trades:event.target.checked}))} />{tr("Trades")}</label>

      <span className="metric-value-positive">{tr("↑ BUY")}</span><span className="metric-value-negative">{tr("↓ SELL")}</span>
    </div>
    <div className="execution-readout" aria-live="polite">{showTrades && selectedTrades.length ? selectedTrades.map((trade,index) => <p key={index}><strong className={trade.type === 'buy' ? 'metric-value-positive' : 'metric-value-negative'}>{tr(trade.type.toUpperCase())}</strong> · {utcTime(trade.time)} UTC{tr(' · Price')} {number(trade.price)}{tr(' · Qty')} {number(trade.quantity,8)}{tr(' · Fee')} {number(trade.fee,6)} USDT</p>) : <p> </p>}</div>
    <div ref={container} className="chart-canvas market-canvas" role="img" aria-label={tr("Interactive historical candlestick chart with execution markers")} />
  </div>;
}
