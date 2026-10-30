import type { Catalogs } from '../types/api';
import { strategyName } from '../lib/format';
import { useState } from 'react';
import type { FormEvent, ReactNode } from 'react';
import { ArrowRight, Info, SlidersHorizontal } from 'lucide-react';
import type { BacktestFormValues, BacktestParameters, FormErrors, Interval, Symbol, StrategyType } from '../types/backtest';
import { formatCount, utcDate, validateForm } from '../lib/backtestForm';

function Field({ name, label, error, children }: {
  name: keyof BacktestFormValues; label: string; error?: string; children: ReactNode;
}) {
  return (
    <div className="field">
      <label htmlFor={name}>{label}</label>
      {children}
      {error && <span className="field-error" id={name + '-error'}>{error}</span>}
    </div>
  );
}

export default function BacktestForm({ values, onChange, catalogs, running, onRun }: {
  values: BacktestFormValues;
  catalogs: Catalogs; running: boolean; onRun: (parameters: BacktestParameters) => void;
  onChange: (values: BacktestFormValues) => void;
}) {
  const [touched, setTouched] = useState<Partial<Record<keyof BacktestFormValues, boolean>>>({});
  const validation = validateForm(values, catalogs);
  const MAX_CANDLES = catalogs.markets.limits.max_candles;
  const sma = catalogs.strategies.strategies.find(s => s.type === 'sma_crossover');
  const MAX_SMA_PERIOD = sma?.parameters.slow_period.maximum ?? MAX_CANDLES - 2;
  const errors: FormErrors = validation.errors;
  const valid = validation.parameters !== null;
  const update = <K extends keyof BacktestFormValues>(name: K, value: BacktestFormValues[K]) => {
    onChange({ ...values, [name]: value });
  };
  const error = (name: keyof BacktestFormValues) => touched[name] ? errors[name] : undefined;
  const attrs = (name: keyof BacktestFormValues) => ({
    id: name, name, value: values[name], disabled: running, 'aria-invalid': !!error(name),
    'aria-describedby': error(name) ? name + '-error' : undefined,
    onBlur: () => setTouched(previous => ({ ...previous, [name]: true })),
  });
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!validation.parameters || running) return;
    onRun(validation.parameters);
  };
  const numberField = (name: 'fastPeriod' | 'slowPeriod' | 'initialCapital' | 'feePercent' | 'slippagePercent', label: string, suffix: string, min: number, max?: number, integer = false) => (
    <Field name={name} label={label} error={error(name)}>
      <div className="input-with-unit">
        <input {...attrs(name)} type="number" required min={min} max={max} step={integer ? 1 : 'any'} inputMode={integer ? 'numeric' : 'decimal'} onChange={event => update(name, event.target.value)} />
        {suffix && <span aria-hidden="true">{suffix}</span>}
      </div>
    </Field>
  );

  return (
    <aside className="configuration panel" aria-labelledby="configuration-heading">
      <div className="configuration-heading"><SlidersHorizontal size={17} aria-hidden="true" /><h2 id="configuration-heading">Backtest Configuration</h2></div>
      <form onSubmit={submit} noValidate>
        <div className="configuration-fields">
        <fieldset>
          <legend><span className="section-number">01</span>Market</legend>
          <Field name="symbol" label="Trading pair">
            <select {...attrs('symbol')} onChange={event => update('symbol', event.target.value as Symbol)}>
              {catalogs.markets.symbols.map(symbol => <option key={symbol} value={symbol}>{symbol.replace(/USDT$/, ' / USDT')}</option>)}
            </select>
          </Field>
          <Field name="interval" label="Timeframe">
            <select {...attrs('interval')} onChange={event => update('interval', event.target.value as Interval)}>
              {catalogs.markets.intervals.map(interval => <option key={interval} value={interval}>{interval}</option>)}
            </select>
          </Field>
          <div className="field-row date-row">
            <Field name="start" label="Start date" error={error('start')}>
              <input {...attrs('start')} type="date" required min="1970-01-01" max={utcDate(new Date())} onChange={event => update('start', event.target.value)} />
            </Field>
            <Field name="end" label="End date" error={error('end')}>
              <input {...attrs('end')} type="date" required min="1970-01-02" max={utcDate(new Date())} onChange={event => update('end', event.target.value)} />
            </Field>
          </div>
          <p className="field-hint">UTC · End date excluded · Up to {catalogs.markets.limits.max_days[values.interval]} days including warm-up</p>
        </fieldset>
        <fieldset>
          <legend><span className="section-number">02</span>Strategy</legend>
          <Field name="strategy" label="Strategy type">
            <select {...attrs('strategy')} onChange={event => update('strategy', event.target.value as StrategyType)}>
              {catalogs.strategies.strategies.map(strategy => <option key={strategy.type} value={strategy.type}>{strategyName(strategy.type)}</option>)}
            </select>
          </Field>
          <p className="strategy-description">{values.strategy === 'sma_crossover' ? 'Trade when the fast and slow moving averages cross.' : 'Buy the asset once and hold it over the selected period.'}</p>
          {values.strategy === 'sma_crossover' && (
            <div className="field-row">
              {numberField('fastPeriod', 'Fast Period', '', sma?.parameters.fast_period.minimum ?? 1, sma?.parameters.fast_period.maximum ?? MAX_SMA_PERIOD - 1, true)}
              {numberField('slowPeriod', 'Slow Period', '', sma?.parameters.slow_period.minimum ?? 2, MAX_SMA_PERIOD, true)}
            </div>
          )}
        </fieldset>
        <fieldset>
          <legend><span className="section-number">03</span>Simulation</legend>
          {numberField('initialCapital', 'Initial Capital (USDT)', 'USDT', 0)}
          <div className="field-row">
            {numberField('feePercent', 'Trading Fee (%)', '%', 0, 100)}
            {numberField('slippagePercent', 'Slippage (%)', '%', 0, 100)}
          </div>
        </fieldset>
        <div className="form-summary">
          <Info size={15} aria-hidden="true" />
          <p>{validation.candles !== null && valid
            ? <><strong>{formatCount(validation.candles)} trading candles</strong>{validation.warmup > 0 ? ' + ' + formatCount(validation.warmup) + ' warm-up' : ' · no warm-up'}</>
            : <>Up to {formatCount(MAX_CANDLES)} candles, including strategy warm-up.</>}</p>
        </div>
        {!valid && <p className="validation-summary" role="status">{Object.values(errors)[0]}</p>}
        </div>
        <div className="configuration-footer">
          <p className="form-footnote">Historical data.<br />No live trading.</p>
          <button className="run-button" type="submit" disabled={!valid || running}>{running ? 'Running Backtest...' : 'Run Backtest'}<ArrowRight size={17} aria-hidden="true" /></button>
        </div>
      </form>
    </aside>
  );
}
