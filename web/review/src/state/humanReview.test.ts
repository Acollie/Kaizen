// The human side of the review: triage, viewed tracking, Think first, the reviewer's
// own comments and their persistence.
import { describe, expect, it } from 'vitest';
import { bindPersistence, createReviewStore, findingsHidden } from './store';
import { makeContext, makeFinding } from '../components/post/testFixtures';
import {
  defaultFilter, dismissedLabel, matchesTriageFilter, progressSentence, triageProgress, untriagedUrgent, withReviewerNote,
} from './triage';
import {
  aiSkippedFunction, allFocusSeen, attentionFiles, lookProgress, lookSentence, nextUnseen, withViewed,
} from './viewing';
import { commentLocation, commentToFinding, isUserComment, newCommentId, normalizeRange } from './comments';
import { loadReview, reviewStorageKey, THINK_FIRST_KEY, type KeyValueStorage } from './persistence';
import { accumulateDwell } from '../components/code/useViewTracking';
import type { FileDiff, FunctionRisk } from '../types';

function memoryStorage(): KeyValueStorage & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return { data, getItem: (key) => data.get(key) ?? null, setItem: (key, value) => { data.set(key, value); } };
}

function storeWithFindings() {
  const store = createReviewStore(makeContext());
  store.getState().applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'w', severity: 'warning' }) } as never);
  store.getState().applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 's', severity: 'suggestion' }) } as never);
  return store;
}

describe('triage in the store', () => {
  it('starts every finding unreviewed (nothing is pre-accepted)', () => {
    expect(storeWithFindings().getState().triage).toEqual({});
  });

  it('accepts with a note, dismisses with a reason, and undoes', () => {
    const store = storeWithFindings();
    store.getState().acceptFinding('w', '  checked it  ');
    store.getState().startDismiss('s', 'code');
    store.getState().dismissFinding('s', 'False positive', '');
    expect(store.getState().triage).toEqual({
      w: { state: 'accepted', note: 'checked it' },
      s: { state: 'dismissed', reason: 'False positive' },
    });
    expect(store.getState().ui.dismissing).toBeNull();
    store.getState().undoTriage('w');
    expect(store.getState().triage.w).toBeUndefined();
  });

  it('toggles accept and keeps the note when re-accepted', () => {
    const store = storeWithFindings();
    store.getState().toggleAccept('w');
    store.getState().setTriageNote('w', 'mine');
    store.getState().acceptFinding('w');
    expect(store.getState().triage.w).toEqual({ state: 'accepted', note: 'mine' });
    store.getState().toggleAccept('w');
    expect(store.getState().triage.w).toBeUndefined();
  });
});

describe('triage helpers', () => {
  const findings = [makeFinding({ id: 'a', severity: 'critical' }), makeFinding({ id: 'b' }), makeFinding({ id: 'c', severity: 'suggestion' })];
  const triage = { a: { state: 'accepted' as const }, c: { state: 'dismissed' as const, reason: "Won't fix" } };

  it('counts progress', () => {
    const progress = triageProgress(findings, triage);
    expect(progress).toEqual({ total: 3, triaged: 2, accepted: 1, dismissed: 1, unreviewed: 1 });
    expect(progressSentence(progress)).toBe('2 of 3 triaged · 1 accepted');
    expect(defaultFilter(progress)).toBe('unreviewed');
    expect(defaultFilter(triageProgress(findings.slice(0, 1), triage))).toBe('all');
  });

  it('filters, finds untriaged urgent findings and labels dismissals', () => {
    expect(matchesTriageFilter('accepted', 'all')).toBe(true);
    expect(matchesTriageFilter('accepted', 'dismissed')).toBe(false);
    expect(untriagedUrgent(findings, triage).map((finding) => finding.id)).toEqual(['b']);
    expect(dismissedLabel(triage.c)).toBe("Dismissed · Won't fix");
  });

  it('appends the reviewer note', () => {
    expect(withReviewerNote(makeFinding({ id: 'x', body: 'Body' }), ' ok ').body).toBe('Body\n\n**Reviewer note:** ok');
    expect(withReviewerNote(makeFinding({ id: 'x', body: 'Body' }), ' ').body).toBe('Body');
  });
});

describe('viewed tracking', () => {
  const focusA = { id: 'A', focus: true, risk_score: 50 } as FunctionRisk;
  const focusB = { id: 'B', focus: true, risk_score: 40 } as FunctionRisk;
  const file = { path: 'pkg/a.go' } as FileDiff;

  it('markViewed records functions and files once', () => {
    const store = storeWithFindings();
    store.getState().markViewed('function', 'A');
    const viewed = store.getState().viewed;
    store.getState().markViewed('function', 'A');
    expect(store.getState().viewed).toBe(viewed);
    store.getState().markViewed('file', 'pkg/a.go');
    expect(store.getState().viewed).toEqual({ functions: { A: true }, files: { 'pkg/a.go': true } });
  });

  it('summarises what was looked at and what to look at next', () => {
    const progress = lookProgress([focusA, focusB], [file], withViewed({ functions: {}, files: {} }, 'function', 'A'));
    expect(lookSentence(progress)).toBe("You've looked at 1/2 focus functions · 0/1 file needing attention");
    expect(nextUnseen(progress)).toEqual({ kind: 'function', id: 'B' });
    expect(allFocusSeen(progress)).toBe(false);
    const done = lookProgress([focusA], [file], { functions: { A: true }, files: {} });
    expect(nextUnseen(done)).toEqual({ kind: 'file', path: 'pkg/a.go' });
    expect(allFocusSeen(done)).toBe(true);
  });

  it('files needing attention: urgent findings not dismissed, plus flagged files without functions', () => {
    const files = [
      { path: 'a.go', risk_reasons: [] }, { path: 'ci.yml', risk_reasons: ['workflow'] }, { path: 'b.go', risk_reasons: [] },
    ] as unknown as FileDiff[];
    const findings = [makeFinding({ id: '1', file: 'a.go' }), makeFinding({ id: '2', file: 'b.go' })];
    const paths = attentionFiles({ files, functionsByFile: new Map(), findings, triage: { 2: { state: 'dismissed' } } }).map((entry) => entry.path);
    expect(paths).toEqual(['a.go', 'ci.yml']);
  });

  it('accumulates on-screen time per target and restarts on a new target', () => {
    let dwell = accumulateDwell({ id: null, elapsed: 0 }, 'A', true, 250);
    dwell = accumulateDwell(dwell, 'A', true, 250);
    dwell = accumulateDwell(dwell, 'A', false, 250);
    expect(dwell).toEqual({ id: 'A', elapsed: 500 });
    expect(accumulateDwell(dwell, 'B', false, 250)).toEqual({ id: 'B', elapsed: 0 });
  });

  it('flags functions the finished AI review never opened', () => {
    const coverage = { examinedFunctions: new Set(['A']), examinedFiles: new Set<string>(), complete: true, known: true };
    expect(aiSkippedFunction(coverage, 'A')).toBe(false);
    expect(aiSkippedFunction(coverage, 'B')).toBe(true);
    expect(aiSkippedFunction({ ...coverage, complete: false }, 'B')).toBe(false);
    expect(aiSkippedFunction({ ...coverage, known: false }, 'B')).toBe(false);
  });
});

describe('Think first', () => {
  it('hides findings until revealed; turning it on again hides them again', () => {
    const store = storeWithFindings();
    expect(findingsHidden(store.getState())).toBe(false);
    store.getState().toggleThinkFirst();
    expect(findingsHidden(store.getState())).toBe(true);
    store.getState().revealFindings();
    expect(findingsHidden(store.getState())).toBe(false);
    store.getState().setThinkFirst(true);
    expect(findingsHidden(store.getState())).toBe(true);
  });
});

describe('your comments', () => {
  it('saves, edits and deletes comments, closing the composer', () => {
    const store = storeWithFindings();
    store.getState().openComposer({ file: 'pkg/a.go', line: 3 });
    const id = store.getState().saveComment({ file: 'pkg/a.go', line: 3, end_line: 1, body: ' Why? ' });
    expect(store.getState().ui.composer).toBeNull();
    expect(store.getState().comments[id]).toMatchObject({ file: 'pkg/a.go', line: 1, end_line: 3, body: 'Why?' });
    store.getState().saveComment({ file: 'pkg/a.go', line: 1, end_line: 3, body: 'Edited', severity: 'warning' }, id);
    expect(Object.keys(store.getState().comments)).toEqual([id]);
    expect(store.getState().comments[id]?.severity).toBe('warning');
    store.getState().openFinding(id);
    expect(store.getState().selection.findingId).toBe(id);
    store.getState().deleteComment(id);
    expect(store.getState().comments).toEqual({});
    expect(store.getState().selection.findingId).toBeNull();
  });

  it('converts to a wire finding from "you"', () => {
    const finding = commentToFinding({ id: 'c-1', file: 'a.go', line: 0, body: 'Overall', createdAt: 1 });
    expect(finding).toEqual({
      id: 'c-1', severity: 'suggestion', category: 'design', file: 'a.go', title: '', body: 'Overall', confidence: 1, source: 'you',
    });
    expect(isUserComment(finding)).toBe(true);
    expect(isUserComment(makeFinding({ id: 'c-2' }))).toBe(false);
  });

  it('normalises ranges, formats locations and makes unique ids', () => {
    expect(normalizeRange(5, 2)).toEqual({ line: 2, end_line: 5 });
    expect(normalizeRange(5, 5)).toEqual({ line: 5 });
    expect(commentLocation({ line: 2, end_line: 5 })).toBe('L2–5');
    expect(commentLocation({ line: 0 })).toBe('file');
    const first = newCommentId(1000, {});
    expect(first.startsWith('c-')).toBe(true);
    expect(newCommentId(1000, { [first]: {} as never })).not.toBe(first);
  });
});

describe('persistence', () => {
  it('restores triage, viewed, comments and reveal for the same PR and head SHA', () => {
    const storage = memoryStorage();
    const first = storeWithFindings();
    bindPersistence(first, storage);
    first.getState().acceptFinding('w', 'note');
    first.getState().markViewed('file', 'pkg/a.go');
    first.getState().saveComment({ file: 'pkg/a.go', line: 2, body: 'Mine' });
    first.getState().setThinkFirst(true);
    first.getState().revealFindings();

    const second = storeWithFindings();
    bindPersistence(second, storage);
    const state = second.getState();
    expect(state.triage).toEqual({ w: { state: 'accepted', note: 'note' } });
    expect(state.viewed.files).toEqual({ 'pkg/a.go': true });
    expect(Object.values(state.comments).map((comment) => comment.body)).toEqual(['Mine']);
    expect(state.ui.thinkFirst).toBe(true);
    expect(state.revealed).toBe(true);
    expect(state.reveal?.viewedFiles).toEqual(['pkg/a.go']);
  });

  it('turns Think first on by default, and remembers an explicit off', () => {
    const storage = memoryStorage();
    const first = storeWithFindings();
    bindPersistence(first, storage);
    expect(first.getState().ui.thinkFirst).toBe(true);
    expect(findingsHidden(first.getState())).toBe(true);
    first.getState().toggleThinkFirst();
    expect(storage.data.get(THINK_FIRST_KEY)).toBe('false');

    const second = storeWithFindings();
    bindPersistence(second, storage);
    expect(second.getState().ui.thinkFirst).toBe(false);
  });

  it('records the first reveal moment with what was viewed by then', () => {
    const store = storeWithFindings();
    store.getState().setThinkFirst(true);
    store.getState().markViewed('function', 'pkg/a.go:F');
    store.getState().revealFindings();
    const reveal = store.getState().reveal;
    expect(reveal?.viewedFunctions).toEqual(['pkg/a.go:F']);
    expect(typeof reveal?.at).toBe('number');
    store.getState().markViewed('file', 'pkg/b.go');
    store.getState().setThinkFirst(false);
    store.getState().setThinkFirst(true);
    store.getState().revealFindings();
    expect(store.getState().reveal).toBe(reveal);
  });

  it('starts fresh on a new head SHA', () => {
    const storage = memoryStorage();
    const first = storeWithFindings();
    bindPersistence(first, storage);
    first.getState().acceptFinding('w');
    const pushed = makeContext();
    pushed.pr.head_sha = 'new-sha';
    const second = createReviewStore(pushed);
    bindPersistence(second, storage);
    expect(second.getState().triage).toEqual({});
    expect(reviewStorageKey(pushed.pr)).toBe('kaizen-review:o/r#7@new-sha');
  });

  it('survives broken storage and bad data', () => {
    const throwing: KeyValueStorage = { getItem: () => { throw new Error('denied'); }, setItem: () => { throw new Error('quota'); } };
    const store = storeWithFindings();
    expect(() => bindPersistence(store, throwing)).not.toThrow();
    expect(() => store.getState().acceptFinding('w')).not.toThrow();
    const storage = memoryStorage();
    storage.setItem('k', '{not json');
    expect(loadReview(storage, 'k')).toBeNull();
    storage.setItem('k', JSON.stringify({ version: 1, triage: { a: { state: 'bogus' }, b: { state: 'dismissed', reason: 'Not worth it' } } }));
    expect(loadReview(storage, 'k')?.triage).toEqual({ b: { state: 'dismissed', reason: 'Not worth it' } });
  });
});
