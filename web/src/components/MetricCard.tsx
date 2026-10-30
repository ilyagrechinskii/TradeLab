import type { LucideIcon } from 'lucide-react';

export default function MetricCard({ label, icon: Icon, value, valueClass = '' }: { label: string; icon: LucideIcon; value?: string; valueClass?: string }) {
  return <div className="metric-card">
    <dt><span>{label}</span><Icon size={14} aria-hidden="true" /></dt>
    <dd><span className={'metric-value ' + valueClass}>{value ?? '—'}</span>{!value && <span className="sr-only">No results yet</span>}<span className="metric-caption">{value ? 'Last completed run' : 'Awaiting backtest'}</span></dd>
  </div>;
}
