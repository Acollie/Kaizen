// Findings grouped Must fix / Should fix / Consider / Good, near-duplicates folded
// together, limited to the clusters the triage filter shows. With collapseNits, the
// suggestions start folded under "N suggestions (triage the warnings first)" while any
// critical or warning finding is unreviewed; they can be expanded at any time.

import { useMemo, useState } from 'react';
import type { Finding } from '../../types';
import type { FindingCluster } from '../../lib/findings';
import { useReview } from '../../state/StoreProvider';
import { collapsedNitsLabel, nitsCollapsed } from '../../state/verifyQueue';
import { severitySections, type SeveritySection } from './reviewModel';
import { FindingItem } from './FindingItem';

interface FindingSectionsProps {
  findings: readonly Finding[];
  showCluster?: (cluster: FindingCluster) => boolean;
  emptyText?: string;
  /** Fold suggestions while urgent findings are unreviewed */
  collapseNits?: boolean;
}

export function FindingSections({ findings, showCluster = () => true, emptyText = '', collapseNits = false }: FindingSectionsProps) {
  const sections = useMemo(() => severitySections(findings), [findings]);
  const triage = useReview((state) => state.triage);
  const [nitsOpen, setNitsOpen] = useState(false);
  const folded = collapseNits && !nitsOpen && nitsCollapsed(findings, triage);
  const visible = sections
    .map((section) => ({ ...section, clusters: section.clusters.filter(showCluster) }))
    .filter((section) => section.clusters.length > 0);
  if (visible.length === 0) return emptyText ? <div className="rv-empty">{emptyText}</div> : null;
  return (
    <div className="rv-findings">
      {visible.map((section) => (section.severity === 'suggestion' && folded
        ? <FoldedNits key={section.severity} section={section} onOpen={() => setNitsOpen(true)} />
        : (
          <section key={section.severity} className={`rv-section sev-${section.severity}`} aria-label={section.title}>
            <h3 className="rv-section-head">
              <span className="label rv-section-title">{section.title}</span>
              <span className="rv-section-count mono">{section.clusters.length}</span>
              {collapseNits && section.severity === 'suggestion' && nitsOpen && (
                <button type="button" className="tr-link rv-nits-fold" onClick={() => setNitsOpen(false)}>Collapse</button>
              )}
            </h3>
            {section.clusters.map((cluster) => <FindingItem key={cluster.lead.id} cluster={cluster} />)}
          </section>
        )))}
    </div>
  );
}

function FoldedNits({ section, onOpen }: { section: SeveritySection; onOpen(): void }) {
  const count = section.clusters.reduce((sum, cluster) => sum + 1 + cluster.similar.length, 0);
  return (
    <section className="rv-section sev-suggestion rv-nits" aria-label={section.title}>
      <button type="button" className="rv-nits-toggle" aria-expanded={false} onClick={onOpen}>
        <svg className="rv-chevron" width="8" height="8" viewBox="0 0 8 8" aria-hidden="true"><path d="M2 1l4 3-4 3z" fill="currentColor" /></svg>
        {collapsedNitsLabel(count)}
      </button>
    </section>
  );
}
