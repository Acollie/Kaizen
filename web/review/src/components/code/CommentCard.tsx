// The reviewer's own comments in the code view: a card like a finding's but authored by
// "you" (editable, deletable), and the inline composer that writes them.

import { memo, useEffect, useRef, useState, type KeyboardEvent, type MouseEvent } from 'react';
import { useReview } from '../../state/StoreProvider';
import { commentLocation, type CommentTarget, type ReviewComment } from '../../state/comments';
import { SEVERITY_LABEL } from '../../lib/findings';
import type { Category, Severity } from '../../types';
import { Markdown } from '../shared/Markdown';
import { useIsCursor } from './cursor';
import { findingKey } from './FindingCard';

const COMMENT_SEVERITIES: readonly Severity[] = ['critical', 'warning', 'suggestion'];
const COMMENT_CATEGORIES: readonly Category[] = ['correctness', 'security', 'performance', 'design', 'tests'];

const stop = (event: MouseEvent) => event.stopPropagation();

export function YouTag() {
  return <span className="agent cv-you" title="Written by you">you</span>;
}

interface CommentCardProps {
  comment: ReviewComment;
  onActivate: (key: string) => void;
}

export const CommentCard = memo(function CommentCard({ comment, onActivate }: CommentCardProps) {
  const key = findingKey(comment.id);
  const isCursor = useIsCursor(key);
  const editing = useReview((state) => state.ui.composer?.editingId === comment.id);
  const openComposer = useReview((state) => state.openComposer);
  const deleteComment = useReview((state) => state.deleteComment);
  const severityClass = comment.severity ? `cv-card-${comment.severity}` : 'cv-card-comment';
  const className = ['cv-card', 'cv-card-user', severityClass, isCursor ? 'cv-cursor' : ''].filter(Boolean).join(' ');

  if (editing) {
    return (
      <div className="cv-card cv-card-user cv-card-comment" data-key={key}>
        <div className="cv-card-inner">
          <CommentComposer target={{ file: comment.file, line: comment.line, end_line: comment.end_line, editingId: comment.id }} initial={comment} />
        </div>
      </div>
    );
  }
  const edit = () => openComposer({ file: comment.file, line: comment.line, end_line: comment.end_line, editingId: comment.id });
  return (
    <article className={className} data-key={key} onClick={() => onActivate(key)}>
      <div className="cv-card-inner">
        <div className="cv-card-head">
          <YouTag />
          <span className={`cv-card-sev${comment.severity ? ` sev-${comment.severity}` : ''}`}>
            {comment.severity ? SEVERITY_LABEL[comment.severity] : 'Comment'}
          </span>
          {comment.category ? <span className="cv-card-meta">{comment.category}</span> : null}
          <span className="cv-card-meta mono">{commentLocation(comment)}</span>
        </div>
        <Markdown source={comment.body} className="cv-card-body cv-comment-body" />
        <div className="cv-card-foot" onClick={stop}>
          <button type="button" className="tr-link" onClick={edit}>Edit</button>
          <button type="button" className="tr-link" onClick={() => deleteComment(comment.id)}>Delete</button>
          <span className="cv-card-spacer" />
          <span className="cv-card-meta">will be posted as your comment</span>
        </div>
      </div>
    </article>
  );
});

interface ComposerProps {
  target: CommentTarget;
  initial?: Pick<ReviewComment, 'body' | 'severity' | 'category'>;
}

/** Textarea + optional severity / category; Cmd/Ctrl+Enter saves, Esc cancels */
export function CommentComposer({ target, initial }: ComposerProps) {
  const saveComment = useReview((state) => state.saveComment);
  const closeComposer = useReview((state) => state.closeComposer);
  const [body, setBody] = useState(initial?.body ?? '');
  const [severity, setSeverity] = useState<Severity | undefined>(initial?.severity);
  const [category, setCategory] = useState<Category | undefined>(initial?.category);
  const textRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    textRef.current?.focus({ preventScroll: true });
  }, []);

  const save = () => {
    if (!body.trim()) return;
    saveComment({ file: target.file, line: target.line, end_line: target.end_line, body, severity, category }, target.editingId);
  };

  const onKeyDown = (event: KeyboardEvent) => {
    event.stopPropagation();
    if (event.key === 'Escape') {
      event.preventDefault();
      closeComposer();
    }
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      save();
    }
  };

  return (
    <div className="cv-composer" onClick={stop} onKeyDown={onKeyDown} role="group" aria-label="Write a comment">
      <div className="cv-composer-head">
        <YouTag />
        <span className="cv-card-meta mono">{commentLocation(target)}</span>
        <span className="cv-card-meta">{target.editingId ? 'Editing your comment' : 'New comment'}</span>
      </div>
      <textarea ref={textRef} className="tr-input tr-textarea cv-composer-text" rows={4} value={body}
        placeholder="Leave a comment. Markdown works: **bold**, `code`, ```suggestion blocks```"
        aria-label="Comment" onChange={(event) => setBody(event.target.value)} />
      <ChipRow<Severity> label="Severity" options={COMMENT_SEVERITIES} value={severity} onChange={setSeverity}
        format={(option) => SEVERITY_LABEL[option]} />
      <ChipRow<Category> label="Category" options={COMMENT_CATEGORIES} value={category} onChange={setCategory} format={(option) => option} />
      <div className="cv-composer-foot">
        <button type="button" className="btn btn-primary tr-btn" disabled={!body.trim()} onClick={save}>
          {target.editingId ? 'Save' : 'Comment'}
        </button>
        <button type="button" className="btn tr-btn" onClick={closeComposer}>Cancel</button>
        <span className="cv-card-spacer" />
        <span className="cv-card-meta"><kbd>⌘</kbd><kbd>↵</kbd> to save · <kbd>Esc</kbd> to cancel</span>
      </div>
    </div>
  );
}

interface ChipRowProps<T extends string> {
  label: string;
  options: readonly T[];
  value: T | undefined;
  onChange(value: T | undefined): void;
  format(option: T): string;
}

function ChipRow<T extends string>({ label, options, value, onChange, format }: ChipRowProps<T>) {
  return (
    <div className="cv-chips" role="radiogroup" aria-label={label}>
      <span className="cv-chips-label">{label}</span>
      {options.map((option) => (
        <button key={option} type="button" role="radio" aria-checked={value === option}
          className={`cv-chip${value === option ? ' cv-chip-on' : ''} sev-${option}`}
          onClick={() => onChange(value === option ? undefined : option)}>
          {format(option)}
        </button>
      ))}
    </div>
  );
}
