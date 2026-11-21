// Result line of the post dialog: the posted review's link and counts, the error, or
// the "already posted" question that must be answered before posting a second review.

import type { PostResult } from '../../types';
import { isSafeHttpUrl, plural } from '../../lib/format';

export type PostPhase =
  | { kind: 'idle' }
  | { kind: 'posting' }
  | { kind: 'posted'; result: PostResult }
  /** This session already posted a review; posting again needs explicit confirmation */
  | { kind: 'already-posted'; previousUrl: string; outcomeUnknown?: boolean }
  | { kind: 'error'; message: string };

function ReviewLink({ url }: { url: string }) {
  if (!url) return <span className="pm-outcome-link mono">(link unavailable)</span>;
  return isSafeHttpUrl(url)
    ? <a href={url} target="_blank" rel="noopener noreferrer" className="pm-outcome-link mono">{url}</a>
    : <span className="pm-outcome-link mono">{url}</span>;
}

export function PostOutcome({ phase }: { phase: PostPhase }) {
  if (phase.kind === 'error') {
    return <div className="pm-outcome pm-outcome-error" role="alert">Posting failed: {phase.message}</div>;
  }
  if (phase.kind === 'already-posted' && phase.outcomeUnknown) {
    return (
      <div className="pm-outcome pm-outcome-repost" role="alert">
        <span>Outcome unknown:</span>
        <span className="pm-outcome-question">The last post may have reached GitHub. Check the PR before posting again.</span>
      </div>
    );
  }
  if (phase.kind === 'already-posted') {
    return (
      <div className="pm-outcome pm-outcome-repost" role="alert">
        <span>Already posted:</span>
        <ReviewLink url={phase.previousUrl} />
        <span className="pm-outcome-question">Post again? This creates a second review on the PR.</span>
      </div>
    );
  }
  if (phase.kind !== 'posted') return null;
  const { url, inline_comments: inline, body_findings: body, fallback_to_body: fallback } = phase.result;
  return (
    <div className="pm-outcome pm-outcome-ok" role="status">
      <span>Review posted</span>
      <span className="pm-outcome-counts">{plural(inline, 'inline comment')} · {plural(body, 'finding')} in the body</span>
      <ReviewLink url={url} />
      {fallback ? (
        <p className="pm-outcome-fallback">
          GitHub rejected a line for an inline comment (it was not in the diff GitHub has), so every finding was
          posted in the review body instead, each with its file and line.
        </p>
      ) : null}
    </div>
  );
}
