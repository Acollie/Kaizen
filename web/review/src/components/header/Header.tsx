// PR title, author, branch and stats; severity chips that jump to the first finding of
// that severity; engine status; help and Post review.

import { useReview } from '../../state/StoreProvider';
import { isSafeHttpUrl } from '../../lib/format';
import { prMeta } from './headerModel';
import { SeverityChips } from './SeverityChips';
import { EngineStatusView } from './EngineStatus';
import './Header.css';

export function Header() {
  const setHelpOpen = useReview((state) => state.setHelpOpen);
  const setPostOpen = useReview((state) => state.setPostOpen);
  return (
    <header className="hd">
      <PrIdentity />
      <div className="hd-right">
        <SeverityChips />
        <EngineStatusView />
        <span className="hd-sep" aria-hidden="true" />
        <button type="button" className="btn hd-help" title="Keyboard shortcuts (?)" aria-label="Keyboard shortcuts" onClick={() => setHelpOpen(true)}>?</button>
        <button type="button" className="btn btn-primary hd-post" title="Post review to GitHub (P)" onClick={() => setPostOpen(true)}>Post review</button>
      </div>
    </header>
  );
}

function PrIdentity() {
  const pr = useReview((state) => state.context.pr);
  const meta = prMeta(pr);
  return (
    <div className="hd-left">
      <span className="hd-mark" aria-label="Kaizen">改</span>
      <div className="hd-identity">
        <div className="hd-title-line">
          <span className="hd-number mono">#{pr.number}</span>
          <span className="hd-title" title={pr.title}>{pr.title}</span>
        </div>
        <div className="hd-meta">
          {meta.author && <span className="hd-author">{meta.author}</span>}
          {meta.branches && <span className="mono hd-branches" title={meta.branches}>{meta.branches}</span>}
          <span className="mono hd-diffstat"><span className="hd-add">{meta.additions}</span> <span className="hd-del">{meta.deletions}</span></span>
          <span>{meta.files}</span>
          {isSafeHttpUrl(pr.url) && <a href={pr.url} target="_blank" rel="noreferrer noopener">View on GitHub</a>}
        </div>
      </div>
    </div>
  );
}
