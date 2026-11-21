// "2 must fix · 6 should fix · 3 consider": each chip opens the first finding of its severity.

import { useFindings, useReviewStore, useSidebarGroups } from '../../state/StoreProvider';
import { orderedFindings } from '../../state/store';
import { sidebarFiles } from '../../lib/grouping';
import { severityCounts, SEVERITY_LABEL } from '../../lib/findings';
import type { Severity } from '../../types';

const CHIP_SEVERITIES: readonly Severity[] = ['critical', 'warning', 'suggestion'];

export function SeverityChips() {
  const store = useReviewStore();
  const findings = useFindings();
  const groups = useSidebarGroups();
  const counts = new Map(severityCounts(findings));

  const openFirst = (severity: Severity) => {
    const first = orderedFindings(findings, sidebarFiles(groups)).find((finding) => finding.severity === severity);
    if (!first) return;
    const state = store.getState();
    state.setTab('review');
    state.openFinding(first.id);
  };

  if (findings.length === 0) return null;
  return (
    <div className="hd-chips" role="group" aria-label="Findings by severity">
      {CHIP_SEVERITIES.map((severity) => {
        const count = counts.get(severity) ?? 0;
        return (
          <button key={severity} type="button" disabled={count === 0}
            className={`hd-chip sev-${severity}${count === 0 ? ' hd-chip-zero' : ''}`}
            title={count ? `Open the first “${SEVERITY_LABEL[severity].toLowerCase()}” finding` : undefined}
            onClick={() => openFirst(severity)}>
            <span className="sev-dot" />
            <span className="hd-chip-count mono">{count}</span>
            <span className="hd-chip-label">{SEVERITY_LABEL[severity].toLowerCase()}</span>
          </button>
        );
      })}
    </div>
  );
}
