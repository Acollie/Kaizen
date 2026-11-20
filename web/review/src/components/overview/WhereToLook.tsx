// "Where to look": a short ranked list of packages that are risky or carry urgent AI
// findings but that you and/or the AI have barely looked at. Each row says why.

import type { LookTarget } from '../../lib/packageMap';
import { riskColor } from '../../lib/risk';

interface WhereToLookProps {
  targets: readonly LookTarget[];
  aiKnown: boolean;
  onOpen(key: string): void;
}

export function WhereToLook({ targets, aiKnown, onOpen }: WhereToLookProps) {
  return (
    <section className="ov-look">
      <div className="ov-aside-head">
        <span className="label">Where to look</span>
        <span className="ov-aside-hint">risky or flagged, little looked at{aiKnown ? '' : ' (AI coverage unknown)'}</span>
      </div>
      {targets.length === 0
        ? <div className="ov-look-empty">Nothing risky is left unlooked-at by both you and the AI. Kaizen ranks by metrics; it can still miss things.</div>
        : (
          <ol className="ov-look-list">
            {targets.map((target, order) => (
              <li key={target.pkg.key}>
                <button type="button" className="ov-look-row" onClick={() => onOpen(target.pkg.key)}
                  title={`Show the files and functions in ${target.pkg.label}`}>
                  <span className="ov-look-rank mono">{order + 1}</span>
                  <span className="ov-look-text">
                    <span className="ov-look-pkg mono">
                      <span className="ov-look-stripe" style={{ background: riskColor(target.pkg.maxRisk) }} aria-hidden="true" />
                      {target.pkg.label}
                    </span>
                    <span className="ov-look-reason">{target.reasons.join(' · ')}</span>
                  </span>
                </button>
              </li>
            ))}
          </ol>
        )}
    </section>
  );
}
