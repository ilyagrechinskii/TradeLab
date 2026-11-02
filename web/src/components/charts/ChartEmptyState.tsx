import { CandlestickChart } from 'lucide-react';
import EmptyState from '../EmptyState';
export default function ChartEmptyState({ title, description, loading = false }: { title: string; description: string; loading?: boolean }) {
  return <div className={'chart-empty-surface' + (loading ? ' chart-loading' : '')} role={loading ? 'status' : undefined} aria-busy={loading}>
    <EmptyState icon={CandlestickChart} title={title} description={description} />
  </div>;
}
