// Blind spots: the changed code the AI never opened. Code it did open is "looked at",
// never "examined" or "verified": the human checks both. Skipped Kaizen focus functions
// get a prominent warning; they are the riskiest.

import { useMemo, useState } from 'react';
import { useReview } from '../../state/StoreProvider';
import { useAiCoverage, type AiCoverage } from '../../state/aiCoverage';
import { plural, qualifiedName, splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { AI_LOOKED_HINT } from '../../lib/coverageCopy';
import type { FileDiff, FunctionRisk } from '../../types';
import { Disclosure } from './Disclosure';
import './BlindSpots.css';

const COLLAPSED_ROWS = 8;

export interface BlindSpotsView {
  /** Changed functions the AI never opened, riskiest first */
  skippedFunctions: FunctionRisk[];
  /** Skipped functions Kaizen marked focus */
  skippedFocus: FunctionRisk[];
  /** PR files never opened, file-level risk first, then by risk */
  skippedFiles: FileDiff[];
  summary: string;
}

/** True when Kaizen flags the file itself (workflows, scripts, config) */
export function hasFileRisk(file: FileDiff): boolean {
  return (file.risk_reasons ?? []).some(Boolean);
}

export function blindSpotsView(coverage: AiCoverage, functions: readonly FunctionRisk[], files: readonly FileDiff[]): BlindSpotsView {
  const byRisk = (left: { risk_score: number }, right: { risk_score: number }) => right.risk_score - left.risk_score;
  const skippedFunctions = functions.filter((fn) => !coverage.examinedFunctions.has(fn.id)).sort(byRisk);
  const skippedFocus = skippedFunctions.filter((fn) => fn.focus);
  const skippedFiles = files
    .filter((file) => !coverage.examinedFiles.has(file.path))
    .sort((left, right) => Number(hasFileRisk(right)) - Number(hasFileRisk(left)) || byRisk(left, right));
  return { skippedFunctions, skippedFocus, skippedFiles, summary: coverageSummary(coverage, functions, files) };
}

/** "AI looked at 11 of 32 changed functions (3/3 focus) and 4 of 7 files", "… so far" while running */
export function coverageSummary(coverage: AiCoverage, functions: readonly FunctionRisk[], files: readonly FileDiff[]): string {
  const opened = functions.filter((fn) => coverage.examinedFunctions.has(fn.id)).length;
  const focus = functions.filter((fn) => fn.focus);
  const focusOpened = focus.filter((fn) => coverage.examinedFunctions.has(fn.id)).length;
  const filesOpened = files.filter((file) => coverage.examinedFiles.has(file.path)).length;
  const focusPart = focus.length > 0 ? ` (${focusOpened}/${focus.length} focus)` : '';
  const functionPart = `${opened} of ${functions.length} changed ${functions.length === 1 ? 'function' : 'functions'}${focusPart}`;
  const filePart = `${filesOpened} of ${files.length} ${files.length === 1 ? 'file' : 'files'}`;
  const verb = coverage.complete ? 'AI looked at' : 'AI has looked at';
  return `${verb} ${functionPart} and ${filePart}${coverage.complete ? '' : ' so far'}`;
}

export function BlindSpots() {
  const coverage = useAiCoverage();
  const functions = useReview((state) => state.context.functions);
  const files = useReview((state) => state.context.files);
  const view = useMemo(() => blindSpotsView(coverage, functions ?? [], files ?? []), [coverage, functions, files]);

  if (!coverage.known) {
    if (!coverage.complete) return null;
    return (
      <p className="rv-blind-unknown">
        Coverage unknown for this review. Run with <code className="mono">--rerun</code> to record it.
      </p>
    );
  }
  const skippedCount = view.skippedFunctions.length + view.skippedFiles.length;
  return (
    <Disclosure title="What the AI didn't look at" count={skippedCount} hint={coverage.complete ? undefined : 'so far'} defaultOpen>
      <p className="rv-blind-summary" title={AI_LOOKED_HINT}>{view.summary}</p>
      <p className="rv-blind-caveat">{AI_LOOKED_HINT}</p>
      {view.skippedFocus.length > 0 && <FocusWarning skipped={view.skippedFocus} complete={coverage.complete} />}
      {skippedCount === 0 && <p className="rv-blind-empty">The AI looked at every changed function and file. That isn't a review: check the risky code yourself.</p>}
      {view.skippedFunctions.length > 0 && (
        <SkippedList label={`Functions not opened · ${view.skippedFunctions.length}`} items={view.skippedFunctions} render={(fn) => <SkippedFunction key={fn.id} fn={fn} />} />
      )}
      {view.skippedFiles.length > 0 && (
        <SkippedList label={`Files never opened · ${view.skippedFiles.length}`} items={view.skippedFiles} render={(file) => <SkippedFile key={file.path} file={file} />} />
      )}
    </Disclosure>
  );
}

function FocusWarning({ skipped, complete }: { skipped: FunctionRisk[]; complete: boolean }) {
  const selectFunction = useReview((state) => state.selectFunction);
  const count = plural(skipped.length, 'Kaizen focus function');
  return (
    <div className="rv-blind-warning">
      <div className="rv-blind-warning-title">
        {complete ? `The AI never opened ${count}` : `The AI hasn't opened ${count} yet`}
      </div>
      <div className="rv-blind-warning-body">These are the riskiest changes by Kaizen's metrics. Review them yourself.</div>
      <ul className="rv-blind-warning-list">
        {skipped.map((fn) => (
          <li key={fn.id}>
            <button type="button" className="rv-blind-link mono" title={fn.id} onClick={() => selectFunction(fn.id)}>
              {qualifiedName(fn)}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function SkippedList<T>({ label, items, render }: { label: string; items: T[]; render: (item: T) => JSX.Element }) {
  const [expanded, setExpanded] = useState(false);
  const shown = expanded ? items : items.slice(0, COLLAPSED_ROWS);
  const hidden = items.length - shown.length;
  return (
    <div className="rv-blind-group">
      <div className="rv-blind-label">{label}</div>
      <ul className="rv-focus">{shown.map(render)}</ul>
      {hidden > 0 && (
        <button type="button" className="rv-blind-more" onClick={() => setExpanded(true)}>Show {hidden} more</button>
      )}
    </div>
  );
}

function SkippedFunction({ fn }: { fn: FunctionRisk }) {
  const selectFunction = useReview((state) => state.selectFunction);
  const isActive = useReview((state) => state.selection.functionId === fn.id);
  const reason = (fn.risk_reasons ?? []).find(Boolean) ?? '';
  return (
    <li className={`rv-focus-row${isActive ? ' rv-selected' : ''}${fn.focus ? ' rv-blind-focus' : ''}`} title={fn.id} onClick={() => selectFunction(fn.id)}>
      <RiskNumber score={fn.risk_score} />
      <div className="rv-focus-main">
        <div className="rv-focus-name mono">
          {qualifiedName(fn)}
          <span className="rv-focus-file">{splitPath(fn.file).base}</span>
          {fn.focus && <span className="rv-blind-tag">focus</span>}
        </div>
        {reason && <div className="rv-focus-reason">{reason}</div>}
      </div>
    </li>
  );
}

function SkippedFile({ file }: { file: FileDiff }) {
  const openFile = useReview((state) => state.openFile);
  const isActive = useReview((state) => state.selection.file === file.path && !state.selection.functionId);
  const reasons = (file.risk_reasons ?? []).filter(Boolean).join(' · ');
  return (
    <li className={`rv-focus-row${isActive ? ' rv-selected' : ''}`} title={file.path} onClick={() => openFile(file.path)}>
      <RiskNumber score={file.risk_score} />
      <div className="rv-focus-main">
        <div className="rv-focus-name mono">
          {file.path}
          {hasFileRisk(file) && <span className="rv-blind-tag">file risk</span>}
        </div>
        {reasons && <div className="rv-focus-reason">{reasons}</div>}
      </div>
    </li>
  );
}

function RiskNumber({ score }: { score: number }) {
  return <span className="rv-risk mono" style={{ color: riskColor(score) }} title={`Kaizen risk ${Math.round(score)}`}>{Math.round(score)}</span>;
}
