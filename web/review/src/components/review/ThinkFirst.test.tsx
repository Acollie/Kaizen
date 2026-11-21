// Think first in the Review panel: "Your pass" shows nothing from the AI, the reveal is
// never a gate, and after the reveal the Compare banner and tags appear.
import { describe, expect, it } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { createReviewStore } from '../../state/store';
import { StoreProvider } from '../../state/StoreProvider';
import { makeContext, makeFile, makeFinding } from '../post/testFixtures';
import { makeFunction, toolCall } from '../../state/aiCoverage.fixtures';
import { ReviewPanel } from './ReviewPanel';
import type { ReviewContext } from '../../types';

function context(): ReviewContext {
  const base = makeContext();
  return {
    ...base,
    files: [{ ...makeFile('pkg/a.go'), risk_score: 40 }, makeFile('pkg/b.go')],
    functions: [makeFunction('pkg.Hot', 'pkg/a.go', 1, 3, { risk_score: 60, focus: true, risk_reasons: ['complexity 18'] })],
  };
}

function setup() {
  const store = createReviewStore(context());
  act(() => {
    const state = store.getState();
    state.setThinkFirst(true);
    state.applyEvent(toolCall({ tool: 'get_function', summary: '', target: 'pkg.Hot', target_kind: 'function' }));
    state.applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 'w1', severity: 'warning', file: 'pkg/a.go', line: 2, title: 'Race in Hot', confidence: 0.9 }) } as never);
    state.applyEvent({ seq: 3, type: 'finding', data: makeFinding({ id: 'w2', severity: 'warning', file: 'pkg/b.go', line: 2, title: 'Leak in b', confidence: 0.5 }) } as never);
    state.applyEvent({ seq: 4, type: 'finding', data: makeFinding({ id: 's1', severity: 'suggestion', file: 'pkg/a.go', line: 1, category: 'design', title: 'Rename x' }) } as never);
  });
  render(<StoreProvider store={store}><ReviewPanel /></StoreProvider>);
  return store;
}

describe('Think first', () => {
  it('shows your pass with no AI content and an always-available reveal', () => {
    setup();
    const pass = screen.getByRole('region', { name: 'Your pass' });
    expect(pass.textContent).toContain('0 of 2 viewed');
    expect(pass.textContent).toContain('Hot');
    expect(pass.textContent).not.toMatch(/looked at|AI coverage|hotspot|lead/);
    expect(screen.queryByText('Race in Hot')).toBeNull();
    expect(screen.queryByText(/AI looked at/)).toBeNull();
    const reveal = screen.getByRole('button', { name: 'Reveal AI findings (3 ready)' }) as HTMLButtonElement;
    expect(reveal.disabled).toBe(false);
  });

  it('encourages comparing once every item is viewed', () => {
    const store = setup();
    act(() => {
      store.getState().markViewed('function', 'pkg.Hot');
    });
    expect(screen.getByText("You've done your pass. Compare with the AI.")).toBeTruthy();
  });

  it('after the reveal: compare banner, tags, severity first and confidence as a word', () => {
    const store = setup();
    act(() => {
      store.getState().markViewed('file', 'pkg/a.go');
      store.getState().saveComment({ file: 'pkg/a.go', line: 3, body: 'Is this racy?' });
    });
    fireEvent.click(screen.getByRole('button', { name: /Reveal AI findings/ }));
    expect(store.getState().reveal).not.toBeNull();
    expect(screen.getByText('You flagged 1 thing yourself · AI raised 3 · 2 overlap by file:line (±3)')).toBeTruthy();
    expect(screen.getAllByText('you also found this').length).toBeGreaterThan(0);
    expect(screen.getByText('outside your pass')).toBeTruthy();
    expect(screen.getAllByText('Should fix').length).toBeGreaterThan(0);
    expect(screen.getByText('high conf.').getAttribute('title')).toContain('0.90');
    expect(screen.queryByText('90%')).toBeNull();
    expect(screen.getByRole('button', { name: /Verify queue/ })).toBeTruthy();
  });

  it('folds suggestions until the warnings are triaged, expandable any time', () => {
    const store = setup();
    act(() => store.getState().revealFindings());
    expect(screen.queryByText('Rename x')).toBeNull();
    const fold = screen.getByRole('button', { name: '1 suggestion (triage the warnings first)' });
    fireEvent.click(fold);
    expect(screen.getByText('Rename x')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Collapse' }));
    expect(screen.queryByText('Rename x')).toBeNull();
    act(() => {
      store.getState().acceptFinding('w1');
      store.getState().dismissFinding('w2', 'False positive');
    });
    expect(screen.getByText('Rename x')).toBeTruthy();
  });
});
