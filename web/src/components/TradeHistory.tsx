import { ArrowLeftRight, History } from 'lucide-react';
import type { BacktestResult } from '../types/api';
import { number, utcTime } from '../lib/format';
import EmptyState from './EmptyState';

export default function TradeHistory({ result }: { result: BacktestResult | null }) {
  return <section className="panel history-panel" aria-labelledby="history-heading">
    <div className="panel-heading"><h3 id="history-heading"><History size={16} aria-hidden="true" />Trade History</h3><span className="panel-meta">{result ? result.trades.length + ' orders' : 'No trades yet'}</span></div>
    {!result ? <EmptyState compact icon={ArrowLeftRight} title="Every trade, accounted for" description="Executed orders, prices and fees will be listed here." />
      : !result.trades.length ? <EmptyState compact icon={ArrowLeftRight} title="No trades were executed during this backtest." description="The strategy produced no executed orders for this period." />
      : <div className="trade-table-scroll" tabIndex={0} role="region" aria-label="Trade history, scroll for more orders">
        <table className="trade-table"><caption className="sr-only">Executed orders. All times UTC; price, fee and notional in USDT.</caption>
          <thead><tr><th scope="col">Time · UTC</th><th scope="col">Type</th><th scope="col">Price</th><th scope="col">Quantity</th><th scope="col">Fee</th><th scope="col">Notional</th></tr></thead>
          <tbody>{result.trades.map((trade, index) => <tr key={trade.time + '-' + index}>
            <td><time dateTime={trade.time}>{utcTime(trade.time)}</time></td><td><span className={'trade-side trade-' + trade.type}>{trade.type.toUpperCase()}</span></td>
            <td>{number(trade.price)}</td><td>{number(trade.quantity, 8)}</td><td>{number(trade.fee, 6)}</td><td>{number(trade.notional)}</td>
          </tr>)}</tbody>
        </table>
      </div>}
  </section>;
}
