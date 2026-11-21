// The lead reviewer's final result is authoritative: live findings it leaves out are
// withdrawn, kept out of every active list and of posting, but still listed.
import { describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach } from 'vitest';
import { createElement } from 'react';
import { createReviewStore, findingsList, type ReviewStore } from './store';
import { StoreProvider } from './StoreProvider';
import { makeContext, makeFile, makeFinding } from '../components/post/testFixtures';
import { planPost } from '../components/post/postLogic';
import { commentsList } from './comments';
import { WithdrawnSection } from '../components/review/WithdrawnSection';
import type { Finding, ReviewEvent } from '../types';

afterEach(cleanup);

const A = makeFinding({ id: 'A', title: 'Finding A', source: 'hotspot-reviewer' });
const B = makeFinding({ id: 'B', title: 'Finding B', line: 2, source: 'impact-analyst' });
const C = makeFinding({ id: 'C', title: 'Finding C' });

function findingEvent(seq: number, finding: Finding): ReviewEvent {
  return { seq, type: 'finding', data: finding } as ReviewEvent;
}

function resultEvent(seq: number, findings: Finding[]): ReviewEvent {
  return { seq, type: 'result', data: { summary: 's', verdict: 'comment', findings } } as unknown as ReviewEvent;
}

/** A live run or a cache replay: the live findings, then the lead's result */
function play(store: ReviewStore, events: ReviewEvent[]) {
  for (const event of events) store.getState().applyEvent(event);
}

const SEQUENCE = [findingEvent(1, A), findingEvent(2, B), findingEvent(3, C), resultEvent(4, [A, C])];

function postPlan(store: ReviewStore) {
  const state = store.getState();
  return planPost({
    comments: commentsList(state.comments),
    findings: findingsList(state),
    triage: state.triage,
    files: new Map([['pkg/a.go', makeFile()]]),
  });
}

describe('findings withdrawn by the lead reviewer', () => {
  it('withdraws a live finding the result leaves out and excludes it from posting', () => {
    const store = createReviewStore(makeContext());
    play(store, SEQUENCE.slice(0, 3));
    store.getState().acceptFinding('B', 'looked real');
    store.getState().acceptFinding('A');
    play(store, SEQUENCE.slice(3));

    const state = store.getState();
    expect(findingsList(state).map((finding) => finding.id)).toEqual(['A', 'C']);
    expect(state.withdrawnOrder).toEqual(['B']);
    expect(state.withdrawn.B?.title).toBe('Finding B');
    // The reviewer's triage for B is kept, but B is never posted
    expect(state.triage.B).toEqual({ state: 'accepted', note: 'looked real' });
    expect(postPlan(store).accepted.map((entry) => entry.finding.id)).toEqual(['A']);
  });

  it('behaves the same on a cache replay into a fresh store', () => {
    const live = createReviewStore(makeContext());
    play(live, SEQUENCE);
    const replay = createReviewStore(makeContext());
    play(replay, [...SEQUENCE, { seq: 5, type: 'done', data: {} } as unknown as ReviewEvent]);
    expect(findingsList(replay.getState()).map((finding) => finding.id)).toEqual(findingsList(live.getState()).map((finding) => finding.id));
    expect(replay.getState().withdrawnOrder).toEqual(['B']);
  });

  it('restores a withdrawn finding if a later result lists it again', () => {
    const store = createReviewStore(makeContext());
    play(store, [...SEQUENCE, resultEvent(5, [A, B, C])]);
    expect(findingsList(store.getState()).map((finding) => finding.id).sort()).toEqual(['A', 'B', 'C']);
    expect(store.getState().withdrawnOrder).toEqual([]);
  });

  it('adds result-only findings and leaves the reviewer\'s own comments untouched', () => {
    const store = createReviewStore(makeContext());
    store.getState().saveComment({ file: 'pkg/a.go', line: 2, body: 'Mine' });
    const D = makeFinding({ id: 'D', title: 'Only in the result' });
    play(store, [...SEQUENCE.slice(0, 3), resultEvent(4, [A, D])]);
    const state = store.getState();
    expect(findingsList(state).map((finding) => finding.id)).toEqual(['A', 'D']);
    expect(state.withdrawnOrder).toEqual(['B', 'C']);
    expect(commentsList(state.comments).map((comment) => comment.body)).toEqual(['Mine']);
    expect(postPlan(store).comments).toHaveLength(1);
  });

  it('never withdraws a finding with a c- id or source "you"', () => {
    const store = createReviewStore(makeContext());
    play(store, [findingEvent(1, makeFinding({ id: 'c-1', source: 'you' })), findingEvent(2, A), resultEvent(3, [])]);
    expect(findingsList(store.getState()).map((finding) => finding.id)).toEqual(['c-1']);
    expect(store.getState().withdrawnOrder).toEqual(['A']);
  });

  it('lists withdrawn findings in a collapsed section with title, location and agent', () => {
    const store = createReviewStore(makeContext());
    play(store, SEQUENCE);
    render(createElement(StoreProvider, { store, children: createElement(WithdrawnSection) }));
    const head = screen.getByText('Withdrawn by the lead reviewer (1)');
    expect(screen.queryByText('Finding B')).toBeNull();
    fireEvent.click(head);
    expect(screen.getByText('Finding B')).toBeTruthy();
    expect(screen.getByText('pkg/a.go:L2')).toBeTruthy();
    expect(screen.getByText('impact-analyst')).toBeTruthy();
  });
});
