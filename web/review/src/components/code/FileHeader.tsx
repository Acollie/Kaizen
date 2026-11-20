// Header above the code: path, status, language, +/- counts, file risk, finding counts
// and a Back button when there is navigation history.

import type { FileDiff, Finding } from '../../types';
import { splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { SeverityCounts } from '../shared/bits';
import { useReview } from '../../state/StoreProvider';

export interface FileHeaderProps {
  path: string;
  diff: FileDiff | undefined;
  /** Read-only view of a file outside the diff */
  plain: boolean;
  findings: readonly Finding[];
  hiddenLines: number;
  onExpandAll: () => void;
}

function RiskBadge({ diff }: { diff: FileDiff }) {
  if (!(diff.risk_score > 0)) return null;
  const reasons = diff.risk_reasons?.length ? diff.risk_reasons.join('\n') : 'No specific risk reasons';
  return (
    <span className="cv-head-risk" title={`File risk ${Math.round(diff.risk_score)}\n${reasons}`}>
      <span className="cv-head-risk-dot" style={{ background: riskColor(diff.risk_score) }} />
      <span className="mono">{Math.round(diff.risk_score)}</span>
      <span className="cv-head-dim">risk</span>
    </span>
  );
}

function DiffFacts({ diff }: { diff: FileDiff }) {
  return (
    <>
      <span className="cv-head-dim">{diff.status}</span>
      {diff.language ? <span className="cv-head-dim">{diff.language}</span> : null}
      <span className="cv-head-stat mono">
        <span className="cv-stat-add">+{diff.additions}</span>
        <span className="cv-stat-del">−{diff.deletions}</span>
      </span>
      <RiskBadge diff={diff} />
    </>
  );
}

export function FileHeader({ path, diff, plain, findings, hiddenLines, onExpandAll }: FileHeaderProps) {
  const canGoBack = useReview((state) => state.history.length > 0);
  const goBack = useReview((state) => state.goBack);
  const { dir, base } = splitPath(path);
  return (
    <header className="cv-head">
      <span className="cv-head-path mono" title={path}>
        <span className="cv-head-dir">{dir}</span>
        <span className="cv-head-base">{base}</span>
      </span>
      {diff && !plain ? <DiffFacts diff={diff} /> : <span className="cv-head-note">Not changed in this PR · read-only</span>}
      <span className="cv-head-counts"><SeverityCounts findings={findings} /></span>
      <span className="cv-head-spacer" />
      {hiddenLines > 0 ? (
        <button type="button" className="btn cv-head-btn" onClick={onExpandAll} title="Expand the whole file (zR)">
          Expand all
        </button>
      ) : null}
      {canGoBack ? (
        <button type="button" className="btn btn-outline cv-head-btn" onClick={goBack} title="Jump back (Ctrl-o)">
          ← Back
        </button>
      ) : null}
    </header>
  );
}
