// The main pane's mode: the Overview on load; any navigation that opens code shows the code.
import { describe, expect, it } from 'vitest';
import { createReviewStore } from './store';
import { makeContext, makeFinding } from '../components/post/testFixtures';
import { makeFunction } from './aiCoverage.fixtures';

function store() {
  const context = makeContext();
  context.functions = [makeFunction('a.F', 'pkg/a.go', 1, 3)];
  context.graph = {
    nodes: [
      { id: 'a.F', name: 'F', file: 'pkg/a.go', line: 1, kind: 'changed', risk_score: 1, focus: false, fan_in: 0, complexity: 1 },
      { id: 'b.G', name: 'G', file: 'pkg/b.go', line: 9, kind: 'callee', risk_score: 0, focus: false, fan_in: 0, complexity: 1 },
    ],
    edges: [{ from: 'a.F', to: 'b.G', weight: 1 }],
  };
  const created = createReviewStore(context);
  created.getState().applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'w' }) } as never);
  return created;
}

describe('main view', () => {
  it('starts on the Overview', () => {
    expect(store().getState().ui.mainView).toBe('overview');
  });

  it('toggles between overview and code', () => {
    const created = store();
    created.getState().toggleMainView();
    expect(created.getState().ui.mainView).toBe('code');
    created.getState().toggleMainView();
    expect(created.getState().ui.mainView).toBe('overview');
    created.getState().setMainView('code');
    expect(created.getState().ui.mainView).toBe('code');
  });

  it('the initial selection (showCode: false) stays on the Overview', () => {
    const created = store();
    created.getState().selectFunction('a.F', { record: false, showCode: false });
    expect(created.getState().selection.functionId).toBe('a.F');
    expect(created.getState().ui.mainView).toBe('overview');
  });

  const navigations: [string, (state: ReturnType<ReturnType<typeof store>['getState']>) => void][] = [
    ['selectFunction', (state) => state.selectFunction('a.F')],
    ['openFile', (state) => state.openFile('pkg/a.go')],
    ['openFinding', (state) => state.openFinding('w')],
    ['openLocation', (state) => state.openLocation('pkg/a.go', 2)],
    ['selectNode', (state) => state.selectNode('b.G')],
  ];
  for (const [name, navigate] of navigations) {
    it(`${name} switches to the code view`, () => {
      const created = store();
      navigate(created.getState());
      expect(created.getState().ui.mainView).toBe('code');
    });
  }

  it('going back shows the code', () => {
    const created = store();
    created.getState().openFile('pkg/a.go');
    created.getState().selectFunction('a.F');
    created.getState().setMainView('overview');
    created.getState().goBack();
    expect(created.getState().ui.mainView).toBe('code');
  });
});
