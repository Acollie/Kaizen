import { describe, expect, it } from 'vitest';
import type { Finding } from '../types';
import { makeContext, makeFinding } from '../components/post/testFixtures';
import { createReviewStore } from './store';
import type { TriageMap } from './triage';
import {
  advanceSession, collapsedNitsLabel, hiddenNitCount, mergeOrder, moveSession, nitsCollapsed, queueProgress,
  queueSentence, startSession, verifyOrder, visibleIds,
} from './verifyQueue';

const byId = (findings: Finding[]) => Object.fromEntries(findings.map((finding) => [finding.id, finding]));
const ids = (findings: Finding[]) => findings.map((finding) => finding.id);

const RISK: Record<string, number> = { 'pkg/hot.go': 80, 'pkg/warm.go': 30, 'pkg/cold.go': 5 };
const riskOf = (finding: Finding) => RISK[finding.file] ?? 0;

const findings = [
  makeFinding({ id: 's1', severity: 'suggestion', file: 'pkg/hot.go' }),
  makeFinding({ id: 'w-cold', severity: 'warning', file: 'pkg/cold.go' }),
  makeFinding({ id: 'c1', severity: 'critical', file: 'pkg/cold.go' }),
  makeFinding({ id: 'w-hot', severity: 'warning', file: 'pkg/hot.go' }),
  makeFinding({ id: 'p1', severity: 'praise', file: 'pkg/hot.go' }),
  makeFinding({ id: 'w-warm', severity: 'warning', file: 'pkg/warm.go' }),
];

describe('verify queue order', () => {
  it('goes critical, warning, suggestion; riskiest first within a severity; no praise', () => {
    expect(ids(verifyOrder(findings, {}, riskOf))).toEqual(['c1', 'w-hot', 'w-warm', 'w-cold', 's1']);
  });

  it('puts unreviewed findings before decided ones within a severity', () => {
    const triage: TriageMap = { 'w-hot': { state: 'accepted' }, c1: { state: 'dismissed', reason: "Won't fix" } };
    expect(ids(verifyOrder(findings, triage, riskOf))).toEqual(['c1', 'w-warm', 'w-cold', 'w-hot', 's1']);
  });
});

describe('verify queue transitions', () => {
  const ordered = verifyOrder(findings, {}, riskOf);
  const all = byId(findings);

  it('starts at the preferred finding, else the first unreviewed one', () => {
    expect(startSession(ordered, {}, 'w-warm').currentId).toBe('w-warm');
    expect(startSession(ordered, { c1: { state: 'accepted' } }).currentId).toBe('w-hot');
    expect(startSession([], {}).currentId).toBeNull();
  });

  it('collapses suggestions while urgent findings are unreviewed', () => {
    const session = startSession(ordered, {});
    expect(nitsCollapsed(findings, {})).toBe(true);
    expect(visibleIds(session, all, {})).toEqual(['c1', 'w-hot', 'w-warm', 'w-cold']);
    expect(hiddenNitCount(session, all, {})).toBe(1);
    expect(visibleIds({ ...session, showNits: true }, all, {})).toContain('s1');
    const decided: TriageMap = { c1: { state: 'accepted' }, 'w-hot': { state: 'accepted' }, 'w-warm': { state: 'dismissed' }, 'w-cold': { state: 'accepted' } };
    expect(visibleIds(session, all, decided)).toContain('s1');
    expect(collapsedNitsLabel(3)).toBe('3 suggestions (triage the warnings first)');
  });

  it('moves with j/k and stops at the ends', () => {
    const visible = ['c1', 'w-hot', 'w-warm'];
    let session = startSession(ordered, {});
    session = moveSession(session, visible, 1);
    expect(session.currentId).toBe('w-hot');
    session = moveSession(session, visible, 5);
    expect(session.currentId).toBe('w-warm');
    session = moveSession(session, visible, -9);
    expect(session.currentId).toBe('c1');
  });

  it('advances to the next unreviewed finding after a decision, wrapping to earlier ones', () => {
    const visible = ['c1', 'w-hot', 'w-warm', 'w-cold'];
    const session = { ...startSession(ordered, {}), currentId: 'w-hot', picking: true };
    const triage: TriageMap = { 'w-hot': { state: 'accepted' }, 'w-warm': { state: 'accepted' } };
    const next = advanceSession(session, visible, triage);
    expect(next.currentId).toBe('w-cold');
    expect(next.picking).toBe(false);
    const atEnd = advanceSession({ ...session, currentId: 'w-cold' }, visible, { ...triage, 'w-cold': { state: 'dismissed' } });
    expect(atEnd.currentId).toBe('c1');
    const allDone: TriageMap = { c1: { state: 'accepted' }, 'w-hot': { state: 'accepted' }, 'w-warm': { state: 'accepted' }, 'w-cold': { state: 'accepted' } };
    expect(advanceSession({ ...session, currentId: 'w-cold' }, visible, allDone).currentId).toBe('w-cold');
  });

  it('keeps its frozen order when findings arrive or are withdrawn', () => {
    const session = startSession(ordered, {});
    const extra = makeFinding({ id: 'c2', severity: 'critical', file: 'pkg/hot.go' });
    const merged = mergeOrder(session, [extra, ...ordered.filter((finding) => finding.id !== 'w-cold')]);
    expect(merged.order).toEqual(['c1', 'w-hot', 'w-warm', 's1', 'c2']);
    expect(mergeOrder(merged, [extra, ...ordered.filter((finding) => finding.id !== 'w-cold')])).toBe(merged);
  });

  it('reports progress as "7 of 21 · 3 accepted · 2 dismissed"', () => {
    const visible = ['c1', 'w-hot', 'w-warm'];
    const session = { ...startSession(ordered, {}), currentId: 'w-hot' };
    const progress = queueProgress(session, visible, { c1: { state: 'accepted' }, 'w-warm': { state: 'dismissed' } });
    expect(queueSentence(progress)).toBe('2 of 3 · 1 accepted · 1 dismissed');
    expect(progress.done).toBeCloseTo(2 / 3);
  });
});

describe('verify queue in the store', () => {
  function store() {
    const context = makeContext();
    const reviewStore = createReviewStore(context);
    const state = reviewStore.getState();
    state.applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'w1', severity: 'warning', line: 2 }) } as never);
    state.applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 'c1', severity: 'critical', line: 3 }) } as never);
    state.applyEvent({ seq: 3, type: 'finding', data: makeFinding({ id: 's1', severity: 'suggestion', line: 1 }) } as never);
    return reviewStore;
  }

  it('does not open while Think first hides the findings', () => {
    const reviewStore = store();
    reviewStore.getState().setThinkFirst(true);
    reviewStore.getState().openVerify();
    expect(reviewStore.getState().ui.verify).toBeNull();
    reviewStore.getState().revealFindings();
    reviewStore.getState().openVerify();
    expect(reviewStore.getState().ui.verify?.currentId).toBe('c1');
  });

  it('accepts, dismisses with a reason, skips and exits', () => {
    const reviewStore = store();
    const get = () => reviewStore.getState();
    get().openVerify();
    get().verifyAccept();
    expect(get().triage.c1).toEqual({ state: 'accepted' });
    expect(get().ui.verify?.currentId).toBe('w1');
    get().verifyStartDismiss();
    expect(get().ui.verify?.picking).toBe(true);
    get().verifyCancelDismiss();
    expect(get().ui.verify?.picking).toBe(false);
    get().verifyStartDismiss();
    get().verifyDismiss('False positive');
    expect(get().triage.w1).toEqual({ state: 'dismissed', reason: 'False positive' });
    // Warnings are all decided: the suggestion is in the queue now
    expect(get().ui.verify?.currentId).toBe('s1');
    get().verifyMove(-1);
    expect(get().ui.verify?.currentId).toBe('w1');
    get().closeVerify();
    expect(get().ui.verify).toBeNull();
  });
});
