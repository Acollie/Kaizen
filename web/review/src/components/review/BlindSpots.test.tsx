import { describe, expect, it } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { createReviewStore } from '../../state/store';
import { StoreProvider } from '../../state/StoreProvider';
import { makeContext, makeFile } from '../post/testFixtures';
import { makeFunction, toolCall } from '../../state/aiCoverage.fixtures';
import { BlindSpots, blindSpotsView, coverageSummary } from './BlindSpots';
import type { ReviewContext } from '../../types';

function context(): ReviewContext {
  const base = makeContext();
  const workflow = { ...makeFile('.github/workflows/ci.yml'), risk_score: 30, risk_reasons: ['workflow change'] };
  return {
    ...base,
    files: [makeFile('pkg/a.go'), makeFile('pkg/b.go'), workflow],
    functions: [
      makeFunction('pkg.Low', 'pkg/a.go', 1, 5, { risk_score: 5 }),
      makeFunction('pkg.Hot', 'pkg/a.go', 10, 20, { risk_score: 70, focus: true, risk_reasons: ['complexity 18'] }),
      makeFunction('pkg.Mid', 'pkg/b.go', 1, 9, { risk_score: 40 }),
      makeFunction('pkg.Seen', 'pkg/b.go', 20, 30, { risk_score: 50, focus: true }),
    ],
  };
}

function setup() {
  const store = createReviewStore(context());
  render(<StoreProvider store={store}><BlindSpots /></StoreProvider>);
  return store;
}

describe('blindSpotsView', () => {
  it('sorts skipped functions by risk, flags focus and puts file-level risk first', () => {
    const ctx = context();
    const coverage = { examinedFunctions: new Set(['pkg.Seen']), examinedFiles: new Set(['pkg/b.go']), complete: true, known: true, hidden: false };
    const view = blindSpotsView(coverage, ctx.functions!, ctx.files!);
    expect(view.skippedFunctions.map((fn) => fn.id)).toEqual(['pkg.Hot', 'pkg.Mid', 'pkg.Low']);
    expect(view.skippedFocus.map((fn) => fn.id)).toEqual(['pkg.Hot']);
    expect(view.skippedFiles.map((file) => file.path)).toEqual(['.github/workflows/ci.yml', 'pkg/a.go']);
    expect(view.summary).toBe('AI looked at 1 of 4 changed functions (1/2 focus) and 1 of 3 files');
    expect(coverageSummary({ ...coverage, complete: false }, ctx.functions!, ctx.files!)).toBe('AI has looked at 1 of 4 changed functions (1/2 focus) and 1 of 3 files so far');
  });
});

describe('BlindSpots', () => {
  it('renders nothing while running without targets, and a muted line for an old review', () => {
    const store = setup();
    expect(screen.queryByText(/Coverage unknown/)).toBeNull();
    act(() => store.getState().applyEvent({ seq: 1, type: 'tool_call', time: '', data: { tool: 'get_function', summary: 'get_function(pkg.Hot)' } }));
    act(() => store.getState().applyEvent({ seq: 2, type: 'done', time: '', data: null }));
    expect(screen.getByText(/Coverage unknown for this review/)).toBeTruthy();
    expect(screen.getByText('--rerun')).toBeTruthy();
  });

  it('labels live coverage "so far", warns about skipped focus functions and navigates on click', () => {
    const store = setup();
    act(() => store.getState().applyEvent(toolCall({ tool: 'get_function', summary: 'get_function(pkg.Seen)', target: 'pkg.Seen', target_kind: 'function' })));
    expect(screen.getAllByText(/so far/).length).toBeGreaterThan(0);
    expect(screen.getByText("The AI hasn't opened 1 Kaizen focus function yet")).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Hot' }));
    expect(store.getState().selection.functionId).toBe('pkg.Hot');

    fireEvent.click(screen.getByText('.github/workflows/ci.yml'));
    expect(store.getState().selection.file).toBe('.github/workflows/ci.yml');

    act(() => store.getState().applyEvent({ seq: 99, type: 'done', time: '', data: null }));
    expect(screen.getByText('The AI never opened 1 Kaizen focus function')).toBeTruthy();
    expect(screen.getByText('AI looked at 1 of 4 changed functions (1/2 focus) and 1 of 3 files')).toBeTruthy();
  });

  it('says so when everything was opened', () => {
    const store = setup();
    act(() => {
      for (const path of ['pkg/a.go', 'pkg/b.go', '.github/workflows/ci.yml']) {
        store.getState().applyEvent(toolCall({ tool: 'Read', summary: `Read(${path})`, target: path, target_kind: 'file' }));
      }
      store.getState().applyEvent({ seq: 100, type: 'done', time: '', data: null });
    });
    expect(screen.getByText("The AI looked at every changed function and file. That isn't a review: check the risky code yourself.")).toBeTruthy();
  });
});
