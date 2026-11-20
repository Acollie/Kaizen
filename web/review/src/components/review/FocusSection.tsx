// Kaizen focus: the functions and non-code files Kaizen's metrics say to review first.

import { useMemo } from 'react';
import { useReview } from '../../state/StoreProvider';
import { qualifiedName, splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import type { FileDiff, FunctionRisk } from '../../types';
import { Disclosure } from './Disclosure';
import { focusFunctions, riskyFiles } from './reviewModel';

export function FocusSection() {
  const functions = useReview((state) => state.context.functions);
  const files = useReview((state) => state.context.files);
  const functionsByFile = useReview((state) => state.index.functionsByFile);
  const focus = useMemo(() => focusFunctions(functions ?? []), [functions]);
  const risky = useMemo(() => riskyFiles(files ?? [], functionsByFile), [files, functionsByFile]);
  if (focus.length === 0 && risky.length === 0) return null;
  return (
    <Disclosure title="Kaizen focus" count={focus.length + risky.length} hint="riskiest by metrics" defaultOpen>
      <ul className="rv-focus">
        {focus.map((fn) => <FocusFunction key={fn.id} fn={fn} />)}
        {risky.map((file) => <RiskyFile key={file.path} file={file} />)}
      </ul>
    </Disclosure>
  );
}

function FocusFunction({ fn }: { fn: FunctionRisk }) {
  const selectFunction = useReview((state) => state.selectFunction);
  const isActive = useReview((state) => state.selection.functionId === fn.id);
  const viewed = useReview((state) => Boolean(state.viewed.functions[fn.id]));
  const reason = (fn.risk_reasons ?? []).find(Boolean) ?? '';
  return (
    <li className={`rv-focus-row${isActive ? ' rv-selected' : ''}`} title={fn.id} onClick={() => selectFunction(fn.id)}>
      <RiskNumber score={fn.risk_score} />
      <div className="rv-focus-main">
        <div className="rv-focus-name mono">{qualifiedName(fn)}<span className="rv-focus-file">{splitPath(fn.file).base}</span>{viewed && <ViewedMark />}</div>
        {reason && <div className="rv-focus-reason">{reason}</div>}
      </div>
    </li>
  );
}

function RiskyFile({ file }: { file: FileDiff }) {
  const openFile = useReview((state) => state.openFile);
  const isActive = useReview((state) => state.selection.file === file.path && !state.selection.functionId);
  const viewed = useReview((state) => Boolean(state.viewed.files[file.path]));
  return (
    <li className={`rv-focus-row${isActive ? ' rv-selected' : ''}`} title={file.path} onClick={() => openFile(file.path)}>
      <RiskNumber score={file.risk_score} />
      <div className="rv-focus-main">
        <div className="rv-focus-name mono">{file.path}{viewed && <ViewedMark />}</div>
        <div className="rv-focus-reason">{(file.risk_reasons ?? []).filter(Boolean).join(' · ')}</div>
      </div>
    </li>
  );
}

function RiskNumber({ score }: { score: number }) {
  return <span className="rv-risk mono" style={{ color: riskColor(score) }} title={`Kaizen risk ${Math.round(score)}`}>{Math.round(score)}</span>;
}

function ViewedMark() {
  return <span className="rv-focus-viewed" title="You've reviewed this: it was on screen for 2 seconds. Only your review counts as verified." aria-label="you reviewed">✓</span>;
}
