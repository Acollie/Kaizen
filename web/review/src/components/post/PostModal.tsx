// "Post review" dialog: the human's review. An editable summary, their own comments, the
// AI findings they accepted (with notes), non-blocking warnings about what they haven't
// checked yet, and the POST to /api/post. Unreviewed and dismissed findings never post.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useAllFindings, useComments, useReview } from '../../state/StoreProvider';
import { postReview } from '../../lib/api';
import { useLookProgress } from '../../hooks/useLookProgress';
import { ModalDialog } from '../layout/ModalDialog';
import { PostFindingGroup } from './PostFindingGroup';
import { PostOutcome, type PostPhase } from './PostOutcome';
import {
  generateSummary, planPost, postButtonLabel, postedFindings, postWarnings, selectionNote, type PostPlan, type PostWarning,
} from './postLogic';
import './PostModal.css';

function usePostPlan(): PostPlan {
  const files = useReview((state) => state.index.files);
  const triage = useReview((state) => state.triage);
  const findings = useAllFindings();
  const comments = useComments();
  return useMemo(() => planPost({ comments, findings, triage, files }), [comments, findings, triage, files]);
}

export function PostModal() {
  const open = useReview((state) => state.ui.postOpen);
  const setPostOpen = useReview((state) => state.setPostOpen);
  const plan = usePostPlan();
  const warnings = usePostWarnings(plan);
  const [summary, setSummary, insertAiSummary] = usePrefilledSummary(open, plan);
  const { phase, submit } = usePostSubmit();
  const total = plan.comments.length + plan.accepted.length;

  const close = () => setPostOpen(false);
  const canPost = phase.kind !== 'posting' && phase.kind !== 'posted' && (total > 0 || summary.trim() !== '');

  return (
    <ModalDialog open={open} onClose={close} labelledBy="pm-title" className="pm-dialog" dismissible={phase.kind !== 'posting'}>
      <header className="modal-head">
        <h2 id="pm-title" className="modal-title">Post your review to GitHub</h2>
        <button type="button" className="btn modal-close" onClick={close} aria-label="Close">×</button>
      </header>
      <div className="modal-body pm-body">
        <p className="pm-lead">This posts a COMMENT review to GitHub as you: your comments and the findings you accepted. Nothing else.</p>
        <PostWarnings warnings={warnings} />
        <div className="pm-field-row">
          <label className="label pm-field-label" htmlFor="pm-summary">Summary</label>
          {insertAiSummary ? <button type="button" className="tr-link" onClick={insertAiSummary}>Use the AI's summary instead</button> : null}
        </div>
        <textarea id="pm-summary" className="pm-summary" data-autofocus value={summary} spellCheck
          onChange={(event) => setSummary(event.target.value)} rows={5} />
        {total === 0 && <NothingAccepted />}
        <PostFindingGroup title="Your comments" hint="written by you" entries={plan.comments} own />
        <PostFindingGroup title="Accepted findings" hint="AI-written, accepted by you" entries={plan.accepted} />
      </div>
      <PostOutcome phase={phase} />
      <footer className="modal-foot pm-foot">
        <span className="pm-note mono">{selectionNote(plan)}</span>
        <div className="pm-buttons">
          <button type="button" className="btn btn-outline" onClick={close}>{phase.kind === 'posted' ? 'Close' : 'Cancel'}</button>
          <button type="button" className="btn btn-primary" disabled={!canPost}
            onClick={() => submit(summary, postedFindings(plan))}>
            {phase.kind === 'posting' ? 'Posting…' : phase.kind === 'posted' ? 'Posted' : phase.kind === 'error' ? 'Retry' : postButtonLabel(plan)}
          </button>
        </div>
      </footer>
    </ModalDialog>
  );
}

function NothingAccepted() {
  return (
    <p className="pm-none" role="note">
      You haven't accepted any findings or written any comments, so there is nothing to post line by line.
      Accept the findings you've checked (<kbd>a</kbd>), or comment on a line (<kbd>c</kbd>). You can still post the summary on its own.
    </p>
  );
}

function PostWarnings({ warnings }: { warnings: PostWarning[] }) {
  if (warnings.length === 0) return null;
  return (
    <ul className="pm-warnings" aria-label="Before you post">
      {warnings.map((warning) => <li key={warning.key} className={`pm-warn pm-warn-${warning.key}`}>{warning.text}</li>)}
    </ul>
  );
}

function usePostWarnings(plan: PostPlan): PostWarning[] {
  const findings = useAllFindings();
  const triage = useReview((state) => state.triage);
  const look = useLookProgress();
  return useMemo(
    () => postWarnings({ findings, triage, look, acceptedCount: plan.accepted.length }),
    [findings, triage, look, plan.accepted.length],
  );
}

/**
 * Summary text generated from what is being posted, until the user edits it. The AI's
 * own summary is offered, not assumed: it describes every finding, not the accepted ones.
 */
function usePrefilledSummary(open: boolean, plan: PostPlan): [string, (value: string) => void, (() => void) | null] {
  const engineSummary = useReview((state) => state.result?.summary ?? '');
  const [summary, setSummary] = useState('');
  const edited = useRef(false);
  const accepted = useMemo(() => plan.accepted.map((entry) => entry.finding), [plan.accepted]);

  useEffect(() => {
    if (!open || edited.current) return;
    setSummary(generateSummary(accepted, plan.comments.length));
  }, [open, accepted, plan.comments.length]);

  const update = (value: string) => {
    edited.current = true;
    setSummary(value);
  };
  const insertAi = engineSummary.trim() && summary !== engineSummary ? () => update(engineSummary) : null;
  return [summary, update, insertAi];
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
