import type { BacktestParameters, StrategyType } from './backtest.ts';

export interface Markets {
  instruments?: { symbol: string; name: string }[];
  symbols: string[];
  intervals: string[];
  limits: { max_candles: number; max_days: Record<string, number>; range: string; time_zone: string; warmup_included_in_limits: boolean };
}
export interface Parameter { default: number; minimum?: number; maximum?: number; minimum_exclusive?: number; maximum_exclusive?: number }
export interface StrategyDefinition { type: StrategyType; parameters: Record<string, Parameter>; constraints: string[] }
export interface Strategies { strategies: StrategyDefinition[]; configuration: { initial_capital: Parameter; fee_percent: Parameter; slippage_percent: Parameter } }
export interface Catalogs { markets: Markets; strategies: Strategies }
export interface Market { symbol: string; interval: string; start: string; end: string; source: string }
export interface Metrics { total_return: number; max_drawdown: number; completed_trades: number; win_rate: number | null; profit_factor: number | null }
export interface Portfolio { final_equity: number; cash: number; asset_quantity: number; total_fees: number }
export interface Trade { time: string; type: 'buy' | 'sell'; price: number; quantity: number; notional: number; fee: number; cash_after: number; asset_quantity_after: number }
export interface IndicatorPoint { candle_index: number; time: string; value: number }
export interface BacktestResult {
  market: Market;
  strategy: BacktestParameters['strategy'];
  configuration: { initial_capital: number; fee_percent: number; slippage_percent: number };
  metrics: Metrics; portfolio: Portfolio;
  benchmark: { strategy: string; metrics: Metrics; portfolio: Portfolio; excess_return: number };
  trades: Trade[]; equity_curve: { time: string; equity: number }[];
  indicators: { fast_sma: IndicatorPoint[]; slow_sma: IndicatorPoint[] };
}
export interface CandlesResult { market: Market; candles: { open_time: string; open: number; high: number; low: number; close: number; volume: number }[] }
