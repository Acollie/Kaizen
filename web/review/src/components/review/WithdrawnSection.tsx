// Findings the lead reviewer's final result left out. They are not part of the review
// (no counts, no navigation, never posted) but stay visible here, collapsed, so nothing
// the AI raised silently disappears.

import { useMemo } from 'react';
import { useReview } from '../../state/StoreProvider';
import { formatLineRef } from '../../lib/findings';
import { AgentTag } from '../shared/bits';
import type { Finding } from '../../types';
import { Disclosure } from './Disclosure';

export function WithdrawnSection() {
  const withdrawn = useReview((state) => state.withdrawn);
  const order = useReview((state) => state.withdrawnOrder);
  const openLocation = useReview((state) => state.openLocation);
  const openFile = useReview((state) => state.openFile);
  const items = useMemo(() => order.map((id) => withdrawn[id]).filter((finding): finding is Finding => Boolean(finding)), [order, withdrawn]);
  if (items.length === 0) return null;
  const open = (finding: Finding) => (finding.line ? openLocation(finding.file, finding.line) : openFile(finding.file));
  return (
    <Disclosure title={`Withdrawn by the lead reviewer (${items.length})`}>
      <p className="rv-withdrawn-note">
        Raised during the review, then left out of the lead reviewer's final result. Not counted and never posted.
      </p>
      <ul className="rv-withdrawn">
        {items.map((finding) => (
          <li key={finding.id}>
            <button type="button" className="rv-withdrawn-row" onClick={() => open(finding)} title={finding.id}>
              <span className="rv-withdrawn-title">{finding.title}</span>
              <span className="rv-withdrawn-meta">
                <span className="mono">{finding.file ? `${finding.file}:${formatLineRef(finding)}` : 'PR'}</span>
                <AgentTag agent={finding.source} />
              </span>
            </button>
          </li>
        ))}
      </ul>
    </Disclosure>
  );
}
