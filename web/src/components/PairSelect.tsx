import { useRef, useState } from 'react';
import type { Markets } from '../types/api';
import { tr } from '../i18n/config';

type Props = {
  markets: Markets;
  value: string;
  onChange: (symbol: string) => void;
  disabled: boolean;
};

export default function PairSelect({ markets, value, onChange, disabled }: Props) {
  const [search, setSearch] = useState('');
  const selectRef = useRef<HTMLSelectElement>(null);
  const label = (symbol: string) => {
    const name = markets.instruments?.find(item => item.symbol === symbol)?.name;
    return symbol.replace(/USDT$/, '/USDT') + (name ? ' — ' + name : '');
  };
  const query = search.trim().toLowerCase();
  const matches = markets.symbols.filter(symbol => label(symbol).toLowerCase().includes(query));

  return (
    <div className="pair-picker">
      <label className="sr-only" htmlFor="pair-search">{tr('Search pair')}</label>
      <input
        id="pair-search"
        type="search"
        value={search}
        placeholder={tr('Search by ticker or name')}
        disabled={disabled}
        onKeyDown={event => {
          if (event.key === 'Enter' || event.key === 'ArrowDown') {
            event.preventDefault();
            selectRef.current?.focus();
          }
          if (event.key === 'Escape') setSearch('');
        }}
        onChange={event => setSearch(event.target.value)}
      />
      <select
        ref={selectRef}
        id="symbol"
        name="symbol"
        value={value}
        disabled={disabled}
        onChange={event => {
          onChange(event.target.value);
          setSearch('');
        }}
      >
        {!matches.includes(value) && (
          <option value={value}>{label(value)} · {tr('Selected')}</option>
        )}
        {matches.map(symbol => <option key={symbol} value={symbol}>{label(symbol)}</option>)}
      </select>
      {!matches.length && <span className="field-hint" role="status">{tr('No pairs found')}</span>}
    </div>
  );
}
