// Memoised row components for the code view. Each row re-renders only when its own
// props change or when it gains/loses the cursor (see cursor.ts).

import { memo, useMemo, type CSSProperties, type MouseEvent } from 'react';
import { isCommentableKind, REMOVED_LINE_HINT } from '../../state/comments';
import type { FunctionRisk } from '../../types';
import { highlightLine } from '../../lib/highlight';
import { riskColor } from '../../lib/risk';
import { qualifiedName } from '../../lib/format';
import { useIsCursor } from './cursor';
import { EXPAND_STEP, type ExpandDirection, type GapRow, type HunkRow, type LineRow } from './rows';

export type ActivateHandler = (key: string) => void;

const MARKS = { added: '+', removed: '−', context: '' } as const;

function Code({ text, language }: { text: string; language: string }) {
  const tokens = useMemo(() => highlightLine(text, language), [text, language]);
  return (
    <span className="cv-code">
      {tokens.map((token, index) => (token.kind === 'plain'
        ? token.text
        : <span key={index} className={`cv-tok-${token.kind}`}>{token.text}</span>))}
    </span>
  );
}

/** Risk (and ★ for focus functions) at a function's first row; the name too when the signature is not on that row */
function FunctionLabel({ fn, showName, onSelect }: { fn: FunctionRisk; showName: boolean; onSelect: (id: string) => void }) {
  return (
    <button
      type="button" className="cv-fnlabel" title={`Select ${qualifiedName(fn)}`}
      onClick={(event) => { event.stopPropagation(); onSelect(fn.id); }}
    >
      {showName ? <span className="cv-fnlabel-name">{qualifiedName(fn)}</span> : null}
      <span className="cv-fnlabel-risk" style={{ color: riskColor(fn.risk_score) }}>risk {Math.round(fn.risk_score)}</span>
      {fn.focus ? <span className="cv-fnlabel-focus" title="Focus function">★</span> : null}
    </button>
  );
}

export interface LineRowProps {
  row: LineRow;
  language: string;
  /** Changed function containing the line (draws the risk rail) */
  fn: FunctionRisk | undefined;
  /** Function whose label starts on this row */
  label: FunctionRisk | undefined;
  callSite: boolean;
  /** Set when the file is in the PR: the "+" that starts a comment on this line */
  onCommentButton?: CommentButtonHandler;
  /** Line is part of the range of an open comment composer */
  inCommentRange?: boolean;
  onActivate: ActivateHandler;
  onSelectFunction: (id: string) => void;
}

export type CommentButtonHandler = (row: LineRow, event: MouseEvent, phase: 'down' | 'click') => void;

function CommentButton({ row, onComment }: { row: LineRow; onComment: CommentButtonHandler }) {
  if (!isCommentableKind(row.kind)) {
    return (
      <button type="button" className="cv-add cv-add-blocked" aria-disabled="true" tabIndex={-1} title={REMOVED_LINE_HINT}
        aria-label="Can't comment on a removed line" onClick={(event) => { event.stopPropagation(); onComment(row, event, 'click'); }}>+</button>
    );
  }
  return (
    <button type="button" className="cv-add" tabIndex={-1} aria-label={`Comment on line ${row.newLine}`}
      title="Comment on this line (c). Shift-click or drag for a range."
      onMouseDown={(event) => { event.stopPropagation(); onComment(row, event, 'down'); }}
      onClick={(event) => { event.stopPropagation(); onComment(row, event, 'click'); }}>+</button>
  );
}

export const LineRowView = memo(function LineRowView({ row, language, fn, label, callSite, onCommentButton, inCommentRange, onActivate, onSelectFunction }: LineRowProps) {
  const isCursor = useIsCursor(row.key);
  const className = ['cv-row', `cv-${row.kind}`, fn ? 'cv-in-fn' : '', isCursor ? 'cv-cursor' : '', callSite ? 'cv-callsite' : '', inCommentRange ? 'cv-in-range' : '']
    .filter(Boolean).join(' ');
  const style = fn ? ({ '--cv-rail': riskColor(fn.risk_score) } as CSSProperties) : undefined;
  return (
    <div className={className} style={style} data-key={row.key} onClick={() => onActivate(row.key)}>
      <span className="cv-gutter">
        <span className="cv-ln cv-ln-old" aria-hidden="true">{row.oldLine || ''}</span>
        <span className="cv-ln cv-ln-new" aria-hidden="true">{row.newLine || ''}</span>
        <span className="cv-mark" aria-hidden="true">{MARKS[row.kind]}</span>
        {onCommentButton ? <CommentButton row={row} onComment={onCommentButton} /> : null}
      </span>
      <Code text={row.text} language={language} />
      {label ? <FunctionLabel fn={label} showName={row.newLine !== label.start_line} onSelect={onSelectFunction} /> : null}
    </div>
  );
});

export interface HunkRowProps {
  row: HunkRow;
  enclosing: FunctionRisk | undefined;
  onActivate: ActivateHandler;
  onSelectFunction: (id: string) => void;
}

export const HunkRowView = memo(function HunkRowView({ row, enclosing, onActivate, onSelectFunction }: HunkRowProps) {
  const isCursor = useIsCursor(row.key);
  return (
    <div className={`cv-hunk${isCursor ? ' cv-cursor' : ''}`} data-key={row.key} onClick={() => onActivate(row.key)}>
      <span className="cv-sticky cv-hunk-inner">
        <span className="cv-hunk-range">{row.range}</span>
        {enclosing ? (
          <button
            type="button" className="cv-hunk-fn" title="Select function"
            onClick={(event) => { event.stopPropagation(); onSelectFunction(enclosing.id); }}
          >
            {qualifiedName(enclosing)}
          </button>
        ) : <span className="cv-hunk-fn cv-hunk-fn-text">{row.header}</span>}
        {enclosing ? <span className="cv-hunk-risk" style={{ color: riskColor(enclosing.risk_score) }}>{Math.round(enclosing.risk_score)}</span> : null}
      </span>
    </div>
  );
});

export interface GapRowProps {
  row: GapRow;
  loading: boolean;
  error: string | undefined;
  onActivate: ActivateHandler;
  onExpand: (key: string, direction: ExpandDirection) => void;
}

function gapControls(row: GapRow): { direction: ExpandDirection; label: string; title: string }[] {
  const size = row.to - row.from + 1;
  if (size <= EXPAND_STEP) return [{ direction: 'all', label: `Show ${size} line${size === 1 ? '' : 's'}`, title: 'Expand' }];
  const controls: { direction: ExpandDirection; label: string; title: string }[] = [];
  if (row.edge !== 'top') controls.push({ direction: 'below', label: `↓ ${EXPAND_STEP} lines`, title: 'Expand down from the code above (E / zc)' });
  if (row.edge !== 'bottom') controls.push({ direction: 'above', label: `↑ ${EXPAND_STEP} lines`, title: 'Expand up from the code below (e / zo)' });
  controls.push({ direction: 'all', label: `Show all ${size} lines`, title: 'Expand the whole gap (zR expands the file)' });
  return controls;
}

export const GapRowView = memo(function GapRowView({ row, loading, error, onActivate, onExpand }: GapRowProps) {
  const isCursor = useIsCursor(row.key);
  return (
    <div className={`cv-gap cv-gap-${row.edge}${isCursor ? ' cv-cursor' : ''}`} data-key={row.key} onClick={() => onActivate(row.key)}>
      <span className="cv-sticky cv-gap-inner">
        {gapControls(row).map((control) => (
          <button
            key={control.direction} type="button" className="cv-gap-btn" title={control.title} disabled={loading}
            onClick={(event) => { event.stopPropagation(); onActivate(row.key); onExpand(row.key, control.direction); }}
          >
            {control.label}
          </button>
        ))}
        <span className="cv-gap-range">{error ? `Could not load lines: ${error}` : `lines ${row.from}–${row.to}`}</span>
        {loading ? <span className="spinner" aria-label="Loading" /> : null}
      </span>
    </div>
  );
});
