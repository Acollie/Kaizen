// Pure triage logic. The developer decides what happens to each AI finding: accept it
// (it will be posted as their review comment) or dismiss it with a reason. Nothing is
// accepted by default; an untouched finding is "unreviewed".

import type { Finding } from '../types';
import { isUrgent } from '../lib/findings';

export type TriageState = 'accepted' | 'dismissed';
export type TriageStatus = TriageState | 'unreviewed';

export interface TriageEntry {
  state: TriageState;
  /** Why it was dismissed (one of DISMISS_REASONS) */
  reason?: string;
  /** The reviewer's own words; appended to an accepted finding's comment */
  note?: string;
}

export type TriageMap = Readonly<Record<string, TriageEntry>>;

export const DISMISS_REASONS = ['False positive', "Won't fix", 'Already handled', 'Not worth it'] as const;

export type TriageFilter = 'unreviewed' | 'accepted' | 'dismissed' | 'all';
export const TRIAGE_FILTERS: readonly TriageFilter[] = ['unreviewed', 'accepted', 'dismissed', 'all'];

export function triageStatus(triage: TriageMap, id: string): TriageStatus {
  return triage[id]?.state ?? 'unreviewed';
}

export function isAccepted(triage: TriageMap, id: string): boolean {
  return triage[id]?.state === 'accepted';
}

export function acceptedFindings(findings: readonly Finding[], triage: TriageMap): Finding[] {
  return findings.filter((finding) => isAccepted(triage, finding.id));
}

/** Urgent (critical / warning) findings the reviewer has not decided on yet */
export function untriagedUrgent(findings: readonly Finding[], triage: TriageMap): Finding[] {
  return findings.filter((finding) => isUrgent(finding) && !triage[finding.id]);
}

export interface TriageProgress {
  total: number;
  triaged: number;
  accepted: number;
  dismissed: number;
  unreviewed: number;
}

export function triageProgress(findings: readonly Finding[], triage: TriageMap): TriageProgress {
  let accepted = 0;
  let dismissed = 0;
  for (const finding of findings) {
    const status = triageStatus(triage, finding.id);
    if (status === 'accepted') accepted++;
    if (status === 'dismissed') dismissed++;
  }
  const triaged = accepted + dismissed;
  return { total: findings.length, triaged, accepted, dismissed, unreviewed: findings.length - triaged };
}

/** "3 of 9 triaged · 2 accepted" */
export function progressSentence(progress: TriageProgress): string {
  if (progress.total === 0) return 'No findings to triage';
  return `${progress.triaged} of ${progress.total} triaged · ${progress.accepted} accepted`;
}

/** Unreviewed while anything is left to decide, otherwise everything */
export function defaultFilter(progress: TriageProgress): TriageFilter {
  return progress.unreviewed > 0 ? 'unreviewed' : 'all';
}

export function matchesTriageFilter(status: TriageStatus, filter: TriageFilter): boolean {
  return filter === 'all' || status === filter;
}

export function filterCount(progress: TriageProgress, filter: TriageFilter): number {
  if (filter === 'all') return progress.total;
  return progress[filter];
}

/** A trimmed note, or undefined when blank */
export function cleanNote(note: string | undefined): string | undefined {
  const trimmed = (note ?? '').trim();
  return trimmed === '' ? undefined : trimmed;
}

/** The finding as it will be posted: the reviewer's note appended to its body */
export function withReviewerNote(finding: Finding, note: string | undefined): Finding {
  const text = cleanNote(note);
  if (!text) return finding;
  const body = finding.body.trim();
  return { ...finding, body: `${body}${body ? '\n\n' : ''}**Reviewer note:** ${text}` };
}

/** "Dismissed · False positive" */
export function dismissedLabel(entry: TriageEntry | undefined): string {
  return ['Dismissed', entry?.reason].filter(Boolean).join(' · ');
}
