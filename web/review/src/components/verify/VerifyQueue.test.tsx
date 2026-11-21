import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { createReviewStore } from '../../state/store';
import { StoreProvider } from '../../state/StoreProvider';
import * as api from '../../lib/api';
import { makeContext, makeFinding } from '../post/testFixtures';
import { VerifyQueue } from './VerifyQueue';

afterEach(() => vi.restoreAllMocks());

function setup() {
  vi.spyOn(api, 'fetchFileLines').mockResolvedValue({ lines: [], total: 3 });
  const store = createReviewStore(makeContext());
  act(() => {
    const state = store.getState();
    state.applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'w1', severity: 'warning', title: 'Warn one', verify: 'Run go test ./pkg/a', confidence: 0.7 }) } as never);
    state.applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 'c1', severity: 'critical', title: 'Crit one' }) } as never);
    state.applyEvent({ seq: 3, type: 'finding', data: makeFinding({ id: 's1', severity: 'suggestion', category: 'design', title: 'Nit one' }) } as never);
  });
  render(<StoreProvider store={store}><VerifyQueue /></StoreProvider>);
  act(() => store.getState().openVerify());
  return store;
}

const press = (key: string, init: KeyboardEventInit = {}) => act(() => { fireEvent.keyDown(document, { key, ...init }); });

describe('VerifyQueue', () => {
  it('shows one finding with severity, how to verify and the code, driven by keys', () => {
    const store = setup();
    expect(screen.getByText('Crit one')).toBeTruthy();
    expect(screen.getByText('1 of 2 · 0 accepted · 0 dismissed')).toBeTruthy();
    expect(screen.getByText('1 suggestion (triage the warnings first)', { exact: false })).toBeTruthy();
    expect(screen.getByLabelText('Code').textContent).toContain('var x = 1');

    press('a');
    expect(store.getState().triage.c1?.state).toBe('accepted');
    expect(screen.getByText('Warn one')).toBeTruthy();
    expect(screen.getByText('Run go test ./pkg/a')).toBeTruthy();
    expect(screen.getByText('medium conf.')).toBeTruthy();

    press('d');
    expect(screen.getByRole('group', { name: 'Dismiss reason' })).toBeTruthy();
    press('2');
    expect(store.getState().triage.w1).toEqual({ state: 'dismissed', reason: "Won't fix" });
    // Warnings are done: the suggestion joins the queue
    expect(screen.getByText('Nit one')).toBeTruthy();

    press('k');
    expect(screen.getByText('Warn one')).toBeTruthy();
    press('Enter');
    expect(screen.getByText('Nit one')).toBeTruthy();
    press('Escape');
    expect(store.getState().ui.verify).toBeNull();
  });

  it('opens a comment composer with c and saves it as your comment', () => {
    const store = setup();
    press('c');
    const box = screen.getByPlaceholderText('In your own words. Posted as yours.');
    fireEvent.change(box, { target: { value: 'I checked: this is real' } });
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true });
    const comments = Object.values(store.getState().comments);
    expect(comments.map((comment) => comment.body)).toEqual(['I checked: this is real']);
    expect(store.getState().triage.c1).toBeUndefined();
  });

  it('o opens the finding in the code view and leaves the queue', () => {
    const store = setup();
    press('o');
    expect(store.getState().ui.verify).toBeNull();
    expect(store.getState().selection.findingId).toBe('c1');
    expect(store.getState().ui.mainView).toBe('code');
  });
});
