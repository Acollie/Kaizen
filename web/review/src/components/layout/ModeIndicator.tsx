// Vim mode indicator (bottom centre): "NORMAL", "NORMAL · g", "INPUT".
import type { KeymapStatus } from '../../hooks/useKeymap';
import './ModeIndicator.css';

export function ModeIndicator({ status }: { status: KeymapStatus }) {
  return (
    <div className={`mode-indicator${status.pending ? ' pending' : ''}${status.mode === 'INPUT' ? ' input' : ''}`} aria-live="polite">
      {status.pending ? `${status.mode} · ${status.pending}` : status.mode}
    </div>
  );
}
