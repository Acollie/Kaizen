// "Where to look": a short ranked list of risky or flagged packages you haven't reviewed.
// Packages no human has opened come first. The AI's coverage is shown in the reason but
// never lowers a package's rank: the AI looking at code is not a review.

import type { LookTarget } from '../../lib/packageMap';
import { AI_LOOKED_HINT } from '../../lib/coverageCopy';
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
        <span className="ov-aside-hint">risky or flagged, not yet reviewed by you{aiKnown ? '' : ' (AI coverage unknown)'}</span>
      </div>
      {targets.length === 0
        ? <div className="ov-look-empty">You've viewed every risky or flagged package. Kaizen ranks by metrics; it can still miss things.</div>
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
      <p className="ov-look-note"><strong>AI looked at:</strong> {AI_LOOKED_HINT}</p>
    </section>
  );
}
