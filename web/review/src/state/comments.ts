// The reviewer's own comments. They are first-class: shown like findings (author "you"),
// counted as accepted (the human wrote them) and posted first. On the wire each comment
// becomes a Finding with source "you" so the server needs no new type.

import type { Category, Finding, Severity } from '../types';
import type { DiffLineKind } from '../types';

export const USER_SOURCE = 'you';
export const COMMENT_ID_PREFIX = 'c-';
export const DEFAULT_COMMENT_SEVERITY: Severity = 'suggestion';
export const DEFAULT_COMMENT_CATEGORY: Category = 'design';

export interface ReviewComment {
  id: string;
  file: string;
  /** Head-side line; 0 for a file-level comment */
  line: number;
  end_line?: number;
  /** Markdown */
  body: string;
  severity?: Severity;
  category?: Category;
  createdAt: number;
}

export type CommentMap = Readonly<Record<string, ReviewComment>>;

/** Where a new comment goes (or which comment is being edited) */
export interface CommentTarget {
  file: string;
  line: number;
  end_line?: number;
  editingId?: string;
}

export interface CommentInput {
  file: string;
  line: number;
  end_line?: number;
  body: string;
  severity?: Severity;
  category?: Category;
}

export function isUserComment(finding: Pick<Finding, 'id' | 'source'>): boolean {
  return finding.source === USER_SOURCE && finding.id.startsWith(COMMENT_ID_PREFIX);
}

/** A stable, unique comment id ("c-lq3k9x-1") */
export function newCommentId(now: number, existing: CommentMap): string {
  let counter = 1;
  let id = `${COMMENT_ID_PREFIX}${now.toString(36)}-${counter}`;
  while (id in existing) id = `${COMMENT_ID_PREFIX}${now.toString(36)}-${++counter}`;
  return id;
}

/** Normalise a range: ordered, end_line dropped when it equals line */
export function normalizeRange(line: number, endLine: number | undefined): { line: number; end_line?: number } {
  if (!endLine || endLine === line) return { line };
  return { line: Math.min(line, endLine), end_line: Math.max(line, endLine) };
}

/** The comment as a Finding: for display, n/N navigation and posting */
export function commentToFinding(comment: ReviewComment): Finding {
  const finding: Finding = {
    id: comment.id,
    severity: comment.severity ?? DEFAULT_COMMENT_SEVERITY,
    category: comment.category ?? DEFAULT_COMMENT_CATEGORY,
    file: comment.file,
    title: '',
    body: comment.body,
    confidence: 1,
    source: USER_SOURCE,
  };
  if (comment.line) finding.line = comment.line;
  if (comment.end_line) finding.end_line = comment.end_line;
  return finding;
}

/** Comments in the order they were written */
export function commentsList(comments: CommentMap): ReviewComment[] {
  return Object.values(comments).sort((left, right) => left.createdAt - right.createdAt || left.id.localeCompare(right.id));
}

/** GitHub accepts review comments on the RIGHT side only: added and context lines */
export function isCommentableKind(kind: DiffLineKind): boolean {
  return kind !== 'removed';
}

export const REMOVED_LINE_HINT = 'GitHub only takes review comments on the new side of the diff. Comment on an added or unchanged line instead.';

/** "L12" or "L12–18" or "file" */
export function commentLocation(comment: Pick<ReviewComment, 'line' | 'end_line'>): string {
  if (!comment.line) return 'file';
  return comment.end_line && comment.end_line !== comment.line ? `L${comment.line}–${comment.end_line}` : `L${comment.line}`;
}
