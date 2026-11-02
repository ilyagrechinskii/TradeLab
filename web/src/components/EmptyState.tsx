import { tr } from '../i18n/config';
import type { LucideIcon } from 'lucide-react';

interface Props {
  icon: LucideIcon;
  title: string;
  description: string;
  compact?: boolean;
}

export default function EmptyState({ icon: Icon, title, description, compact = false }: Props) {
  return (
    <div className={'empty-state' + (compact ? ' empty-state-compact' : '')}>
      <span className="empty-icon"><Icon size={23} strokeWidth={1.5} aria-hidden="true" /></span>
      <p className="empty-title">{tr(title)}</p>
      <p className="empty-description">{tr(description)}</p>
    </div>
  );
}
