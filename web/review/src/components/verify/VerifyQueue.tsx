// The verify queue (V): a focused mode that shows one AI finding at a time with its
// severity, the "How to verify" step, the full body and the code around it, and takes
// single-key decisions. Fixed regions (header, snippet box, action bar) keep the layout
// still while stepping. It owns the keyboard while open (useKeymap stands aside).

import { useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import { useReview, useReviewStore } from '../../state/StoreProvider';
import { functionAtLine, type ReviewState } from '../../state/store';
import { DISMISS_REASONS } from '../../state/triage';
import { hiddenNitCount, queueProgress, queueSentence, collapsedNitsLabel, visibleIds } from '../../state/verifyQueue';
import { VERIFY_BINDINGS, verifyActionFor, verifyToken, type VerifyAction } from '../../keys/keymap';
import { highlightLanguage, highlightLine } from '../../lib/highlight';
import { formatLineRef } from '../../lib/findings';
import { qualifiedName, splitPath } from '../../lib/format';
import type { FileDiff, Finding, FunctionRef, FunctionRisk } from '../../types';
import { AgentTag, ConfidenceWord, PassTag, SeverityChip } from '../shared/bits';
import { Markdown } from '../shared/Markdown';
import { useCompareTag } from '../../hooks/useCompare';
import { needsFetch, snippetRange, snippetRows, SNIPPET_ROWS, type SnippetRow } from './snippet';
import { prefetchSnippet, useSnippetLines } from './useSnippetLines';
import './VerifyQueue.css';

const MARKS = { added: '+', removed: '−', context: '' } as const;

export function VerifyQueue() {
  const session = useReview((state) => state.ui.verify);
  if (!session) return null;
  return <VerifyQueueOpen />;
}

function useQueueView() {
  const session = useReview((state) => state.ui.verify)!;
  const findings = useReview((state) => state.findings);
  const triage = useReview((state) => state.triage);
  return useMemo(() => {
    const visible = visibleIds(session, findings, triage);
    return {
      session,
      visible,
      finding: session.currentId ? findings[session.currentId] ?? null : null,
      progress: queueProgress(session, visible, triage),
      hiddenNits: hiddenNitCount(session, findings, triage),
      nextId: session.currentId ? visible[visible.indexOf(session.currentId) + 1] ?? null : null,
    };
  }, [session, findings, triage]);
}

function VerifyQueueOpen() {
  const store = useReviewStore();
  const view = useQueueView();
  const [composing, setComposing] = useState(false);
  const { finding, session } = view;
  const index = useReview((state) => state.index);

  // Leaving a finding closes its composer
  useEffect(() => setComposing(false), [session.currentId]);

  // Warm the next finding's code so j is instant
  const nextFinding = useReview((state) => (view.nextId ? state.findings[view.nextId] : undefined));
  useEffect(() => {
    if (!nextFinding) return;
    const file = index.files.get(nextFinding.file);
    const range = snippetRange(nextFinding, file);
    if (file && range) prefetchSnippet(nextFinding.file, range, needsFetch(file, range));
  }, [nextFinding, index]);

  useQueueKeys(composing, setComposing);

  return (
    <div className="vq-backdrop" role="dialog" aria-modal="true" aria-label="Verify queue">
      <div className="vq">
        <QueueHeader progress={view.progress} />
        {finding
          ? <FindingView finding={finding} relations={session.showRelations} composing={composing} onCloseComposer={() => setComposing(false)} />
          : <div className="vq-empty">No AI findings to verify.</div>}
        {view.hiddenNits > 0 && (
          <button type="button" className="vq-nits" onClick={() => store.getState().verifyToggleNits()}>
            {collapsedNitsLabel(view.hiddenNits)} <kbd>n</kbd> to show
          </button>
        )}
        <ActionBar picking={session.picking} disabled={!finding} onCompose={() => setComposing(true)} />
      </div>
    </div>
  );
}

/** Keyboard: the queue's own keys (VERIFY_BINDINGS), plus 1-4 while picking a dismiss reason */
function useQueueKeys(composing: boolean, setComposing: (open: boolean) => void) {
  const store = useReviewStore();
  const latest = useRef({ composing, setComposing });
  latest.current = { composing, setComposing };

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (latest.current.composing) return; // the composer handles its own keys
      const target = event.target;
      if (target instanceof Element && target.closest('input, textarea, select')) return;
      const state = store.getState();
      const session = state.ui.verify;
      if (!session) return;
      const token = verifyToken(event);
      if (session.picking) {
        const reason = DISMISS_REASONS[Number(event.key) - 1];
        if (reason) state.verifyDismiss(reason);
        else if (token === 'Esc' || token === 'd') state.verifyCancelDismiss();
        else return;
        event.preventDefault();
        event.stopPropagation();
        return;
      }
      const action = verifyActionFor(token);
      if (!action) return;
      event.preventDefault();
      event.stopPropagation();
      runAction(action, state, () => latest.current.setComposing(true));
    }
    document.addEventListener('keydown', onKeyDown, true);
    return () => document.removeEventListener('keydown', onKeyDown, true);
  }, [store]);
}

function runAction(action: VerifyAction, state: ReviewState, compose: () => void) {
  const id = state.ui.verify?.currentId ?? null;
  switch (action) {
    case 'accept': state.verifyAccept(); break;
    case 'dismiss': state.verifyStartDismiss(); break;
    case 'skip': state.verifyMove(1); break;
    case 'next': state.verifyMove(1); break;
    case 'prev': state.verifyMove(-1); break;
    case 'comment': if (id) compose(); break;
    case 'relations': state.verifyToggleRelations(); break;
    case 'nits': state.verifyToggleNits(); break;
    case 'open':
      state.closeVerify();
      if (id) state.openFinding(id);
      break;
    case 'exit': state.closeVerify(); break;
  }
}

function QueueHeader({ progress }: { progress: ReturnType<typeof queueProgress> }) {
  const close = useReview((state) => state.closeVerify);
  return (
    <header className="vq-head">
      <span className="label vq-title">Verify queue</span>
      <span className="vq-progress-text mono">{queueSentence(progress)}</span>
      <button type="button" className="btn vq-close" onClick={close} aria-label="Leave the verify queue">Esc</button>
      <div className="vq-bar" aria-hidden="true">
        <span className="vq-bar-accepted" style={{ width: `${pct(progress.accepted, progress.total)}%` }} />
        <span className="vq-bar-dismissed" style={{ width: `${pct(progress.dismissed, progress.total)}%` }} />
      </div>
    </header>
  );
}

const pct = (part: number, total: number) => (total ? (part / total) * 100 : 0);

interface FindingViewProps {
  finding: Finding;
  relations: boolean;
  composing: boolean;
  onCloseComposer(): void;
}

function FindingView({ finding, relations, composing, onCloseComposer }: FindingViewProps) {
  const index = useReview((state) => state.index);
  const entry = useReview((state) => state.triage[finding.id]);
  const tag = useCompareTag(finding.id);
  const file = index.files.get(finding.file);
  // The function the finding's line is in; the AI's function_id only when there is no line
  const fn = (finding.line ? functionAtLine(index, finding.file, finding.line) : null)
    || (finding.function_id ? index.functions.get(finding.function_id) : null) || null;
  const bodyRef = useRef<HTMLDivElement>(null);
  useEffect(() => { if (bodyRef.current) bodyRef.current.scrollTop = 0; }, [finding.id]);

  return (
    <div className="vq-main" ref={bodyRef}>
      <div className="vq-meta">
        <SeverityChip severity={finding.severity} />
        <span>{finding.category}</span>
        <span className="mono vq-loc" title={finding.file}>{splitPath(finding.file).base} {formatLineRef(finding)}</span>
        <ConfidenceWord confidence={finding.confidence} />
        <PassTag kind={tag} />
        <span className="vq-status">{entry ? (entry.state === 'accepted' ? '✓ accepted' : `dismissed · ${entry.reason ?? ''}`) : 'unreviewed'}</span>
      </div>
      <h2 className="vq-title-text">{finding.title || '(untitled finding)'}</h2>
      <div className="vq-verify">
        <span className="vq-verify-label">How to verify</span>
        <span className="vq-verify-text">{finding.verify?.trim() || 'No verification step given. Read the code below and decide for yourself.'}</span>
      </div>
      <Snippet finding={finding} file={file} />
      {relations && <Relations fn={fn} />}
      {composing && <QueueComposer finding={finding} onClose={onCloseComposer} />}
      <Markdown source={finding.body} className="vq-body" />
      {finding.suggestion ? <pre className="vq-suggestion mono">{finding.suggestion}</pre> : null}
      <div className="vq-foot"><AgentTag agent={finding.source} />{fn ? <span className="mono vq-fn">in {qualifiedName(fn)}</span> : null}</div>
    </div>
  );
}

function Snippet({ finding, file }: { finding: Finding; file: FileDiff | undefined }) {
  const range = useMemo(() => snippetRange(finding, file), [finding, file]);
  const needed = Boolean(file && range && needsFetch(file, range));
  const { lines, failed } = useSnippetLines(finding.file, range, needed);
  const rows = useMemo(() => (range ? snippetRows(file, range, needed ? lines : new Map()) : []), [file, range, needed, lines]);
  const language = highlightLanguage(finding.file, file?.language);
  return (
    <div className="vq-snippet" style={{ height: `calc(${SNIPPET_ROWS} * var(--code-line) + 2px)` }} aria-label="Code">
      {rows.length === 0
        ? <div className="vq-snippet-empty">{file ? 'No code to show for this finding.' : 'This file is not part of the PR diff.'}</div>
        : rows.map((row) => <SnippetLine key={row.key} row={row} language={language} />)}
      {failed && <div className="vq-snippet-note">Some lines are unavailable.</div>}
    </div>
  );
}

function SnippetLine({ row, language }: { row: SnippetRow; language: string }) {
  const tokens = useMemo(() => highlightLine(row.text, language), [row.text, language]);
  return (
    <div className={`vq-line vq-${row.kind}${row.marked ? ' vq-marked' : ''}`}>
      <span className="vq-ln mono">{row.newLine || row.oldLine || ''}</span>
      <span className="vq-mark mono">{MARKS[row.kind]}</span>
      <span className="vq-code mono">
        {row.pending ? <span className="vq-pending" /> : tokens.map((token, tokenIndex) => (token.kind === 'plain'
          ? token.text
          : <span key={tokenIndex} className={`cv-tok-${token.kind}`}>{token.text}</span>))}
      </span>
    </div>
  );
}

function Relations({ fn }: { fn: FunctionRisk | null }) {
  if (!fn) return <div className="vq-relations vq-relations-none">No enclosing changed function.</div>;
  return (
    <div className="vq-relations">
      <RefList title={`Callers of ${qualifiedName(fn)}`} refs={fn.callers ?? []} />
      <RefList title="Callees" refs={fn.callees ?? []} />
    </div>
  );
}

function RefList({ title, refs }: { title: string; refs: readonly FunctionRef[] }) {
  return (
    <div className="vq-refs">
      <span className="label">{title}</span>
      {refs.length === 0 ? <span className="vq-refs-none">none</span> : (
        <ul>
          {refs.slice(0, 12).map((ref) => (
            <li key={`${ref.id}:${ref.line}`} className="mono">
              {ref.name}<span className="vq-ref-loc">{ref.external ? 'external' : `${splitPath(ref.file).base}:${ref.line}`}</span>
            </li>
          ))}
          {refs.length > 12 && <li className="vq-refs-none">+{refs.length - 12} more</li>}
        </ul>
      )}
    </div>
  );
}

/** Your own comment at the finding's spot; Ctrl/Cmd-Enter saves, Esc cancels */
function QueueComposer({ finding, onClose }: { finding: Finding; onClose(): void }) {
  const saveComment = useReview((state) => state.saveComment);
  const [body, setBody] = useState('');
  const save = () => {
    if (body.trim()) saveComment({ file: finding.file, line: finding.line ?? 0, end_line: finding.end_line, body });
    onClose();
  };
  const onKeyDown = (event: ReactKeyboardEvent) => {
    event.stopPropagation();
    if (event.key === 'Escape') { event.preventDefault(); onClose(); }
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); save(); }
  };
  return (
    <div className="vq-composer" onKeyDown={onKeyDown}>
      <label className="label" htmlFor="vq-comment">Your comment at {splitPath(finding.file).base} {formatLineRef(finding)}</label>
      <textarea id="vq-comment" className="tr-input tr-textarea" rows={3} autoFocus value={body}
        placeholder="In your own words. Posted as yours." onChange={(event) => setBody(event.target.value)} />
      <div className="vq-composer-buttons">
        <button type="button" className="btn btn-primary" onClick={save}>Save <kbd>Ctrl-Enter</kbd></button>
        <button type="button" className="btn" onClick={onClose}>Cancel <kbd>Esc</kbd></button>
      </div>
    </div>
  );
}

/** Fixed-height bar: the key hints, or the dismiss reasons while picking */
function ActionBar({ picking, disabled, onCompose }: { picking: boolean; disabled: boolean; onCompose(): void }) {
  const store = useReviewStore();
  if (picking) {
    return (
      <footer className="vq-actions vq-picking" role="group" aria-label="Dismiss reason">
        <span className="vq-actions-label">Dismiss because</span>
        {DISMISS_REASONS.map((reason, index) => (
          <button key={reason} type="button" className="btn btn-outline vq-act" onClick={() => store.getState().verifyDismiss(reason)}>
            <kbd>{index + 1}</kbd>{reason}
          </button>
        ))}
        <button type="button" className="btn vq-act" onClick={() => store.getState().verifyCancelDismiss()}><kbd>Esc</kbd>Cancel</button>
      </footer>
    );
  }
  const shown = VERIFY_BINDINGS.filter((binding) => binding.action !== 'exit' && binding.action !== 'nits');
  return (
    <footer className="vq-actions" aria-label="Actions">
      {shown.map((binding) => (
        <button key={binding.action} type="button" className={`btn vq-act${binding.action === 'accept' ? ' btn-outline' : ''}`}
          disabled={disabled} title={binding.description}
          onClick={() => runAction(binding.action, store.getState(), onCompose)}>
          <kbd>{binding.keys[0]}</kbd>{ACTION_LABEL[binding.action]}
        </button>
      ))}
    </footer>
  );
}

const ACTION_LABEL: Record<VerifyAction, string> = {
  accept: 'Accept', dismiss: 'Dismiss', skip: 'Skip', comment: 'Comment', open: 'Open in code',
  relations: 'Callers', next: 'Next', prev: 'Prev', nits: 'Suggestions', exit: 'Exit',
};
