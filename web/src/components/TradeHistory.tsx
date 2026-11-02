import { tr } from '../i18n/config';
import { useMemo } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { DisplaySettings, TradeSortKey } from '../types/display';
import { ArrowLeftRight } from 'lucide-react';
import type { BacktestResult, Trade } from '../types/api';
import { number, utcTime } from '../lib/format';
import EmptyState from './EmptyState';

type SortKey = TradeSortKey;
const columns: {key: SortKey; label: string}[] = [{key:'time',label:"Time · UTC"},{key:'price',label:"Price"},{key:'quantity',label:"Quantity"},{key:'notional',label:"Notional"},{key:'fee',label:"Fee"}];
export default function TradeHistory({ result, display, setDisplay }: { result: BacktestResult | null; display: DisplaySettings; setDisplay: Dispatch<SetStateAction<DisplaySettings>> }) {
  const {side,sort}=display;
  const rows=useMemo(()=>[...(result?.trades??[])].filter(trade=>side==='all'||trade.type===side).sort((a,b)=>{
    const direction=sort.ascending?1:-1;
    return direction*(sort.key==='time'?Date.parse(a.time)-Date.parse(b.time):a[sort.key]-b[sort.key]);
  }),[result,side,sort]);
  const sortHeader=(key:SortKey,label:string)=><th key={key} scope="col" aria-sort={sort.key===key?(sort.ascending?'ascending':'descending'):'none'}><button type="button" onClick={()=>setDisplay(previous=>({...previous,sort:{key,ascending:previous.sort.key===key?!previous.sort.ascending:true}}))}>{tr(label)}{sort.key===key?(sort.ascending?' ↑':' ↓'):''}</button></th>;
  const cell=(trade:Trade,key:SortKey)=>key==='time'?<time dateTime={trade.time}>{utcTime(trade.time)}</time>:number(trade[key],key==='quantity'?8:key==='fee'?6:2);
  return <section className="panel history-panel" aria-labelledby="history-heading">
    <div className="panel-heading"><h3 id="history-heading">{tr("Trade history")}</h3><span className="panel-meta">{result?result.trades.length+tr(" orders"):tr("No trades yet")}</span></div>
    {!result?<EmptyState compact icon={ArrowLeftRight} title={tr("No backtest yet")} description={tr("Run a backtest to inspect its executed orders")} />:!result.trades.length?<EmptyState compact icon={ArrowLeftRight} title={tr("No trades were executed during this backtest.")} description={tr("The strategy produced no executed orders for this period.")} />:<>
      <div className="trade-controls"><div className="trade-filter"><label htmlFor="trade-side">{tr("Side")}</label><select id="trade-side" value={side} onChange={event=>setDisplay(previous=>({...previous,side:event.target.value as DisplaySettings['side']}))}><option value="all">{tr("All orders")}</option><option value="buy">{tr("BUY")}</option><option value="sell">{tr("SELL")}</option></select></div><span>{rows.length}{tr(' shown · Prices, notional and fees in USDT')}</span></div>
      {!rows.length?<p className="trade-no-matches">{tr("No orders match this filter")}</p>:<div className="trade-table-scroll" tabIndex={0} role="region" aria-label={tr("Trade history, scroll for more orders")}>
        <table className="trade-table"><caption className="sr-only">{tr("Executed orders. All times UTC; price, fee and notional in USDT.")}</caption>
          <thead><tr>{sortHeader('time',tr("Time · UTC"))}<th scope="col">{tr("Side")}</th>{columns.slice(1).map(column=>sortHeader(column.key,column.label))}</tr></thead>
          <tbody>{rows.map((trade,index)=><tr key={trade.time+'-'+index}><td>{cell(trade,'time')}</td><td><span className={'trade-side trade-'+trade.type}>{tr(trade.type.toUpperCase())}</span></td>{columns.slice(1).map(column=><td key={column.key}>{cell(trade,column.key)}</td>)}</tr>)}</tbody>
        </table>
      </div>}
    </>}
  </section>;
}
