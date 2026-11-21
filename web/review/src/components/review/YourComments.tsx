// "Your comments": what the reviewer wrote themselves, first in the panel. Click to open
// it in the code view (edit or delete it there).

import { useComments, useReview } from '../../state/StoreProvider';
import { commentLocation, type ReviewComment } from '../../state/comments';
import { SEVERITY_LABEL } from '../../lib/findings';
import { splitPath } from '../../lib/format';

export function YourComments() {
  const comments = useComments();
  if (comments.length === 0) return null;
  return (
    <section className="rv-section rv-comments" aria-label="Your comments">
      <h3 className="rv-section-head">
        <span className="label rv-section-title rv-you-title">Your comments</span>
        <span className="rv-section-count mono">{comments.length}</span>
      </h3>
      {comments.map((comment) => <CommentItem key={comment.id} comment={comment} />)}
    </section>
  );
}

function CommentItem({ comment }: { comment: ReviewComment }) {
  const openFinding = useReview((state) => state.openFinding);
  const isSelected = useReview((state) => state.selection.findingId === comment.id);
  const location = `${splitPath(comment.file).base} ${commentLocation(comment)}`;
  return (
    <article className={`rv-item rv-item-user${comment.severity ? ` sev-${comment.severity}` : ''}${isSelected ? ' rv-selected' : ''}`}
      tabIndex={0} onClick={() => openFinding(comment.id)}
      onKeyDown={(event) => { if (event.key === 'Enter') openFinding(comment.id); }}>
      <div className="rv-item-meta">
        <span className="rv-you">you</span>
        <span className="mono rv-loc" title={comment.file}>{location}</span>
        {comment.severity ? <span>{SEVERITY_LABEL[comment.severity]}</span> : null}
        {comment.category ? <span>{comment.category}</span> : null}
      </div>
      <div className="rv-item-body rv-comment-text">{comment.body}</div>
    </article>
  );
}
