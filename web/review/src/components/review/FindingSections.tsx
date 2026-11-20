// Findings grouped Must fix / Should fix / Consider / Good, near-duplicates folded together.

import { useMemo } from 'react';
import type { Finding } from '../../types';
import { severitySections } from './reviewModel';
import { FindingItem } from './FindingItem';

export function FindingSections({ findings }: { findings: readonly Finding[] }) {
  const sections = useMemo(() => severitySections(findings), [findings]);
  if (sections.length === 0) return null;
  return (
    <div className="rv-findings">
      {sections.map((section) => (
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
