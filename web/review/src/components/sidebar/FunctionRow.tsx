// One changed function: focus star, qualified name, finding counts and a thin risk bar.

import { useReview } from '../../state/StoreProvider';
import { qualifiedName } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { findingBelongsTo } from '../../lib/findings';
import { SeverityCounts } from '../shared/bits';
import type { Finding, FunctionRisk } from '../../types';
import { useScrollIntoView } from './useScrollIntoView';
import { splitForMiddleTruncation } from './sidebarModel';

export function FunctionRow({ fn, fileFindings }: { fn: FunctionRisk; fileFindings: readonly Finding[] }) {
  const isActive = useReview((state) => state.selection.functionId === fn.id);
  const reveal = useReview((state) => state.selection.reveal);
  const selectFunction = useReview((state) => state.selectFunction);
  const rowRef = useScrollIntoView<HTMLDivElement>(isActive, reveal);
  const findings = fileFindings.filter((finding) => findingBelongsTo(finding, fn));
  const name = qualifiedName(fn);
  const score = Math.round(fn.risk_score);
  const tooltip = [fn.id, ...(fn.risk_reasons ?? []).filter(Boolean)].join('\n');

  return (
    <div ref={rowRef} role="treeitem" aria-selected={isActive}
      className={`sb-row sb-fn-row${isActive ? ' sb-active' : ''}`} title={tooltip}
      onClick={() => selectFunction(fn.id)}>
      <span className={`sb-star${fn.focus ? ' sb-star-on' : ''}`} aria-label={fn.focus ? 'Kaizen focus' : undefined}>{fn.focus ? '★' : ''}</span>
      <MiddleTruncated text={name} />
      {fn.is_new && <span className="sb-new" title="New function" aria-label="new" />}
      <span className="sb-counts"><SeverityCounts findings={findings} /></span>
      <span className="sb-fn-risk" title={`Kaizen risk ${score}`}>
        <span className="sb-bar"><span className="sb-bar-fill" style={{ width: `${Math.max(2, Math.min(100, fn.risk_score))}%`, background: riskColor(fn.risk_score) }} /></span>
        <span className="sb-risk mono" style={{ color: riskColor(fn.risk_score) }}>{score}</span>
      </span>
    </div>
  );
}

/** CSS middle truncation: the head shrinks with an ellipsis, the tail always shows */
function MiddleTruncated({ text }: { text: string }) {
  const { head, tail } = splitForMiddleTruncation(text);
  return (
    <span className="sb-fn-name mono">
      <span className="sb-fn-head">{head}</span>
      {tail && <span className="sb-fn-tail">{tail}</span>}
    </span>
  );
}
