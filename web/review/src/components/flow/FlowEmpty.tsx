// Shown when the PR has no call graph (Kaizen builds call flow for Go code).

import { useReview } from '../../state/StoreProvider';
import { changedLanguages } from './flowData';

export function FlowEmpty() {
  const files = useReview((state) => state.context.files);
  const languages = changedLanguages(files ?? []);
  const what = languages.length ? `This PR changes ${languages.join(', ')} code` : 'This PR has no analysable code changes';
  return (
    <div className="fl-empty">
      <div className="fl-empty-title">No call flow for this PR</div>
      <p>{what}. Kaizen builds the call graph from Go sources, so there are no callers or callees to draw.</p>
      <p>Risk scores, metrics and AI findings still work: use the Function tab and the code view.</p>
    </div>
  );
}
