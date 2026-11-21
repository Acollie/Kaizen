// "Your pass": shown instead of the AI's findings while Think first is on. A short
// checklist from Kaizen's own data (Where to look packages, focus functions, risky
// non-code files), progress from what you've viewed, a visible way to write your own
// comments, and the reveal button, which is never a gate. Nothing here comes from the AI.

import { useMemo } from 'react';
import { useComments, useReview, useReviewStore } from '../../state/StoreProvider';
import {
  buildPass, nextPassItem, passProgress, passPrompt, passSentence, passTarget, revealLabel,
  type PassItem, type PassTarget,
} from '../../state/ownPass';
import { riskColor } from '../../lib/risk';
import { splitPath } from '../../lib/format';

const KIND_LABEL: Record<PassItem['kind'], string> = { package: 'package', function: 'focus', file: 'file' };

export function YourPass({ ready, running }: { ready: number; running: boolean }) {
  const store = useReviewStore();
  const files = useReview((state) => state.context.files);
  const functions = useReview((state) => state.context.functions);
  const graph = useReview((state) => state.context.graph);
  const functionsByFile = useReview((state) => state.index.functionsByFile);
  const viewed = useReview((state) => state.viewed);
  const reveal = useReview((state) => state.revealFindings);
  const items = useMemo(
    () => buildPass({ files: files ?? [], functions: functions ?? [], graph, functionsByFile }),
    [files, functions, graph, functionsByFile],
  );
  const progress = useMemo(() => passProgress(items, viewed), [items, viewed]);
  const next = nextPassItem(items, progress);

  const open = (target: PassTarget) => {
    const state = store.getState();
    if (target.kind === 'function') state.selectFunction(target.id);
    else if (target.path) state.openFile(target.path);
  };

  return (
    <section className={`rv-pass${progress.done ? ' rv-pass-done' : ''}`} aria-label="Your pass">
      <div className="rv-pass-head">
        <span className="label">Your pass</span>
        {progress.total > 0 && <span className="rv-pass-count mono">{passSentence(progress)}</span>}
        {next && (
          <button type="button" className="tr-link rv-pass-next" onClick={() => open(passTarget(next))} title="Open the next item you haven't viewed">
            next →
          </button>
        )}
      </div>
      {progress.total > 0 && (
        <div className="rv-pass-bar" aria-hidden="true">
          <span style={{ width: `${(progress.viewed / progress.total) * 100}%` }} />
        </div>
      )}
      <p className="rv-pass-prompt">{passPrompt(progress, running)}</p>
      {items.length > 0 && (
        <ul className="rv-pass-list">
          {items.map((item) => (
            <PassRow key={item.key} item={item} viewed={progress.viewedKeys.has(item.key)} onOpen={() => open(passTarget(item))} />
          ))}
        </ul>
      )}
      <CommentPrompt />
      <button type="button" className={`btn ${progress.done ? 'btn-primary' : 'btn-outline'} rv-pass-reveal`} onClick={reveal}
        title="Show the AI's findings now. You can reveal at any time.">
        {revealLabel(ready)}
      </button>
    </section>
  );
}

function PassRow({ item, viewed, onOpen }: { item: PassItem; viewed: boolean; onOpen(): void }) {
  return (
    <li>
      <button type="button" className={`rv-pass-row${viewed ? ' rv-pass-viewed' : ''}`} onClick={onOpen}
        title={viewed ? 'You viewed this' : 'Not viewed yet'}>
        <span className="rv-pass-check" aria-label={viewed ? 'viewed' : 'not viewed'}>{viewed ? '✓' : ''}</span>
        <span className="rv-risk mono" style={{ color: riskColor(item.risk) }} title={`Kaizen risk ${Math.round(item.risk)}`}>{Math.round(item.risk)}</span>
        <span className="rv-pass-main">
          <span className="rv-pass-name mono">{item.kind === 'file' ? splitPath(item.label).base : item.label}</span>
          <span className="rv-pass-reason"><span className="rv-pass-kind">{KIND_LABEL[item.kind]}</span>{item.reason}</span>
        </span>
      </button>
    </li>
  );
}

/** Makes writing your own comments obvious while you do your pass */
function CommentPrompt() {
  const store = useReviewStore();
  const comments = useComments();
  const file = useReview((state) => state.selection.file);
  const inPr = useReview((state) => Boolean(state.selection.file && state.index.files.has(state.selection.file)));
  const commentOnFile = () => {
    if (!file) return;
    const state = store.getState();
    state.setMainView('code');
    state.openComposer({ file, line: 0 });
  };
  return (
    <div className="rv-pass-comment">
      <span className="rv-pass-comment-text">
        {comments.length > 0 ? `You've written ${comments.length} ${comments.length === 1 ? 'comment' : 'comments'}. ` : ''}
        Comment as you go: <kbd>c</kbd> on a line, or the + in the gutter.
      </span>
      {inPr && file && (
        <button type="button" className="btn btn-outline rv-pass-comment-btn" onClick={commentOnFile} title={`Comment on ${file}`}>
          + Comment on {splitPath(file).base}
        </button>
      )}
    </div>
  );
}
