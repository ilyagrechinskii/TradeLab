export type TradeSortKey = 'time' | 'price' | 'quantity' | 'notional' | 'fee';
export interface DisplaySettings {
  sma: boolean;
  trades: boolean;
  side: 'all' | 'buy' | 'sell';
  sort: { key: TradeSortKey; ascending: boolean };
}
export const defaultDisplaySettings = (): DisplaySettings => ({ sma: true, trades: true, side: 'all', sort: { key: 'time', ascending: true } });
