// The verify queue (V): one AI finding at a time, to accept, dismiss or skip with single
// keys. Pure ordering and state transitions; the store holds the session in ui.verify.
//
// Order: critical, then warning, then suggestion (praise is not something to verify).
// Within a severity, unreviewed first, then by Kaizen risk, then file and line. The
// order is frozen when the queue opens, so deciding on a finding never reshuffles it.
// Suggestions stay collapsed while any critical or warning finding is unreviewed.

import type { Finding, Severity } from '../types';
import { isUrgent, severityRank } from '../lib/findings';
import type { TriageMap } from './triage';

export const QUEUE_SEVERITIES: readonly Severity[] = ['critical', 'warning', 'suggestion'];

export interface VerifySession {
  /** Finding IDs in queue order, frozen when the queue opened */
  order: string[];
  /** The finding on screen, or null when the queue is empty */
  currentId: string | null;
  /** Suggestions expanded even though urgent findings are unreviewed */
  showNits: boolean;
  /** The dismiss-reason picker is open */
  picking: boolean;
  /** Callers / callees of the enclosing function shown (r) */
  showRelations: boolean;
}

type RiskOf = (finding: Finding) => number;

export function inQueue(finding: Finding): boolean {
  return QUEUE_SEVERITIES.includes(finding.severity);
}

/** Queue order (see the module comment) */
export function verifyOrder(findings: readonly Finding[], triage: TriageMap, riskOf: RiskOf): Finding[] {
  return findings.filter(inQueue).sort((left, right) =>
    severityRank(left.severity) - severityRank(right.severity)
    || Number(Boolean(triage[left.id])) - Number(Boolean(triage[right.id]))
    || riskOf(right) - riskOf(left)
    || left.file.localeCompare(right.file)
    || (left.line ?? 0) - (right.line ?? 0)
    || left.id.localeCompare(right.id));
}

/** True while suggestions should start collapsed: some critical / warning finding is unreviewed */
export function nitsCollapsed(findings: readonly Finding[], triage: TriageMap): boolean {
  return findings.some((finding) => isUrgent(finding) && !triage[finding.id]);
}

/** "3 suggestions (triage the warnings first)" */
export function collapsedNitsLabel(count: number): string {
  return `${count} ${count === 1 ? 'suggestion' : 'suggestions'} (triage the warnings first)`;
}

export function startSession(ordered: readonly Finding[], triage: TriageMap, preferId?: string | null): VerifySession {
  const order = ordered.map((finding) => finding.id);
  const preferred = preferId && order.includes(preferId) ? preferId : null;
  const firstOpen = ordered.find((finding) => !triage[finding.id])?.id ?? null;
  return { order, currentId: preferred ?? firstOpen ?? order[0] ?? null, showNits: false, picking: false, showRelations: false };
}

/** The IDs the queue walks: suggestions left out while they are collapsed */
export function visibleIds(session: VerifySession, findings: Readonly<Record<string, Finding>>, triage: TriageMap): string[] {
  const present = session.order.filter((id) => findings[id]);
  const all = present.map((id) => findings[id]!);
  if (session.showNits || !nitsCollapsed(all, triage)) return present;
  return present.filter((id) => findings[id]!.severity !== 'suggestion');
}

/** Number of suggestions currently collapsed out of the queue */
export function hiddenNitCount(session: VerifySession, findings: Readonly<Record<string, Finding>>, triage: TriageMap): number {
  const present = session.order.filter((id) => findings[id]);
  return present.length - visibleIds(session, findings, triage).length;
}

/** Step through the visible queue; stays put at either end */
export function moveSession(session: VerifySession, visible: readonly string[], delta: number): VerifySession {
  if (visible.length === 0) return { ...session, currentId: null, picking: false };
  const at = session.currentId ? visible.indexOf(session.currentId) : -1;
  const from = at === -1 ? (delta > 0 ? -1 : visible.length) : at;
  const next = Math.max(0, Math.min(visible.length - 1, from + delta));
  return { ...session, currentId: visible[next] ?? null, picking: false };
}

/**
 * After accepting or dismissing: the next unreviewed finding after the current one,
 * else the first unreviewed one before it, else stay (everything is decided).
 */
export function advanceSession(session: VerifySession, visible: readonly string[], triage: TriageMap): VerifySession {
  const at = session.currentId ? visible.indexOf(session.currentId) : -1;
  const after = visible.slice(at + 1).find((id) => !triage[id]);
  const before = visible.slice(0, Math.max(0, at)).find((id) => !triage[id]);
  const currentId = after ?? before ?? session.currentId ?? visible[0] ?? null;
  return { ...session, currentId, picking: false };
}

/** Keep the frozen order in step with the findings: drop withdrawn ones, append new ones */
export function mergeOrder(session: VerifySession, ordered: readonly Finding[]): VerifySession {
  const ids = new Set(ordered.map((finding) => finding.id));
  const kept = session.order.filter((id) => ids.has(id));
  const known = new Set(kept);
  const added = ordered.map((finding) => finding.id).filter((id) => !known.has(id));
  if (added.length === 0 && kept.length === session.order.length) return session;
  const order = [...kept, ...added];
  const currentId = session.currentId && ids.has(session.currentId) ? session.currentId : order[0] ?? null;
  return { ...session, order, currentId };
}

export interface QueueProgress {
  position: number;
  total: number;
  accepted: number;
  dismissed: number;
  /** 0-1: decided share of the visible queue */
  done: number;
}

export function queueProgress(session: VerifySession, visible: readonly string[], triage: TriageMap): QueueProgress {
  let accepted = 0;
  let dismissed = 0;
  for (const id of visible) {
    if (triage[id]?.state === 'accepted') accepted++;
    if (triage[id]?.state === 'dismissed') dismissed++;
  }
  const at = session.currentId ? visible.indexOf(session.currentId) : -1;
  return {
    position: at + 1, total: visible.length, accepted, dismissed,
    done: visible.length ? (accepted + dismissed) / visible.length : 0,
  };
}

/** "7 of 21 · 3 accepted · 2 dismissed" */
export function queueSentence(progress: QueueProgress): string {
  return `${progress.position} of ${progress.total} · ${progress.accepted} accepted · ${progress.dismissed} dismissed`;
}
