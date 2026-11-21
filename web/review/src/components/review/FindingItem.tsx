// One finding cluster: severity rail, severity · location · category · confidence word, title, excerpt,
// agents, and the folded "+N similar" findings from other reviewers.

import { useState, type KeyboardEvent, type MouseEvent } from 'react';
import { useReview } from '../../state/StoreProvider';
import type { FindingCluster } from '../../lib/findings';
import type { Finding } from '../../types';
import { AgentTag, ConfidenceWord, PassTag, SeverityChip } from '../shared/bits';
import { useCompareTag } from '../../hooks/useCompare';
import { DismissedLine, TriageActions, VerifyStep } from '../shared/Triage';
import { triageStatus } from '../../state/triage';
import { useScrollIntoView } from '../sidebar/useScrollIntoView';
import { clusterAgents, firstSentence, locationLabel } from './reviewModel';

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
  const entry = useReview((state) => state.triage[lead.id]);
  const isSelected = selectedId === lead.id || similar.some((finding) => finding.id === selectedId);
  const itemRef = useScrollIntoView<HTMLElement>(isSelected, reveal);

  const toggleSimilar = (event: MouseEvent) => {
    event.stopPropagation();
    setShowSimilar((open) => !open);
  };

  const selectedClass = isSelected ? ' rv-selected' : '';
  if (entry?.state === 'dismissed') {
    return (
      <article ref={itemRef} className={`rv-item rv-item-dismissed sev-${lead.severity}${selectedClass}`} tabIndex={0}
        aria-current={isSelected || undefined} onKeyDown={(event) => activateOnEnter(event, () => openFinding(lead.id))}>
        <DismissedLine finding={lead} entry={entry} onOpen={() => openFinding(lead.id)} />
      </article>
    );
  }

  return (
    <article ref={itemRef} className={`rv-item sev-${lead.severity}${selectedClass}${fresh ? ' rv-fresh' : ''}${entry ? ' rv-item-accepted' : ''}`}
      tabIndex={0} aria-current={isSelected || undefined}
      onClick={() => openFinding(lead.id)} onKeyDown={(event) => activateOnEnter(event, () => openFinding(lead.id))}>
      <FindingMeta finding={lead} />
      <div className="rv-item-title">
        {entry?.state === 'accepted' && <span className="rv-check" title="Accepted" aria-label="Accepted">✓ </span>}
        {lead.title}
      </div>
      {lead.body && <div className="rv-item-body">{firstSentence(lead.body)}</div>}
      <VerifyStep text={lead.verify} />
      <div className="rv-item-triage"><TriageActions finding={lead} surface="panel" /></div>
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
  const tag = useCompareTag(finding.id);
  return (
    <div className="rv-item-meta">
      <SeverityChip severity={finding.severity} />
      <span className="mono rv-loc" title={finding.line ? `${finding.file}:${finding.line}` : finding.file}>{locationLabel(finding)}</span>
      <span>{finding.category}</span>
      <ConfidenceWord confidence={finding.confidence} />
      <PassTag kind={tag} />
    </div>
  );
}

const DUPLICATE_REASON = 'Duplicate';

function SimilarList({ findings, selectedId, onOpen }: { findings: Finding[]; selectedId: string | null; onOpen(id: string): void }) {
  const triage = useReview((state) => state.triage);
  const dismissFinding = useReview((state) => state.dismissFinding);
  const untriaged = findings.filter((finding) => triageStatus(triage, finding.id) === 'unreviewed');
  const dismissAll = (event: MouseEvent) => {
    event.stopPropagation();
    untriaged.forEach((finding) => dismissFinding(finding.id, DUPLICATE_REASON));
  };
  return (
    <ul className="rv-similar">
      {findings.map((finding) => {
        const status = triageStatus(triage, finding.id);
        return (
          <li key={finding.id} className={`${finding.id === selectedId ? 'rv-selected' : ''} rv-similar-${status}`}
            onClick={(event) => { event.stopPropagation(); onOpen(finding.id); }}>
            <span className={`sev-dot sev-${finding.severity}`} aria-hidden="true" />
            <span className="rv-similar-title">{finding.title}</span>
            {status !== 'unreviewed' && <span className="rv-similar-status">{status === 'accepted' ? '✓ accepted' : 'dismissed'}</span>}
            <AgentTag agent={finding.source} />
          </li>
        );
      })}
      {untriaged.length > 0 && (
        <li className="rv-similar-action">
          <button type="button" className="tr-link" onClick={dismissAll} title="Same problem in other words: dismiss them as duplicates">
            Dismiss {untriaged.length === 1 ? 'it' : `these ${untriaged.length}`} as duplicates
          </button>
        </li>
      )}
    </ul>
  );
}

function activateOnEnter(event: KeyboardEvent, action: () => void) {
  if (event.key === 'Enter' && event.target === event.currentTarget) {
    event.preventDefault();
    action();
  }
}
