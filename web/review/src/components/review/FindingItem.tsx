// One finding cluster: severity rail, location · category · confidence, title, excerpt,
// agents, and the folded "+N similar" findings from other reviewers.

import { useState, type KeyboardEvent, type MouseEvent } from 'react';
import { useReview } from '../../state/StoreProvider';
import type { FindingCluster } from '../../lib/findings';
import type { Finding } from '../../types';
import { AgentTag } from '../shared/bits';
import { useScrollIntoView } from '../sidebar/useScrollIntoView';
import { clusterAgents, firstSentence, formatConfidence, locationLabel } from './reviewModel';

/** Findings that arrived within this window animate in */
const FRESH_MS = 2500;

export function FindingItem({ cluster }: { cluster: FindingCluster }) {
  const { lead, similar } = cluster;
  const selectedId = useReview((state) => state.selection.findingId);
  const reveal = useReview((state) => state.selection.reveal);
  const arrivedAt = useReview((state) => state.arrivedAt[lead.id] ?? 0);
  const openFinding = useReview((state) => state.openFinding);
  const [fresh] = useState(() => Date.now() - arrivedAt < FRESH_MS);
  const [showSimilar, setShowSimilar] = useState(false);
  const isSelected = selectedId === lead.id || similar.some((finding) => finding.id === selectedId);
  const itemRef = useScrollIntoView<HTMLElement>(isSelected, reveal);

  const toggleSimilar = (event: MouseEvent) => {
    event.stopPropagation();
    setShowSimilar((open) => !open);
  };

  return (
    <article ref={itemRef} className={`rv-item sev-${lead.severity}${isSelected ? ' rv-selected' : ''}${fresh ? ' rv-fresh' : ''}`}
      tabIndex={0} aria-current={isSelected || undefined}
      onClick={() => openFinding(lead.id)} onKeyDown={(event) => activateOnEnter(event, () => openFinding(lead.id))}>
      <FindingMeta finding={lead} />
      <div className="rv-item-title">{lead.title}</div>
      {lead.body && <div className="rv-item-body">{firstSentence(lead.body)}</div>}
      <div className="rv-item-foot">
        {clusterAgents(cluster).map((agent) => <AgentTag key={agent} agent={agent} />)}
        {similar.length > 0 && (
          <button type="button" className="rv-similar-toggle" aria-expanded={showSimilar} onClick={toggleSimilar}>
            +{similar.length} similar
          </button>
        )}
      </div>
      {showSimilar && <SimilarList findings={similar} selectedId={selectedId} onOpen={openFinding} />}
    </article>
  );
}

function FindingMeta({ finding }: { finding: Finding }) {
  const confidence = formatConfidence(finding.confidence);
  return (
    <div className="rv-item-meta">
      <span className="mono rv-loc" title={finding.line ? `${finding.file}:${finding.line}` : finding.file}>{locationLabel(finding)}</span>
      <span>{finding.category}</span>
      {confidence && <span className="mono" title="Reviewer confidence">{confidence}</span>}
    </div>
  );
}

function SimilarList({ findings, selectedId, onOpen }: { findings: Finding[]; selectedId: string | null; onOpen(id: string): void }) {
  return (
    <ul className="rv-similar">
      {findings.map((finding) => (
        <li key={finding.id} className={finding.id === selectedId ? 'rv-selected' : undefined}
          onClick={(event) => { event.stopPropagation(); onOpen(finding.id); }}>
          <span className={`sev-dot sev-${finding.severity}`} aria-hidden="true" />
          <span className="rv-similar-title">{finding.title}</span>
          <AgentTag agent={finding.source} />
        </li>
      ))}
    </ul>
  );
}

function activateOnEnter(event: KeyboardEvent, action: () => void) {
  if (event.key === 'Enter' && event.target === event.currentTarget) {
    event.preventDefault();
    action();
  }
}
