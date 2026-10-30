export type Symbol = string;
export type Interval = string;
export type StrategyType = 'sma_crossover' | 'buy_and_hold';

export interface BacktestFormValues {
  symbol: Symbol;
  interval: Interval;
  start: string;
  end: string;
  strategy: StrategyType;
  fastPeriod: string;
  slowPeriod: string;
  initialCapital: string;
  feePercent: string;
  slippagePercent: string;
}

export type FormErrors = Partial<Record<keyof BacktestFormValues, string>>;

// Exact JSON request contract of the Go API.
export interface BacktestParameters {
  symbol: Symbol;
  interval: Interval;
  start: string;
  end: string;
  initial_capital: number;
  fee_percent: number;
  slippage_percent: number;
  strategy:
    | { type: 'sma_crossover'; fast_period: number; slow_period: number }
    | { type: 'buy_and_hold' };
}
