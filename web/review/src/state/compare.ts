// After "Think first": compare the reviewer's own pass with what the AI raised. The
// reveal moment (when, and what had been viewed by then) is persisted, so the
// comparison survives restarts. Pure; informative, never a score.

import type { Finding } from '../types';
import type { ReviewComment } from './comments';
import type { ViewedState } from './viewing';

/** When the AI's findings were first revealed, and what the reviewer had viewed by then */
export interface RevealMoment {
  /** Date.now() at the reveal */
  at: number;
  viewedFunctions: string[];
  viewedFiles: string[];
}

/** Lines either side of a comment that still count as the same spot */
export const OVERLAP_LINES = 3;

export function captureReveal(now: number, viewed: ViewedState): RevealMoment {
  return { at: now, viewedFunctions: Object.keys(viewed.functions), viewedFiles: Object.keys(viewed.files) };
}

/** Comments the reviewer wrote before seeing the AI's findings */
export function commentsBeforeReveal(comments: readonly ReviewComment[], reveal: RevealMoment): ReviewComment[] {
  return comments.filter((comment) => comment.createdAt <= reveal.at);
}

/**
 * True when a comment and a finding point at the same spot: the same file and lines
 * within OVERLAP_LINES of the comment's range. File-level ones match file-level ones.
 */
export function overlaps(comment: Pick<ReviewComment, 'file' | 'line' | 'end_line'>, finding: Pick<Finding, 'file' | 'line' | 'end_line'>): boolean {
  if (comment.file !== finding.file) return false;
  const findingLine = finding.line ?? 0;
  if (!comment.line || !findingLine) return !comment.line && !findingLine;
  const from = comment.line - OVERLAP_LINES;
  const to = (comment.end_line ?? comment.line) + OVERLAP_LINES;
  const findingEnd = finding.end_line && finding.end_line > findingLine ? finding.end_line : findingLine;
  return findingLine <= to && findingEnd >= from;
}

/** Files the reviewer had looked at (or commented on) before the reveal */
export function filesInPass(reveal: RevealMoment, comments: readonly ReviewComment[], fileOfFunction: (id: string) => string | undefined): Set<string> {
  const files = new Set(reveal.viewedFiles);
  for (const id of reveal.viewedFunctions) {
    const file = fileOfFunction(id);
    if (file) files.add(file);
  }
  for (const comment of comments) files.add(comment.file);
  return files;
}

export interface CompareView {
  /** Comments the reviewer wrote before the reveal */
  yours: number;
  /** AI findings */
  ai: number;
  /** AI findings at the same file:line (±3) as one of the reviewer's comments */
  overlap: number;
  overlapIds: ReadonlySet<string>;
  /** AI findings in files the reviewer never viewed before the reveal */
  outsideIds: ReadonlySet<string>;
}

export interface CompareInput {
  reveal: RevealMoment;
  comments: readonly ReviewComment[];
  findings: readonly Finding[];
  fileOfFunction(id: string): string | undefined;
}

export function compareView({ reveal, comments, findings, fileOfFunction }: CompareInput): CompareView {
  const mine = commentsBeforeReveal(comments, reveal);
  const passFiles = filesInPass(reveal, mine, fileOfFunction);
  const overlapIds = new Set<string>();
  const outsideIds = new Set<string>();
  for (const finding of findings) {
    if (mine.some((comment) => overlaps(comment, finding))) overlapIds.add(finding.id);
    else if (!passFiles.has(finding.file)) outsideIds.add(finding.id);
  }
  return { yours: mine.length, ai: findings.length, overlap: overlapIds.size, overlapIds, outsideIds };
}

/** "You flagged 3 things yourself · AI raised 12 · 2 overlap by file:line (±3)" */
export function compareSentence(view: Pick<CompareView, 'yours' | 'ai' | 'overlap'>): string {
  const things = view.yours === 1 ? 'thing' : 'things';
  return `You flagged ${view.yours} ${things} yourself · AI raised ${view.ai} · ${view.overlap} overlap by file:line (±${OVERLAP_LINES})`;
}

const isStringArray = (value: unknown): value is string[] => Array.isArray(value) && value.every((item) => typeof item === 'string');

/** A stored reveal moment, or null when missing or malformed */
export function parseRevealMoment(value: unknown): RevealMoment | null {
  if (typeof value !== 'object' || value === null) return null;
  const record = value as Record<string, unknown>;
  if (typeof record.at !== 'number' || !Number.isFinite(record.at)) return null;
  return {
    at: record.at,
    viewedFunctions: isStringArray(record.viewedFunctions) ? record.viewedFunctions : [],
    viewedFiles: isStringArray(record.viewedFiles) ? record.viewedFiles : [],
  };
}
