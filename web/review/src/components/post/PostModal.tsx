// "Post review" dialog: an editable summary, the findings with include checkboxes
// (each marked inline comment or review body), and the POST to /api/post.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useFindings, useReview } from '../../state/StoreProvider';
import { postReview } from '../../lib/api';
import { ModalDialog } from '../layout/ModalDialog';
import { PostFindingGroup } from './PostFindingGroup';
import { PostOutcome, type PostPhase } from './PostOutcome';
import { generateSummary, planPost, selectedFindings, selectionNote } from './postLogic';
import './PostModal.css';

export function PostModal() {
  const open = useReview((state) => state.ui.postOpen);
  const setPostOpen = useReview((state) => state.setPostOpen);
  const included = useReview((state) => state.included);
  const files = useReview((state) => state.index.files);
  const findings = useFindings();
  const plan = useMemo(() => planPost(findings, files), [findings, files]);
  const [summary, setSummary] = usePrefilledSummary(open);
  const { phase, submit } = usePostSubmit();
  const selectedCount = plan.inline.concat(plan.body).filter((finding) => included[finding.id]).length;

  const close = () => setPostOpen(false);
  const canPost = phase.kind !== 'posting' && phase.kind !== 'posted' && (selectedCount > 0 || summary.trim() !== '');

  return (
    <ModalDialog open={open} onClose={close} labelledBy="pm-title" className="pm-dialog" dismissible={phase.kind !== 'posting'}>
      <header className="modal-head">
        <h2 id="pm-title" className="modal-title">Post review to GitHub</h2>
        <button type="button" className="btn modal-close" onClick={close} aria-label="Close">×</button>
      </header>
      <div className="modal-body pm-body">
        <p className="pm-warning">This posts a COMMENT review to GitHub as you.</p>
        <label className="label pm-field-label" htmlFor="pm-summary">Summary</label>
        <textarea id="pm-summary" className="pm-summary" data-autofocus value={summary} spellCheck
          onChange={(event) => setSummary(event.target.value)} rows={6} />
        {findings.length === 0 && <p className="pm-none">No findings yet. You can still post the summary.</p>}
        <PostFindingGroup title="Inline comments" hint="on a line in the diff" findings={plan.inline} />
        <PostFindingGroup title="Review body" hint="file-level or outside the diff" findings={plan.body} />
      </div>
      <PostOutcome phase={phase} />
      <footer className="modal-foot pm-foot">
        <span className="pm-note mono">{selectionNote(plan, included)}</span>
        <div className="pm-buttons">
          <button type="button" className="btn btn-outline" onClick={close}>{phase.kind === 'posted' ? 'Close' : 'Cancel'}</button>
          <button type="button" className="btn btn-primary" disabled={!canPost}
            onClick={() => submit(summary, selectedFindings(findings, included))}>
            {phase.kind === 'posting' ? 'Posting…' : phase.kind === 'posted' ? 'Posted' : phase.kind === 'error' ? 'Retry' : 'Post review'}
          </button>
        </div>
      </footer>
    </ModalDialog>
  );
}

/** Summary text: the engine's summary, or one generated from the findings, until the user edits it */
function usePrefilledSummary(open: boolean): [string, (value: string) => void] {
  const engineSummary = useReview((state) => state.result?.summary ?? '');
  const findings = useFindings();
  const [summary, setSummary] = useState('');
  const edited = useRef(false);

  useEffect(() => {
    if (!open || edited.current) return;
    setSummary(engineSummary.trim() ? engineSummary : generateSummary(findings));
  }, [open, engineSummary, findings]);

  const update = (value: string) => {
    edited.current = true;
    setSummary(value);
  };
  return [summary, update];
}

function usePostSubmit() {
  const [phase, setPhase] = useState<PostPhase>({ kind: 'idle' });
  const posting = useRef(false);

  const submit = async (summary: string, findings: Parameters<typeof postReview>[0]['findings']) => {
    if (posting.current) return;
    posting.current = true;
    setPhase({ kind: 'posting' });
    try {
      const result = await postReview({ summary, findings });
      setPhase({ kind: 'posted', result });
    } catch (error) {
      setPhase({ kind: 'error', message: error instanceof Error ? error.message : String(error) });
    } finally {
      posting.current = false;
    }
  };

  return { phase, submit };
}
