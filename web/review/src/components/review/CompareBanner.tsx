// After a Think-first reveal: how the reviewer's own pass compares with the AI's
// findings. Informative, not a score.

import { useCompare } from '../../hooks/useCompare';
import { compareSentence } from '../../state/compare';

export function CompareBanner() {
  const view = useCompare();
  if (!view) return null;
  const outside = view.outsideIds.size;
  return (
    <div className="rv-compare" role="status" aria-label="Compare">
      <span className="label rv-compare-label">Compare</span>
      <span className="rv-compare-text">{compareSentence(view)}</span>
      {outside > 0 && (
        <span className="rv-compare-hint">{outside} {outside === 1 ? 'finding is' : 'findings are'} in files outside your pass.</span>
      )}
    </div>
  );
}
