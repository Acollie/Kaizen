// One group in the post dialog ("Your comments" / "Accepted findings"): what will be
// posted, each marked inline or review body. Accepted findings can be un-accepted here.

import { useReview } from '../../state/StoreProvider';
import { formatLineRef, SEVERITY_LABEL } from '../../lib/findings';
import { splitPath } from '../../lib/format';
import { firstSentence } from '../review/reviewModel';
import { SeverityDot } from '../shared/bits';
import type { PostEntry } from './postLogic';

interface PostFindingGroupProps {
  title: string;
  hint: string;
  entries: PostEntry[];
  /** True for the reviewer's own comments (no un-accept; edit them in the code view) */
  own?: boolean;
}

export function PostFindingGroup({ title, hint, entries, own = false }: PostFindingGroupProps) {
  if (entries.length === 0) return null;
  return (
    <section className="pm-group">
      <div className="pm-group-head">
        <span className="label">{title}</span>
        <span className="pm-group-count mono">{entries.length}</span>
        <span className="pm-group-hint">{hint}</span>
      </div>
      <ul className="pm-list">
        {entries.map((entry) => <PostFindingRow key={entry.finding.id} entry={entry} own={own} />)}
      </ul>
    </section>
  );
}

function PostFindingRow({ entry, own }: { entry: PostEntry; own: boolean }) {
  const undoTriage = useReview((state) => state.undoTriage);
  const note = useReview((state) => state.triage[entry.finding.id]?.note);
  const ownSeverity = useReview((state) => state.comments[entry.finding.id]?.severity);
  const { finding } = entry;
  const { base } = splitPath(finding.file);
  const title = own ? firstSentence(finding.body) : finding.title;
  return (
    <li className="pm-item">
      {own ? <span className="pm-you">you</span> : <span className="pm-check" aria-label="Accepted">✓</span>}
      <SeverityDot severity={finding.severity} />
      <span className="pm-item-main">
        <span className="pm-item-title" title={title}>{title}</span>
        {note ? <span className="pm-item-note" title={note}>+ your note</span> : null}
      </span>
      <span className="pm-item-sev">{own && !ownSeverity ? 'Comment' : SEVERITY_LABEL[finding.severity]}</span>
      <span className="pm-item-loc mono" title={finding.file}>
        {base}{finding.line ? ` ${formatLineRef(finding)}` : ''} · {entry.inline ? 'inline' : 'body'}
      </span>
      {own ? <span /> : (
        <button type="button" className="tr-link pm-remove" onClick={() => undoTriage(finding.id)}
          aria-label={`Un-accept ${finding.title}`} title="Un-accept (back to unreviewed)">×</button>
      )}
    </li>
  );
}
