// Result line of the post dialog: the posted review's link and counts, or the error.

import type { PostResult } from '../../types';
import { isSafeHttpUrl, plural } from '../../lib/format';

export type PostPhase =
  | { kind: 'idle' }
  | { kind: 'posting' }
  | { kind: 'posted'; result: PostResult }
  | { kind: 'error'; message: string };

export function PostOutcome({ phase }: { phase: PostPhase }) {
  if (phase.kind === 'error') {
    return <div className="pm-outcome pm-outcome-error" role="alert">Posting failed: {phase.message}</div>;
  }
  if (phase.kind !== 'posted') return null;
  const { url, inline_comments: inline, body_findings: body } = phase.result;
  return (
    <div className="pm-outcome pm-outcome-ok" role="status">
      <span>Review posted</span>
      <span className="pm-outcome-counts">{plural(inline, 'inline comment')} · {plural(body, 'finding')} in the body</span>
      {isSafeHttpUrl(url)
        ? <a href={url} target="_blank" rel="noopener noreferrer" className="pm-outcome-link mono">{url}</a>
        : <span className="pm-outcome-link mono">{url}</span>}
    </div>
  );
}
