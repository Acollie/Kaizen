// Findings grouped Must fix / Should fix / Consider / Good, near-duplicates folded
// together, limited to the clusters the triage filter shows.

import { useMemo } from 'react';
import type { Finding } from '../../types';
import type { FindingCluster } from '../../lib/findings';
import { severitySections } from './reviewModel';
import { FindingItem } from './FindingItem';

interface FindingSectionsProps {
  findings: readonly Finding[];
  showCluster?: (cluster: FindingCluster) => boolean;
  emptyText?: string;
}

export function FindingSections({ findings, showCluster = () => true, emptyText = '' }: FindingSectionsProps) {
  const sections = useMemo(() => severitySections(findings), [findings]);
  const visible = sections
    .map((section) => ({ ...section, clusters: section.clusters.filter(showCluster) }))
    .filter((section) => section.clusters.length > 0);
  if (visible.length === 0) return emptyText ? <div className="rv-empty">{emptyText}</div> : null;
  return (
    <div className="rv-findings">
      {visible.map((section) => (
        <section key={section.severity} className={`rv-section sev-${section.severity}`} aria-label={section.title}>
          <h3 className="rv-section-head">
            <span className="label rv-section-title">{section.title}</span>
            <span className="rv-section-count mono">{section.clusters.length}</span>
          </h3>
          {section.clusters.map((cluster) => <FindingItem key={cluster.lead.id} cluster={cluster} />)}
        </section>
      ))}
    </div>
  );
}
