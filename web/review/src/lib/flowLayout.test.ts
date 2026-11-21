import type { GraphEdge, GraphNode, ReviewGraph } from '../types';
import {
  ARROW_LENGTH, NODE_HEIGHT, bezierPoint, buildFlowModel, connectedComponents, edgeStrokeWidth, findBackEdges,
  layoutFlow, longestPathRanks, wrapRanks, nodeWidth, packComponents, routeEdges, separate, signedDistances, visibleNodeIds,
  type FlowLayout, type LaidOutNode,
} from './flowLayout';

function node(id: string, overrides: Partial<GraphNode> = {}): GraphNode {
  return { id, name: id, file: 'pkg/a.go', line: 1, kind: 'changed', risk_score: 0, focus: false, fan_in: 0, complexity: 1, ...overrides };
}
const edge = (from: string, to: string, weight = 1): GraphEdge => ({ from, to, weight });
const graph = (nodes: GraphNode[], edges: GraphEdge[]): ReviewGraph => ({ nodes, edges });

function layoutOf(nodes: GraphNode[], edges: GraphEdge[], options: Parameters<typeof layoutFlow>[2] = {}): FlowLayout {
  const model = buildFlowModel(graph(nodes, edges));
  return layoutFlow(model, visibleNodeIds(model, options.focusId ?? null), options);
}
const byId = (layout: FlowLayout) => new Map(layout.nodes.map((laidOut) => [laidOut.id, laidOut]));
const overlaps = (left: LaidOutNode, right: LaidOutNode) =>
  left.x < right.x + right.width && right.x < left.x + left.width && left.y < right.y + right.height && right.y < left.y + left.height;

describe('buildFlowModel', () => {
  it('drops dangling edges, merges duplicates and separates self loops', () => {
    const model = buildFlowModel(graph([node('a'), node('b')], [edge('a', 'b'), edge('a', 'b', 2), edge('a', 'zzz'), edge('a', 'a')]));
    expect(model.edges).toEqual([{ from: 'a', to: 'b', weight: 3 }]);
    expect(model.selfLoops).toEqual([{ from: 'a', to: 'a', weight: 1 }]);
    expect(model.callees.get('a')).toEqual(['b']);
    expect(model.callers.get('b')).toEqual(['a']);
  });

  it('accepts null slices from Go', () => {
    const model = buildFlowModel({ nodes: null, edges: null });
    expect(model.nodes.size).toBe(0);
    expect(layoutFlow(model, new Set()).nodes).toEqual([]);
  });
});

describe('signedDistances', () => {
  // c2 -> c1 -> f -> d1 -> d2 -> d3
  const model = buildFlowModel(graph(
    ['c2', 'c1', 'f', 'd1', 'd2', 'd3'].map((id) => node(id)),
    [edge('c2', 'c1'), edge('c1', 'f'), edge('f', 'd1'), edge('d1', 'd2'), edge('d2', 'd3')],
  ));

  it('is negative for callers and positive for callees, up to the depth', () => {
    const distances = signedDistances(model, 'f');
    expect(Object.fromEntries(distances)).toEqual({ f: 0, c1: -1, c2: -2, d1: 1, d2: 2 });
    expect(visibleNodeIds(model, 'f').has('d3')).toBe(false);
  });

  it('shows everything without a (known) focus', () => {
    expect(visibleNodeIds(model, null).size).toBe(6);
    expect(visibleNodeIds(model, 'missing').size).toBe(6);
  });
});

describe('ranks', () => {
  it('uses longest paths so a shortcut edge does not pull the target up', () => {
    const ranks = longestPathRanks([node('a'), node('b'), node('c')], [edge('a', 'b'), edge('b', 'c'), edge('a', 'c')]);
    expect(Object.fromEntries(ranks)).toEqual({ a: 0, b: 1, c: 2 });
  });

  it('breaks cycles starting from the uncalled node', () => {
    const nodes = [node('entry'), node('x'), node('y')];
    const edges = [edge('entry', 'x'), edge('x', 'y'), edge('y', 'x')];
    expect([...findBackEdges(nodes, edges)]).toEqual(['y->x']);
    expect(Object.fromEntries(longestPathRanks(nodes, edges))).toEqual({ entry: 0, x: 1, y: 2 });
  });

  it('terminates on a pure cycle, breaking it at the riskiest node', () => {
    const nodes = [node('a'), node('b', { risk_score: 50 }), node('c')];
    const ranks = longestPathRanks(nodes, [edge('a', 'b'), edge('b', 'c'), edge('c', 'a')]);
    expect(ranks.get('b')).toBe(0);
    expect(ranks.get('c')).toBe(1);
    expect(ranks.get('a')).toBe(2);
  });

  it('marks edges against the flow as back edges and same-rank edges as lateral', () => {
    const layout = layoutOf([node('entry'), node('x'), node('y')], [edge('entry', 'x'), edge('x', 'y'), edge('y', 'x')]);
    const kinds = Object.fromEntries(layout.edges.map((laidOut) => [laidOut.key, laidOut.kind]));
    expect(kinds).toEqual({ 'entry->x': 'forward', 'x->y': 'forward', 'y->x': 'back' });

    const focused = layoutOf([node('f'), node('p'), node('q')], [edge('f', 'p'), edge('f', 'q'), edge('p', 'q')], { focusId: 'f' });
    expect(focused.edges.find((laidOut) => laidOut.key === 'p->q')?.kind).toBe('lateral');
  });

  it('keeps self loops (recursion) as their own edge kind', () => {
    const layout = layoutOf([node('a')], [edge('a', 'a')]);
    expect(layout.edges).toEqual([expect.objectContaining({ key: 'a->a', kind: 'self' })]);
  });
});

describe('layoutFlow placement', () => {
  const nodes = [node('caller'), node('f', { focus: true }), node('left'), node('right')];
  const edges = [edge('caller', 'f'), edge('f', 'left'), edge('f', 'right')];

  it('flows top to bottom in TB: callers above, callees below', () => {
    const placed = byId(layoutOf(nodes, edges, { focusId: 'f', direction: 'TB' }));
    expect(placed.get('caller')!.rank).toBe(-1);
    expect(placed.get('caller')!.y).toBeLessThan(placed.get('f')!.y);
    expect(placed.get('f')!.y).toBeLessThan(placed.get('left')!.y);
    expect(placed.get('left')!.y).toBe(placed.get('right')!.y);
  });

  it('flows left to right in LR', () => {
    const placed = byId(layoutOf(nodes, edges, { focusId: 'f', direction: 'LR' }));
    expect(placed.get('caller')!.x + placed.get('caller')!.width).toBeLessThan(placed.get('f')!.x);
    expect(placed.get('f')!.x + placed.get('f')!.width).toBeLessThan(placed.get('left')!.x);
    expect(placed.get('left')!.y).not.toBe(placed.get('right')!.y);
  });

  it('centres a parent over its children and never overlaps nodes', () => {
    const layout = layoutOf(nodes, edges, { focusId: 'f', direction: 'TB' });
    const placed = byId(layout);
    const centre = (id: string) => placed.get(id)!.x + placed.get(id)!.width / 2;
    expect(centre('f')).toBeCloseTo((centre('left') + centre('right')) / 2, 0);
    expect(centre('caller')).toBeCloseTo(centre('f'), 0);
    for (const left of layout.nodes) for (const right of layout.nodes) if (left !== right) expect(overlaps(left, right)).toBe(false);
  });

  it('orders a rank by its parents to avoid crossings', () => {
    // a -> a2, b -> b2; the children start in the wrong order by name (zA sorts after b2)
    const layout = layoutOf(
      [node('root'), node('a', { risk_score: 9 }), node('b', { risk_score: 1 }), node('zA'), node('b2')],
      [edge('root', 'a'), edge('root', 'b'), edge('a', 'zA'), edge('b', 'b2')],
      { direction: 'TB' },
    );
    const placed = byId(layout);
    const aIsLeft = placed.get('a')!.x < placed.get('b')!.x;
    expect(placed.get('zA')!.x < placed.get('b2')!.x).toBe(aIsLeft);
  });

  it('sizes nodes by their text within bounds', () => {
    expect(nodeWidth(node('x'))).toBe(96);
    expect(nodeWidth(node('x'.repeat(200)))).toBe(220);
    const long = layoutOf([node('n'.repeat(200))], []).nodes[0]!;
    expect(long.label.endsWith('…')).toBe(true);
    expect(long.height).toBe(NODE_HEIGHT);
    expect(layoutOf([node('ext', { file: '', kind: 'external' })], []).nodes[0]!.subLabel).toBe('external');
  });
});

describe('components', () => {
  const nodes = [
    node('low1', { risk_score: 2 }), node('low2'),
    node('high1', { risk_score: 40 }), node('high2'),
    node('alone', { risk_score: 10, file: 'pkg/solo.go' }),
  ];
  const edges = [edge('low1', 'low2'), edge('high1', 'high2')];

  it('splits weakly connected groups, riskiest first', () => {
    const groups = connectedComponents(nodes, edges).map((group) => group.map((member) => member.id).sort());
    expect(groups).toEqual([['high1', 'high2'], ['alone'], ['low1', 'low2']]);
  });

  it('labels each group and keeps groups apart', () => {
    const layout = layoutOf(nodes, edges, { aspect: 1 });
    expect(layout.groups.map((group) => group.label)).toEqual(['a.go', 'a.go', 'No calls within the PR']);
    expect(layout.groups[0]!.detail).toBe('2 changed');
    for (const left of layout.nodes) for (const right of layout.nodes) if (left !== right) expect(overlaps(left, right)).toBe(false);
  });

  it('gathers unconnected functions into one grid group, riskiest first', () => {
    const loners = [node('l1', { risk_score: 1 }), node('l2', { risk_score: 9 }), node('l3')];
    const layout = layoutOf([...nodes.slice(0, 2), ...loners], edges.slice(0, 1), { maxRankExtent: 220 });
    expect(layout.groups.at(-1)).toMatchObject({ key: 'loose', detail: '3 functions' });
    const placed = byId(layout);
    expect(placed.get('l2')!.y).toBeLessThanOrEqual(placed.get('l1')!.y);
    for (const left of layout.nodes) for (const right of layout.nodes) if (left !== right) expect(overlaps(left, right)).toBe(false);
  });

  it('can hide external functions in the whole-PR view only', () => {
    const model = buildFlowModel(graph([node('a'), node('ext', { kind: 'external', file: '' })], [edge('a', 'ext')]));
    expect([...visibleNodeIds(model, null, { hideExternal: true })]).toEqual(['a']);
    expect(visibleNodeIds(model, 'a', { hideExternal: true }).has('ext')).toBe(true);
  });

  it('does not label a single group or a focus view', () => {
    expect(layoutOf(nodes.slice(2, 4), edges.slice(1)).groups).toEqual([]);
    expect(layoutOf(nodes, edges, { focusId: 'high1' }).groups).toEqual([]);
  });

  it('packs groups into rows towards the viewport shape', () => {
    const sizes = [{ width: 100, height: 100 }, { width: 100, height: 100 }, { width: 100, height: 100 }];
    expect(new Set(packComponents(sizes, 100).map((offset) => offset.y)).size).toBe(1);
    expect(new Set(packComponents(sizes, 0.01).map((offset) => offset.x)).size).toBe(1);
    expect(packComponents([], 1)).toEqual([]);
  });

  it('bounds contain every node', () => {
    const layout = layoutOf(nodes, edges);
    for (const laidOut of layout.nodes) {
      expect(laidOut.x).toBeGreaterThanOrEqual(layout.bounds.x);
      expect(laidOut.y).toBeGreaterThanOrEqual(layout.bounds.y);
      expect(laidOut.x + laidOut.width).toBeLessThanOrEqual(layout.bounds.x + layout.bounds.width);
      expect(laidOut.y + laidOut.height).toBeLessThanOrEqual(layout.bounds.y + layout.bounds.height);
    }
  });
});

describe('wrapRanks', () => {
  const wide = (count: number) => [node('f'), ...Array.from({ length: count }, (_, index) => node(`callee${index}`))];
  const fanOut = (count: number) => Array.from({ length: count }, (_, index) => edge('f', `callee${index}`));

  it('wraps a wide rank onto continuation rows in TB', () => {
    const layout = layoutOf(wide(6), fanOut(6), { focusId: 'f', direction: 'TB', maxRankExtent: 260 });
    const callees = layout.nodes.filter((laidOut) => laidOut.rank === 1);
    const rows = new Set(callees.map((laidOut) => laidOut.y));
    expect(rows.size).toBe(3);
    const top = byId(layout).get('f')!;
    for (const callee of callees) expect(callee.y).toBeGreaterThan(top.y);
    for (const left of layout.nodes) for (const right of layout.nodes) if (left !== right) expect(overlaps(left, right)).toBe(false);
    for (const row of rows) {
      const inRow = callees.filter((laidOut) => laidOut.y === row);
      const extent = Math.max(...inRow.map((laidOut) => laidOut.x + laidOut.width)) - Math.min(...inRow.map((laidOut) => laidOut.x));
      expect(extent).toBeLessThanOrEqual(260);
    }
  });

  it('keeps a rank on one row when it fits, and never wraps LR', () => {
    expect(new Set(layoutOf(wide(2), fanOut(2), { focusId: 'f', maxRankExtent: 1000 }).nodes.filter((laidOut) => laidOut.rank === 1).map((laidOut) => laidOut.y)).size).toBe(1);
    const lr = layoutOf(wide(6), fanOut(6), { focusId: 'f', direction: 'LR', maxRankExtent: 100 });
    expect(new Set(lr.nodes.filter((laidOut) => laidOut.rank === 1).map((laidOut) => laidOut.x)).size).toBe(1);
  });

  it('puts nodes that continue downwards on the last row of a wrapped rank', () => {
    const nodes = [...wide(4), node('deeper')];
    const layout = layoutOf(nodes, [...fanOut(4), edge('callee0', 'deeper')], { focusId: 'f', direction: 'TB', maxRankExtent: 220 });
    const placed = byId(layout);
    const rankOne = layout.nodes.filter((laidOut) => laidOut.rank === 1);
    expect(placed.get('callee0')!.y).toBe(Math.max(...rankOne.map((laidOut) => laidOut.y)));
  });

  it('opens a corridor in earlier rows for edges into a later row', () => {
    const nodes = [...wide(4), node('deeper')];
    const layout = layoutOf(nodes, [...fanOut(4), edge('callee0', 'deeper')], { focusId: 'f', direction: 'TB', maxRankExtent: 220 });
    const placed = byId(layout);
    const parentX = placed.get('f')!.x + placed.get('f')!.width / 2;
    const targetX = placed.get('callee0')!.x + placed.get('callee0')!.width / 2;
    const between = layout.nodes.filter((laidOut) => laidOut.rank === 1 && laidOut.y < placed.get('callee0')!.y);
    const lo = Math.min(parentX, targetX);
    const hi = Math.max(parentX, targetX);
    for (const blocker of between) {
      // No box in a middle row may straddle a straight descent from the parent to its target
      const straddles = blocker.x < lo - 1 ? blocker.x + blocker.width > hi + 1 : false;
      expect(straddles).toBe(false);
      if (lo === hi) expect(blocker.x > lo || blocker.x + blocker.width < lo).toBe(true);
    }
  });

  it('marks continuation rows', () => {
    const rows = wrapRanks([[{ width: 100, rank: 0 }], [{ width: 100, rank: 1 }, { width: 100, rank: 1 }]], 150, 10);
    expect(rows.map((row) => [row.rank, row.continued, row.nodes.length])).toEqual([[0, false, 1], [1, false, 1], [1, true, 1]]);
  });
});

describe('separate', () => {
  it('keeps desired positions when there is room', () => {
    expect(separate([0, 200], [100, 100], 10)).toEqual([0, 200]);
  });
  it('pushes overlapping items apart around their mean', () => {
    const result = separate([50, 50], [100, 100], 10);
    expect(result[1]! - result[0]!).toBe(110);
    expect((result[0]! + result[1]!) / 2).toBe(50);
  });
});

describe('routeEdges', () => {
  const route = (direction: 'TB' | 'LR', nodes: GraphNode[], edges: GraphEdge[], focusId: string | null = null) => {
    const layout = layoutOf(nodes, edges, { direction, focusId });
    return { layout, routed: routeEdges(new Map(layout.nodes.map((laidOut) => [laidOut.id, laidOut])), layout.edges, direction) };
  };

  it('TB forward edges run from the bottom of the caller to the top of the callee', () => {
    const { layout, routed } = route('TB', [node('a'), node('b')], [edge('a', 'b')]);
    const [a, b] = [byId(layout).get('a')!, byId(layout).get('b')!];
    const only = routed[0]!;
    expect(only.curve.start).toEqual({ x: a.x + a.width / 2, y: a.y + NODE_HEIGHT });
    expect(only.tip).toEqual({ x: b.x + b.width / 2, y: b.y });
    expect(only.curve.end.y).toBe(b.y - ARROW_LENGTH);
    expect(only.path.startsWith('M')).toBe(true);
  });

  it('LR forward edges run from the right side to the left side', () => {
    const { layout, routed } = route('LR', [node('a'), node('b')], [edge('a', 'b')]);
    const [a, b] = [byId(layout).get('a')!, byId(layout).get('b')!];
    expect(routed[0]!.curve.start.x).toBe(a.x + a.width);
    expect(routed[0]!.tip.x).toBe(b.x);
    expect(routed[0]!.curve.end.x).toBe(b.x - ARROW_LENGTH);
  });

  it('spreads ports for fan-out, ordered by target position', () => {
    const { layout, routed } = route('TB', [node('f'), node('p'), node('q'), node('r')], [edge('f', 'p'), edge('f', 'q'), edge('f', 'r')]);
    const placed = byId(layout);
    const starts = routed.map((laidOut) => ({ x: laidOut.curve.start.x, target: placed.get(laidOut.to)!.x }));
    starts.sort((left, right) => left.target - right.target);
    expect(starts[0]!.x).toBeLessThan(starts[1]!.x);
    expect(starts[1]!.x).toBeLessThan(starts[2]!.x);
  });

  it('loops back edges around the outer side', () => {
    const nodes = [node('entry'), node('x'), node('y')];
    const edges = [edge('entry', 'x'), edge('x', 'y'), edge('y', 'x')];
    const tb = route('TB', nodes, edges).routed.find((laidOut) => laidOut.kind === 'back')!;
    expect(tb.curve.control1.x).toBeGreaterThan(tb.curve.start.x);
    expect(tb.curve.end.x).toBe(tb.tip.x + ARROW_LENGTH);
    const lr = route('LR', nodes, edges).routed.find((laidOut) => laidOut.kind === 'back')!;
    expect(lr.curve.control1.y).toBeGreaterThan(lr.curve.start.y);
  });

  it('routes self loops outside the box', () => {
    const { layout, routed } = route('TB', [node('a')], [edge('a', 'a')]);
    const box = layout.nodes[0]!;
    expect(routed[0]!.curve.control1.x).toBeGreaterThan(box.x + box.width);
  });

  it('places the weight label at the curve midpoint', () => {
    const { routed } = route('TB', [node('a'), node('b')], [edge('a', 'b', 3)]);
    expect(routed[0]!.weight).toBe(3);
    expect(routed[0]!.label).toEqual(bezierPoint(routed[0]!.curve, 0.5));
  });

  it('skips edges whose boxes are missing', () => {
    expect(routeEdges(new Map(), [{ from: 'a', to: 'b', weight: 1, key: 'a->b', kind: 'forward' }], 'TB')).toEqual([]);
  });
});

describe('edgeStrokeWidth', () => {
  it('grows with weight and is capped', () => {
    expect(edgeStrokeWidth(1)).toBe(1.25);
    expect(edgeStrokeWidth(4)).toBe(3.25);
    expect(edgeStrokeWidth(1000)).toBe(4);
  });
});
