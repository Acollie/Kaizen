// "Post review" dialog: the human's review. An editable summary, their own comments, the
// AI findings they accepted (with notes), non-blocking warnings about what they haven't
// checked yet, and the POST to /api/post. Unreviewed and dismissed findings never post.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useAllFindings, useComments, useReview } from '../../state/StoreProvider';
import { AlreadyPostedError, postReview } from '../../lib/api';
import { useLookProgress } from '../../hooks/useLookProgress';
import { useAiCoverage } from '../../state/aiCoverage';
import { unviewedRiskyPackages } from '../../lib/packageMap';
import { AI_LOOKED_HINT } from '../../lib/coverageCopy';
import { useOverviewData } from '../overview/useOverviewData';
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
  const { phase, submit } = usePostSubmit(open);
  const total = plan.comments.length + plan.accepted.length;

  const close = () => setPostOpen(false);
  const canPost = phase.kind !== 'posting' && phase.kind !== 'posted' && (total > 0 || summary.trim() !== '');
  const repost = phase.kind === 'already-posted';

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
            onClick={() => submit(summary, postedFindings(plan), repost)}>
            {primaryLabel(phase, plan)}
          </button>
        </div>
      </footer>
    </ModalDialog>
  );
}

function primaryLabel(phase: PostPhase, plan: PostPlan): string {
  switch (phase.kind) {
    case 'posting': return 'Posting…';
    case 'posted': return 'Posted';
    case 'already-posted': return 'Yes, post again';
    case 'error': return 'Retry';
    default: return postButtonLabel(plan);
  }
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
      {warnings.map((warning) => (
        <li key={warning.key} className={`pm-warn pm-warn-${warning.key}`}>
          {warning.text}
          {warning.items && warning.items.length > 0 && (
            <ul className="pm-warn-items">
              {warning.items.map((item, order) => (
                <li key={`${order}:${item.label}`}>
                  <span className="mono">{item.label}</span>
                  {item.aiLooked && <span className="pm-warn-ai" title={AI_LOOKED_HINT}> · AI looked at it, not verified</span>}
                </li>
              ))}
              {warning.more ? <li className="pm-warn-more">and {warning.more} more</li> : null}
            </ul>
          )}
        </li>
      ))}
    </ul>
  );
}

function usePostWarnings(plan: PostPlan): PostWarning[] {
  const findings = useAllFindings();
  const triage = useReview((state) => state.triage);
  const look = useLookProgress();
  const coverage = useAiCoverage();
  const { map } = useOverviewData();
  return useMemo(() => {
    const unviewedPackages = unviewedRiskyPackages(map).map((pkg) => ({
      label: pkg.label,
      aiLooked: pkg.ai.functions.seen + pkg.ai.files.seen > 0,
    }));
    return postWarnings({
      findings, triage, look, acceptedCount: plan.accepted.length, aiFunctions: coverage.examinedFunctions, unviewedPackages,
    });
  }, [findings, triage, look, plan.accepted.length, coverage.examinedFunctions, map]);
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

/**
 * Posting state. A second review is never posted by accident: reopening the dialog
 * after a post, or a 409 from the server (e.g. after a reload), asks "Post again?",
 * and only that explicit answer sends confirm_repost.
 */
function usePostSubmit(open: boolean) {
  const [phase, setPhase] = useState<PostPhase>({ kind: 'idle' });
  const posting = useRef(false);

  useEffect(() => {
    if (open) setPhase((current) => (current.kind === 'posted' ? { kind: 'already-posted', previousUrl: current.result.url } : current));
  }, [open]);

  const submit = async (summary: string, findings: Parameters<typeof postReview>[0]['findings'], confirmRepost: boolean) => {
    if (posting.current) return;
    posting.current = true;
    setPhase({ kind: 'posting' });
    try {
      const result = await postReview({ summary, findings, ...(confirmRepost ? { confirm_repost: true } : {}) });
      setPhase({ kind: 'posted', result });
    } catch (error) {
      if (error instanceof AlreadyPostedError) {
        setPhase({ kind: 'already-posted', previousUrl: error.previousUrl, outcomeUnknown: error.outcomeUnknown });
      }
      else setPhase({ kind: 'error', message: error instanceof Error ? error.message : String(error) });
    } finally {
      posting.current = false;
    }
  };

  return { phase, submit };
}
