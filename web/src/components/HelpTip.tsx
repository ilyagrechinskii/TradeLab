import { tr } from '../i18n/config';
import { useId, useState } from 'react';

export default function HelpTip({ label, children }: { label: string; children: string }) {
  const id=useId();
  const [open,setOpen]=useState(false);
  return <><button type="button" className="help-trigger" aria-label={tr("About ")+tr(label)} aria-expanded={open} aria-controls={id} onClick={()=>setOpen(value=>!value)}>?</button><span className="help-content" id={id} hidden={!open}>{tr(children)}</span></>;
}
