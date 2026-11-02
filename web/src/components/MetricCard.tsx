import { tr } from '../i18n/config';
import HelpTip from './HelpTip';
import { metricHelp } from '../lib/helpText';
export default function MetricCard({ label, value, valueClass = '' }: { label: string; value?: string; valueClass?: string }) {
  return <div className="metric-card">
    <dt><span>{tr(label)}</span>{metricHelp[label] && <HelpTip label={label}>{metricHelp[label]}</HelpTip>}</dt>
    <dd><span className={'metric-value ' + valueClass}>{value ?? '—'}</span>{!value && <span className="sr-only">{tr("No results yet")}</span>}</dd>
  </div>;
}
