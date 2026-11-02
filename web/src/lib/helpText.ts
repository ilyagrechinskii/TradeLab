export const metricHelp: Record<string,string> = {
  'Total return': 'Percentage change in portfolio value over this backtest, after trading costs.',
  'Final equity': 'Cash plus the value of any crypto still held at the end of the period.',
  'Max drawdown': 'The largest percentage fall from a previous portfolio peak during this backtest.',
  'Win rate': 'The percentage of completed positions that made a profit after fees. A dash means there are no completed positions.',
  'Profit factor': 'Total profit divided by total loss across completed positions. A dash means the ratio is undefined.',
  'Completed trades': 'Completed buy–sell pairs. The Trades tab lists each individual order separately.',
};
export const fieldHelp: Record<string,string> = {
  fastPeriod: 'Number of candles in the fast moving average. It must be smaller than Slow Period.',
  slowPeriod: 'Number of candles in the slow moving average. These extra warm-up candles count toward the history limits.',
  initialCapital: 'Virtual starting capital in USDT. No real money is used.',
  feePercent: 'Commission charged on each buy or sell, in percent. Enter 0.1 for a 0.1% fee.',
  slippagePercent: 'Models a worse execution price: buys cost more and sells receive less than the historical open price.',
};
