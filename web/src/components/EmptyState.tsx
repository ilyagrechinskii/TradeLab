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
      <p className="empty-title">{title}</p>
      <p className="empty-description">{description}</p>
    </div>
  );
}
