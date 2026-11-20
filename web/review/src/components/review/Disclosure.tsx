// A quiet collapsible section: hairline above, label + optional count, chevron.

import { useState, type ReactNode } from 'react';

interface DisclosureProps {
  title: string;
  count?: number;
  hint?: string;
  defaultOpen?: boolean;
  children: ReactNode;
}

export function Disclosure({ title, count, hint, defaultOpen = false, children }: DisclosureProps) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className="rv-disclosure">
      <button type="button" className="rv-disclosure-head" aria-expanded={open} onClick={() => setOpen(!open)}>
        <svg className={`rv-chevron${open ? ' rv-open' : ''}`} width="8" height="8" viewBox="0 0 8 8" aria-hidden="true"><path d="M2 1l4 3-4 3z" fill="currentColor" /></svg>
        <span className="label">{title}</span>
        {count !== undefined && <span className="rv-section-count mono">{count}</span>}
        {hint && <span className="rv-disclosure-hint">{hint}</span>}
      </button>
      {open && <div className="rv-disclosure-body">{children}</div>}
    </section>
  );
}
