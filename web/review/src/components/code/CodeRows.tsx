// The list of rows for the current file, with finding cards slotted under their lines.

import { Fragment, memo, useCallback, useMemo } from 'react';
import type { Finding, FunctionRisk } from '../../types';
import { useReview, useReviewStore } from '../../state/StoreProvider';
import { FindingCard } from './FindingCard';
import { GapRowView, HunkRowView, LineRowView, type ActivateHandler } from './DiffRows';
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
  const included = useReview((state) => state.included);
  const arrivedAt = useReview((state) => state.arrivedAt);
  const setIncluded = useReview((state) => state.setIncluded);
  return (
    <div className="cv-slot">
      {findings.map((finding) => (
        <FindingCard
          key={finding.id} finding={finding} included={Boolean(included[finding.id])} arrivedAt={arrivedAt[finding.id]}
          language={language} onActivate={activate} onToggleIncluded={setIncluded} onShowLine={onShowLine}
        />
      ))}
    </div>
  );
});

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
    return (
      <Fragment key={row.key}>
        <LineRowView
          row={row} language={language} fn={rails.byRow.get(row.key)} label={rails.labels.get(row.key)}
          callSite={callSiteLine !== null && row.newLine === callSiteLine}
          onActivate={activate} onSelectFunction={selectFunction}
        />
        {findings ? <FindingSlot findings={findings} language={language} activate={activate} /> : null}
      </Fragment>
    );
  };

  return <div className="cv-lines" role="table" aria-label={model.path ?? 'code'}>{model.rows.map(renderRow)}</div>;
});
