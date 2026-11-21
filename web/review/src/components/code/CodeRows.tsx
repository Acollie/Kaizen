// The list of rows for the current file, with finding cards slotted under their lines.

import { Fragment, memo, useCallback, useMemo, useRef } from 'react';
import type { Finding, FunctionRisk } from '../../types';
import { useReview, useReviewStore } from '../../state/StoreProvider';
import { isUserComment, normalizeRange, REMOVED_LINE_HINT, type CommentTarget } from '../../state/comments';
import { FindingCard } from './FindingCard';
import { CommentCard, CommentComposer } from './CommentCard';
import { GapRowView, HunkRowView, LineRowView, type ActivateHandler, type CommentButtonHandler } from './DiffRows';
import type { FindingPlacement, FunctionRails } from './placement';
import type { ExpandDirection, HunkRow, Row } from './rows';
import type { FileModel } from './useFileModel';

interface SlotProps {
  findings: readonly Finding[];
  language: string;
  activate: ActivateHandler;
  onShowLine?: (finding: Finding) => void;
}

/** Finding cards for one line (or the file notes); reads include/arrival state itself */
export const FindingSlot = memo(function FindingSlot({ findings, language, activate, onShowLine }: SlotProps) {
  const triage = useReview((state) => state.triage);
  const arrivedAt = useReview((state) => state.arrivedAt);
  const comments = useReview((state) => state.comments);
  return (
    <div className="cv-slot">
      {findings.map((finding) => {
        const comment = isUserComment(finding) ? comments[finding.id] : undefined;
        if (comment) return <CommentCard key={finding.id} comment={comment} onActivate={activate} />;
        return (
          <FindingCard
            key={finding.id} finding={finding} triage={triage[finding.id]} arrivedAt={arrivedAt[finding.id]}
            language={language} onActivate={activate} onShowLine={onShowLine}
          />
        );
      })}
    </div>
  );
});

/** A new comment's composer, shown under the last line of its range (or in the file notes) */
export function ComposerSlot({ target }: { target: CommentTarget }) {
  const key = `${target.file}:${target.line}-${target.end_line ?? ''}`;
  return (
    <div className="cv-card cv-card-user cv-card-comment">
      <div className="cv-card-inner"><CommentComposer key={key} target={target} /></div>
    </div>
  );
}

function BlockedCommentHint() {
  const clear = useReview((state) => state.setBlockedComment);
  return (
    <div className="cv-card cv-card-hint" role="note">
      <div className="cv-card-inner cv-hint">
        <span>{REMOVED_LINE_HINT}</span>
        <button type="button" className="tr-link" onClick={() => clear(null)}>OK</button>
      </div>
    </div>
  );
}

/** Click the gutter "+" for one line, shift-click to extend the open composer, or drag across lines */
function useCommentGestures(path: string | null): CommentButtonHandler {
  const store = useReviewStore();
  const dragStart = useRef<number | null>(null);

  return useCallback((row, event, phase) => {
    const state = store.getState();
    if (!path) return;
    if (row.kind === 'removed') {
      state.setBlockedComment(row.key);
      return;
    }
    if (phase === 'down') {
      dragStart.current = row.newLine;
      document.addEventListener('mouseup', (upEvent) => {
        const start = dragStart.current;
        dragStart.current = null;
        const end = lineUnderPointer(upEvent.target);
        if (start && end && end !== start) store.getState().openComposer({ file: path, ...normalizeRange(start, end) });
      }, { once: true });
      return;
    }
    const open = state.ui.composer;
    if (event.shiftKey && open && open.file === path && !open.editingId && open.line) {
      const lines = [open.line, open.end_line ?? open.line, row.newLine];
      state.openComposer({ file: path, ...normalizeRange(Math.min(...lines), Math.max(...lines)) });
      return;
    }
    state.openComposer({ file: path, line: row.newLine });
  }, [path, store]);
}

function lineUnderPointer(target: EventTarget | null): number | null {
  if (!(target instanceof Element)) return null;
  const key = target.closest('[data-key]')?.getAttribute('data-key') ?? '';
  return key.startsWith('n') ? Number(key.slice(1)) || null : null;
}

/** For each hunk, the riskiest changed function it touches */
function enclosingFunctions(rows: readonly Row[], functions: readonly FunctionRisk[]): Map<string, FunctionRisk> {
  const result = new Map<string, FunctionRisk>();
  for (const row of rows) {
    if (row.type !== 'hunk') continue;
    const best = functions
      .filter((fn) => fn.start_line <= row.end && fn.end_line >= row.begin)
      .sort((left, right) => right.risk_score - left.risk_score)[0];
    if (best) result.set(row.key, best);
  }
  return result;
}

export interface CodeRowsProps {
  model: FileModel;
  language: string;
  placement: FindingPlacement;
  rails: FunctionRails;
  functions: readonly FunctionRisk[] | undefined;
  callSiteLine: number | null;
  activate: ActivateHandler;
  onExpand: (key: string, direction: ExpandDirection) => void;
}

export const CodeRows = memo(function CodeRows({ model, language, placement, rails, functions, callSiteLine, activate, onExpand }: CodeRowsProps) {
  const store = useReviewStore();
  const selectFunction = useCallback((id: string) => store.getState().selectFunction(id), [store]);
  const composer = useReview((state) => (state.ui.composer?.file === model.path && !state.ui.composer?.editingId ? state.ui.composer : null));
  const blockedKey = useReview((state) => state.ui.blockedCommentKey);
  const onCommentButton = useCommentGestures(model.plain ? null : model.path);
  const composerAnchor = composer && composer.line ? (composer.end_line || composer.line) : null;
  const enclosing = useMemo(() => enclosingFunctions(model.rows, functions ?? []), [model.rows, functions]);

  const renderRow = (row: Row) => {
    if (row.type === 'hunk') {
      return <HunkRowView key={row.key} row={row as HunkRow} enclosing={enclosing.get(row.key)} onActivate={activate} onSelectFunction={selectFunction} />;
    }
    if (row.type === 'gap') {
      return (
        <GapRowView
          key={row.key} row={row} loading={model.loadingGaps.has(row.key)} error={model.gapErrors[row.key]}
          onActivate={activate} onExpand={onExpand}
        />
      );
    }
    const findings = row.newLine && row.kind !== 'removed' ? placement.byLine.get(row.newLine) : undefined;
    const onNewSide = row.kind !== 'removed' && row.newLine > 0;
    const inRange = Boolean(composer && onNewSide && row.newLine >= composer.line && row.newLine <= (composer.end_line || composer.line));
    return (
      <Fragment key={row.key}>
        <LineRowView
          row={row} language={language} fn={rails.byRow.get(row.key)} label={rails.labels.get(row.key)}
          callSite={callSiteLine !== null && row.newLine === callSiteLine}
          onCommentButton={model.plain ? undefined : onCommentButton} inCommentRange={inRange}
          onActivate={activate} onSelectFunction={selectFunction}
        />
        {findings ? <FindingSlot findings={findings} language={language} activate={activate} /> : null}
        {composer && onNewSide && composerAnchor === row.newLine ? <ComposerSlot target={composer} /> : null}
        {blockedKey === row.key ? <BlockedCommentHint /> : null}
      </Fragment>
    );
  };

  return <div className="cv-lines" role="table" aria-label={model.path ?? 'code'}>{model.rows.map(renderRow)}</div>;
});
