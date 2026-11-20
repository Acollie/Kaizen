// The reviewer's decision on an AI finding, shared by code-view cards and Review panel
// items: Accept / Dismiss (with a reason picker), an optional note, Undo, and the
// "How to verify" step the reviewer should do before accepting.

import { useEffect, useRef, useState, type KeyboardEvent, type MouseEvent } from 'react';
import { useReview } from '../../state/StoreProvider';
import type { DismissSurface } from '../../state/store';
import { DISMISS_REASONS, dismissedLabel, type TriageEntry } from '../../state/triage';
import type { Finding } from '../../types';
import './Triage.css';

const stop = (event: MouseEvent | KeyboardEvent) => event.stopPropagation();

export function VerifyStep({ text }: { text: string | undefined }) {
  if (!text?.trim()) return null;
  return (
    <div className="tr-verify">
      <span className="tr-verify-label">How to verify</span>
      <span className="tr-verify-text">{text}</span>
    </div>
  );
}

interface TriageActionsProps {
  finding: Finding;
  surface: DismissSurface;
}

/** Accept / Dismiss for an unreviewed finding; the accepted state with note and Undo otherwise */
export function TriageActions({ finding, surface }: TriageActionsProps) {
  const entry = useReview((state) => state.triage[finding.id]);
  const picking = useReview((state) => state.ui.dismissing?.id === finding.id && state.ui.dismissing.surface === surface);
  const acceptFinding = useReview((state) => state.acceptFinding);
  const startDismiss = useReview((state) => state.startDismiss);

  if (picking) return <DismissPicker findingId={finding.id} />;
  if (entry?.state === 'accepted') return <AcceptedState findingId={finding.id} entry={entry} />;
  if (entry?.state === 'dismissed') return null;
  return (
    <div className="tr-actions" onClick={stop}>
      <button type="button" className="btn btn-outline tr-btn" title="Accept: post this as your comment (a)"
        onClick={() => acceptFinding(finding.id)}>
        <span aria-hidden="true">✓</span> Accept
      </button>
      <button type="button" className="btn tr-btn" title="Dismiss with a reason (d)"
        onClick={() => startDismiss(finding.id, surface)}>
        Dismiss
      </button>
    </div>
  );
}

function AcceptedState({ findingId, entry }: { findingId: string; entry: TriageEntry }) {
  const undoTriage = useReview((state) => state.undoTriage);
  const setTriageNote = useReview((state) => state.setTriageNote);
  const [editing, setEditing] = useState(false);

  return (
    <div className="tr-accepted" onClick={stop}>
      <div className="tr-accepted-row">
        <span className="tr-check" aria-hidden="true">✓</span>
        <span className="tr-accepted-label">Accepted</span>
        {!editing && (
          <button type="button" className="tr-link" onClick={() => setEditing(true)}>{entry.note ? 'Edit note' : 'Add note'}</button>
        )}
        <button type="button" className="tr-link" onClick={() => undoTriage(findingId)} title="Back to unreviewed">Undo</button>
      </div>
      {editing
        ? <NoteEditor initial={entry.note ?? ''} placeholder="Your note, posted under the comment"
          onSave={(note) => { setTriageNote(findingId, note); setEditing(false); }} onCancel={() => setEditing(false)} />
        : entry.note && <div className="tr-note"><span className="tr-note-label">Your note</span>{entry.note}</div>}
    </div>
  );
}

function DismissPicker({ findingId }: { findingId: string }) {
  const dismissFinding = useReview((state) => state.dismissFinding);
  const cancelDismiss = useReview((state) => state.cancelDismiss);
  const [note, setNote] = useState('');
  const firstRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    firstRef.current?.focus({ preventScroll: true });
  }, []);

  const pick = (reason: string) => dismissFinding(findingId, reason, note);

  const onKeyDown = (event: KeyboardEvent) => {
    event.stopPropagation();
    if (event.key === 'Escape') {
      event.preventDefault();
      cancelDismiss();
      return;
    }
    const isTyping = event.target instanceof HTMLInputElement;
    const reason = DISMISS_REASONS[Number(event.key) - 1];
    if (!isTyping && reason) {
      event.preventDefault();
      pick(reason);
    }
  };

  return (
    <div className="tr-picker" role="group" aria-label="Dismiss reason" onClick={stop} onKeyDown={onKeyDown}>
      <span className="tr-picker-label">Dismiss because</span>
      <div className="tr-reasons">
        {DISMISS_REASONS.map((reason, index) => (
          <button key={reason} ref={index === 0 ? firstRef : undefined} type="button" className="btn btn-outline tr-reason" onClick={() => pick(reason)}>
            <kbd>{index + 1}</kbd>{reason}
          </button>
        ))}
      </div>
      <div className="tr-picker-foot">
        <input className="tr-input" value={note} placeholder="Optional note (for you; not posted)" aria-label="Dismiss note"
          onChange={(event) => setNote(event.target.value)} />
        <button type="button" className="tr-link" onClick={cancelDismiss}>Cancel <kbd>Esc</kbd></button>
      </div>
    </div>
  );
}

interface NoteEditorProps {
  initial: string;
  placeholder: string;
  onSave(note: string): void;
  onCancel(): void;
}

function NoteEditor({ initial, placeholder, onSave, onCancel }: NoteEditorProps) {
  const [value, setValue] = useState(initial);
  const onKeyDown = (event: KeyboardEvent) => {
    event.stopPropagation();
    if (event.key === 'Escape') {
      event.preventDefault();
      onCancel();
    }
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      onSave(value);
    }
  };
  return (
    <div className="tr-note-editor" onKeyDown={onKeyDown}>
      <textarea className="tr-input tr-textarea" value={value} rows={2} placeholder={placeholder} autoFocus aria-label="Reviewer note"
        onChange={(event) => setValue(event.target.value)} />
      <div className="tr-note-buttons">
        <button type="button" className="btn btn-primary tr-btn" onClick={() => onSave(value)}>Save note</button>
        <button type="button" className="btn tr-btn" onClick={onCancel}>Cancel</button>
      </div>
    </div>
  );
}

interface DismissedLineProps {
  finding: Finding;
  entry: TriageEntry;
  onOpen?(): void;
  className?: string;
}

/** A dismissed finding, collapsed to one muted line with its reason and Undo */
export function DismissedLine({ finding, entry, onOpen, className = '' }: DismissedLineProps) {
  const undoTriage = useReview((state) => state.undoTriage);
  return (
    <div className={`tr-dismissed ${className}`} onClick={onOpen} title={entry.note ? `Your note: ${entry.note}` : finding.title}>
      <span className={`sev-dot sev-${finding.severity}`} aria-hidden="true" />
      <span className="tr-dismissed-reason">{dismissedLabel(entry)}</span>
      <span className="tr-dismissed-title">{finding.title}</span>
      <button type="button" className="tr-link" onClick={(event) => { event.stopPropagation(); undoTriage(finding.id); }}>Undo</button>
    </div>
  );
}
