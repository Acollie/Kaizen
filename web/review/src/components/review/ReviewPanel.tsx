// Review panel: the AI's verdict, the reviewer's own progress (triage, what they've
// looked at, "Think first"), their comments, the findings to triage (filtered), the AI
// summary, Kaizen focus and Kaizen concerns.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useAllFindings, useFindings, useReview } from '../../state/StoreProvider';
import { findingsHidden } from '../../state/store';
import {
  defaultFilter, matchesTriageFilter, triageProgress, triageStatus, type TriageFilter, type TriageMap,
} from '../../state/triage';
import type { FindingCluster } from '../../lib/findings';
import { Headline } from './Headline';
import { FindingSections } from './FindingSections';
import { SummarySection } from './SummarySection';
import { FocusSection } from './FocusSection';
import { ConcernsSection } from './ConcernsSection';
import { ThinkFirstNotice, YourReview } from './YourReview';
import { YourComments } from './YourComments';
import './ReviewPanel.css';

export function ReviewPanel() {
  const findings = useFindings();
  const allFindings = useAllFindings();
  const hidden = useReview(findingsHidden);
  const running = useReview((state) => state.status.state !== 'done' && state.status.state !== 'error');
  const hasResult = useReview((state) => state.result !== null);
  const triage = useReview((state) => state.triage);
  const progress = useMemo(() => triageProgress(allFindings, triage), [allFindings, triage]);
  const [chosen, setChosen] = useState<TriageFilter | null>(null);
  const filter = chosen ?? defaultFilter(progress);
  const kept = useKeptWhileFiltering(filter, triage);
  const showCluster = (cluster: FindingCluster) =>
    kept.has(cluster.lead.id) || matchesTriageFilter(triageStatus(triage, cluster.lead.id), filter);

  return (
    <div className="rv">
      <Headline findings={findings} hidden={hidden} />
      <YourReview progress={progress} filter={filter} onFilter={setChosen} hidden={hidden} />
      <YourComments />
      {hidden
        ? <ThinkFirstNotice total={allFindings.length} running={running} />
        : <FindingSections findings={findings} showCluster={showCluster} emptyText={emptyText(filter, progress.total)} />}
      {hasResult && !hidden && <SummarySection />}
      <FocusSection />
      <ConcernsSection />
    </div>
  );
}

function emptyText(filter: TriageFilter, total: number): string {
  if (total === 0) return '';
  if (filter === 'unreviewed') return "Everything is triaged. Check Accepted before you post.";
  return `No ${filter} findings.`;
}

/**
 * Findings triaged while a filter is showing stay in view (collapsed, with Undo) until
 * the filter changes, so accepting or dismissing never makes an item jump away.
 */
function useKeptWhileFiltering(filter: TriageFilter, triage: TriageMap): ReadonlySet<string> {
  const [kept, setKept] = useState<ReadonlySet<string>>(new Set());
  const previous = useRef(triage);
  useEffect(() => setKept(new Set()), [filter]);
  useEffect(() => {
    const before = previous.current;
    previous.current = triage;
    const changed = [...new Set([...Object.keys(before), ...Object.keys(triage)])]
      .filter((id) => before[id]?.state !== triage[id]?.state);
    if (changed.length) setKept((current) => new Set([...current, ...changed]));
  }, [triage]);
  return kept;
}
