import { tr } from '../i18n/config';
import { useEffect, useState } from 'react';
export default function RequestStatus({ label, onCancel }: { label: string; onCancel: () => void }) {
  const [slow, setSlow] = useState(false);
  useEffect(() => { const timer = setTimeout(() => setSlow(true), 8000); return () => clearTimeout(timer); }, []);
  return <div className="request-status" role="status" aria-live="polite">
    <div><strong>{tr(label)}</strong><p>{slow ? tr("Render Free may be waking up, or market data is still loading. You can keep waiting or cancel.") : tr("Waiting for the API. The first request can take longer on Render Free.")}</p></div>
    <button type="button" className="secondary-button" onClick={onCancel}>{tr("Cancel request")}</button>
  </div>;
}
