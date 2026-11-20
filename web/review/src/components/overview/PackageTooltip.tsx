// Hover card for a package: its riskiest changed functions and coverage at a glance.

import type { PackageInfo } from '../../lib/packageMap';
import { percent } from '../../lib/packageMap';
import { qualifiedName } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { SeverityCounts } from '../shared/bits';

const TOP_FUNCTIONS = 5;
const CARD_WIDTH = 300;
const CARD_HEIGHT = 190;

export function placePackageTooltip(pointerX: number, pointerY: number, width: number, height: number): { left: number; top: number } {
  const left = pointerX + 16 + CARD_WIDTH > width ? Math.max(4, pointerX - CARD_WIDTH - 12) : pointerX + 16;
  const top = pointerY + 16 + CARD_HEIGHT > height ? Math.max(4, pointerY - CARD_HEIGHT - 12) : pointerY + 16;
  return { left, top };
}

export function PackageTooltip({ pkg, left, top }: { pkg: PackageInfo; left: number; top: number }) {
  const top5 = pkg.functions.slice(0, TOP_FUNCTIONS);
  const riskyFiles = pkg.functions.length ? [] : [...pkg.files].sort((a, b) => b.risk_score - a.risk_score).slice(0, 3);
  return (
    <div className="fl-tooltip ov-tooltip" style={{ left, top }} role="tooltip">
      <div className="fl-tt-name">{pkg.label}</div>
      <div className="fl-tt-stats">
        <span><b>{pkg.files.length}</b> files</span>
        <span><b>{pkg.functions.length}</b> functions</span>
        {pkg.testFiles > 0 && <span><b>{percent(pkg.testShare)}</b> tests</span>}
      </div>
      {top5.length > 0 && <div className="ov-tt-heading">Riskiest changed functions</div>}
      {top5.map((fn) => (
        <div key={fn.id} className="ov-tt-row">
          <span className="ov-tt-risk" style={{ color: riskColor(fn.risk_score) }}>{Math.round(fn.risk_score)}</span>
          <span className="ov-tt-fn">{fn.focus ? '★ ' : ''}{qualifiedName(fn)}</span>
        </div>
      ))}
      {riskyFiles.map((file) => (
        <div key={file.path} className="ov-tt-row">
          <span className="ov-tt-risk" style={{ color: riskColor(file.risk_score) }}>{Math.round(file.risk_score)}</span>
          <span className="ov-tt-fn">{file.path.split('/').pop()}</span>
        </div>
      ))}
      {pkg.findings.length > 0 && (
        <div className="fl-tt-findings"><SeverityCounts findings={pkg.findings} skipPraise={false} /> <span>AI findings</span></div>
      )}
    </div>
  );
}
