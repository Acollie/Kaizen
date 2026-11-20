import type { FileDiff, Finding, FunctionRisk } from '../../types';
import { buildFlowModel, layoutFlow, visibleNodeIds } from '../../lib/flowLayout';
import { changedLanguages, chooseDirection, findingsByNode, fitTransform, flowHint } from './flowData';

const fn = (id: string, start: number, end: number): FunctionRisk => ({
  id, name: id, file: 'a.go', language: 'Go', start_line: start, end_line: end, lines_added: 0, lines_removed: 0, is_new: false,
  fan_in: 0, fan_in_approximate: false, callers: [], callees: [], risk_score: 0, risk_reasons: [], focus: false,
});
const finding = (id: string, extra: Partial<Finding>): Finding => ({
  id, severity: 'warning', category: 'design', file: 'a.go', title: id, body: '', confidence: 1, ...extra,
});

describe('findingsByNode', () => {
  it('assigns findings by function id or line range', () => {
    const result = findingsByNode([fn('f', 1, 10), fn('g', 11, 20)], [
      finding('by-line', { line: 5 }),
      finding('by-id', { function_id: 'g', line: 2 }),
      finding('elsewhere', { line: 99 }),
    ]);
    expect(result.get('f')?.map((item) => item.id)).toEqual(['by-line']);
    expect(result.get('g')?.map((item) => item.id)).toEqual(['by-id']);
    expect(result.size).toBe(2);
  });
});

describe('chooseDirection', () => {
  it('is top-to-bottom in the side panel and left-to-right when expanded or wide', () => {
    expect(chooseDirection(440, 800, false)).toBe('TB');
    expect(chooseDirection(440, 800, true)).toBe('LR');
    expect(chooseDirection(900, 900, false)).toBe('LR');
    expect(chooseDirection(600, 300, false)).toBe('LR');
  });
});

describe('flowHint', () => {
  const nodes = ['c', 'f', 'd1', 'd2'].map((id) => ({
    id, name: id, file: 'a.go', line: 1, kind: 'changed' as const, risk_score: 0, focus: false, fan_in: 0, complexity: 1,
  }));
  const model = buildFlowModel({ nodes, edges: [{ from: 'c', to: 'f', weight: 1 }, { from: 'f', to: 'd1', weight: 1 }, { from: 'f', to: 'd2', weight: 1 }] });

  it('counts callers and callees around the focus', () => {
    const layout = layoutFlow(model, visibleNodeIds(model, 'f'), { focusId: 'f' });
    expect(flowHint({ layout, scope: 'selection', focusName: 'f', highlightName: null })).toBe('f · 1 caller → 2 callees');
  });

  it('describes the whole PR', () => {
    const layout = layoutFlow(model, visibleNodeIds(model, null));
    expect(flowHint({ layout, scope: 'all', focusName: null, highlightName: 'f' })).toBe('Whole PR · 1 call flow · f highlighted');
  });
});

describe('fitTransform', () => {
  it('centres small content without zooming past the cap', () => {
    const transform = fitTransform({ x: 0, y: 0, width: 100, height: 100 }, 400, 400, 20, 1);
    expect(transform).toEqual({ k: 1, x: 150, y: 150 });
  });
  it('scales large content down to fit', () => {
    const transform = fitTransform({ x: -50, y: 0, width: 720, height: 200 }, 400, 400, 20, 1);
    expect(transform.k).toBeCloseTo(0.5);
    expect(transform.x).toBeCloseTo(20 + 25);
  });
});

describe('changedLanguages', () => {
  it('lists distinct non-empty languages', () => {
    const files = [{ language: 'Python' }, { language: '' }, { language: 'Python' }, { language: 'Swift' }] as FileDiff[];
    expect(changedLanguages(files)).toEqual(['Python', 'Swift']);
  });
});
