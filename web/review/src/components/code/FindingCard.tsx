// Inline finding card shown under its line (or in the file notes at the top).

import { memo, useState } from 'react';
import type { Finding } from '../../types';
import { formatLineRef, SEVERITY_LABEL } from '../../lib/findings';
import { highlightLine } from '../../lib/highlight';
import { AgentTag, ConfidenceWord, PassTag } from '../shared/bits';
import { useCompareTag } from '../../hooks/useCompare';
import { Markdown } from '../shared/Markdown';
import type { TriageEntry } from '../../state/triage';
import { DismissedLine, TriageActions, VerifyStep } from '../shared/Triage';
import { useIsCursor } from './cursor';

/** Cards that arrived this recently animate in */
const ARRIVAL_WINDOW_MS = 2500;

export const findingKey = (id: string) => `f:${id}`;

export interface FindingCardProps {
  finding: Finding;
  triage: TriageEntry | undefined;
  arrivedAt: number | undefined;
  language: string;
  /** Set when the finding's line is not shown; renders a "Show line N" control */
  onShowLine?: (finding: Finding) => void;
  onActivate: (key: string) => void;
}

function Suggestion({ text, language }: { text: string; language: string }) {
  const lines = text.replace(/\n$/, '').split('\n');
  return (
    <div className="cv-suggestion">
      <div className="cv-suggestion-title">Suggested change</div>
      <pre className="cv-suggestion-code">
        {lines.map((line, index) => (
          <div key={index} className="cv-suggestion-line">
            <span className="cv-mark">+</span>
            <span>{highlightLine(line, language).map((token, tokenIndex) => (
              <span key={tokenIndex} className={token.kind === 'plain' ? undefined : `cv-tok-${token.kind}`}>{token.text}</span>
            ))}</span>
          </div>
        ))}
      </pre>
    </div>
  );
}

function CardFooter({ finding, onShowLine }: Pick<FindingCardProps, 'finding' | 'onShowLine'>) {
  return (
    <div className="cv-card-foot">
      <TriageActions finding={finding} surface="code" />
      {onShowLine && finding.line ? (
        <button type="button" className="btn btn-outline cv-card-show" onClick={(event) => { event.stopPropagation(); onShowLine(finding); }}>
          Show line {finding.line}
        </button>
      ) : null}
      <span className="cv-card-spacer" />
      <ConfidenceWord confidence={finding.confidence} />
      <AgentTag agent={finding.source} />
    </div>
  );
}

function CardPassTag({ id }: { id: string }) {
  return <PassTag kind={useCompareTag(id)} />;
}

export const FindingCard = memo(function FindingCard(props: FindingCardProps) {
  const { finding, triage, arrivedAt, language, onActivate } = props;
  const key = findingKey(finding.id);
  const isCursor = useIsCursor(key);
  const [arriving] = useState(() => arrivedAt !== undefined && Date.now() - arrivedAt < ARRIVAL_WINDOW_MS);
  const [expanded, setExpanded] = useState(false);
  const dismissed = triage?.state === 'dismissed';
  const className = [
    'cv-card', `cv-card-${finding.severity}`, isCursor ? 'cv-cursor' : '', arriving ? 'cv-card-arrive' : '',
    dismissed ? 'cv-card-dismissed' : '', triage?.state === 'accepted' ? 'cv-card-accepted' : '',
  ].filter(Boolean).join(' ');

  if (dismissed && !expanded) {
    return (
      <article className={className} data-key={key} onClick={() => onActivate(key)}>
        <div className="cv-card-inner">
          <DismissedLine finding={finding} entry={triage} onOpen={() => setExpanded(true)} />
        </div>
      </article>
    );
  }
  return (
    <article className={className} data-key={key} onClick={() => onActivate(key)}>
      <div className="cv-card-inner">
        <div className="cv-card-head">
          {triage?.state === 'accepted' ? <span className="cv-card-check" title="Accepted">✓</span> : null}
          <span className={`cv-card-sev sev-${finding.severity}`}>{SEVERITY_LABEL[finding.severity] ?? finding.severity}</span>
          {finding.category ? <span className="cv-card-meta">{finding.category}</span> : null}
          <span className="cv-card-meta mono">{formatLineRef(finding)}</span>
          <CardPassTag id={finding.id} />
          {dismissed ? (
            <button type="button" className="tr-link cv-card-collapse" onClick={(event) => { event.stopPropagation(); setExpanded(false); }}>Collapse</button>
          ) : null}
        </div>
        <h4 className="cv-card-title">{finding.title || '(untitled finding)'}</h4>
        <Markdown source={finding.body} className="cv-card-body" />
        {finding.suggestion ? <Suggestion text={finding.suggestion} language={language} /> : null}
        <VerifyStep text={finding.verify} />
        <CardFooter {...props} />
      </div>
    </article>
  );
});
