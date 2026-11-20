// Keeps the human's work across reloads: triage decisions and notes, what they viewed,
// their own comments and whether they revealed the AI's findings. Keyed by PR and head
// SHA, so a new push starts fresh. The "Think first" preference is global (a habit, not
// a per-PR choice). Storage can be missing or throw (private mode, quota): every access
// is guarded and the UI works without it.

import type { PullRequest } from '../types';
import type { CommentMap, ReviewComment } from './comments';
import type { TriageEntry, TriageMap } from './triage';
import { EMPTY_VIEWED, type ViewedState } from './viewing';

export const THINK_FIRST_KEY = 'kaizen-review-think-first';
const VERSION = 1;

export interface PersistedReview {
  triage: TriageMap;
  viewed: ViewedState;
  comments: CommentMap;
  revealed: boolean;
}

export type KeyValueStorage = Pick<Storage, 'getItem' | 'setItem'>;

export function reviewStorageKey(pr: Pick<PullRequest, 'owner' | 'repo' | 'number' | 'head_sha'>): string {
  return `kaizen-review:${pr.owner}/${pr.repo}#${pr.number}@${pr.head_sha}`;
}

/** window.localStorage, or null where it is unavailable */
export function safeLocalStorage(): KeyValueStorage | null {
  try {
    return typeof window !== 'undefined' && window.localStorage ? window.localStorage : null;
  } catch {
    return null;
  }
}

function readJson(storage: KeyValueStorage | null, key: string): unknown {
  if (!storage) return null;
  try {
    const raw = storage.getItem(key);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function writeJson(storage: KeyValueStorage | null, key: string, value: unknown): void {
  if (!storage) return;
  try {
    storage.setItem(key, JSON.stringify(value));
  } catch {
    // Quota or privacy mode: the review still works, it just won't survive a reload
  }
}

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

function parseTriage(value: unknown): TriageMap {
  if (!isObject(value)) return {};
  const triage: Record<string, TriageEntry> = {};
  for (const [id, entry] of Object.entries(value)) {
    if (!isObject(entry) || (entry.state !== 'accepted' && entry.state !== 'dismissed')) continue;
    triage[id] = {
      state: entry.state,
      ...(typeof entry.reason === 'string' ? { reason: entry.reason } : {}),
      ...(typeof entry.note === 'string' ? { note: entry.note } : {}),
    };
  }
  return triage;
}

function parseIdSet(value: unknown): Record<string, true> {
  if (!Array.isArray(value)) return {};
  return Object.fromEntries(value.filter((id): id is string => typeof id === 'string').map((id) => [id, true as const]));
}

function parseComments(value: unknown): CommentMap {
  if (!isObject(value)) return {};
  const comments: Record<string, ReviewComment> = {};
  for (const [id, entry] of Object.entries(value)) {
    if (!isObject(entry) || typeof entry.file !== 'string' || typeof entry.body !== 'string') continue;
    comments[id] = { ...(entry as unknown as ReviewComment), id, line: Number(entry.line) || 0 };
  }
  return comments;
}

export function loadReview(storage: KeyValueStorage | null, key: string): PersistedReview | null {
  const data = readJson(storage, key);
  if (!isObject(data) || data.version !== VERSION) return null;
  const viewed = isObject(data.viewed) ? data.viewed : {};
  return {
    triage: parseTriage(data.triage),
    viewed: { functions: parseIdSet(viewed.functions), files: parseIdSet(viewed.files) },
    comments: parseComments(data.comments),
    revealed: data.revealed === true,
  };
}

export function saveReview(storage: KeyValueStorage | null, key: string, review: PersistedReview): void {
  writeJson(storage, key, {
    version: VERSION,
    triage: review.triage,
    viewed: { functions: Object.keys(review.viewed.functions), files: Object.keys(review.viewed.files) },
    comments: review.comments,
    revealed: review.revealed,
  });
}

export function loadThinkFirst(storage: KeyValueStorage | null): boolean {
  return readJson(storage, THINK_FIRST_KEY) === true;
}

export function saveThinkFirst(storage: KeyValueStorage | null, enabled: boolean): void {
  writeJson(storage, THINK_FIRST_KEY, enabled);
}

export const EMPTY_REVIEW: PersistedReview = { triage: {}, viewed: EMPTY_VIEWED, comments: {}, revealed: false };
