// Layered (Sugiyama-style) layout for the call-flow diagram. Pure and DOM-free.
//
// Calls flow from callers to callees. With a focus node, ranks are the signed call
// distance from it (callers negative, callees positive). Without one, ranks are longest
// paths after breaking cycles with a DFS. Order within a rank is refined with
// barycentre sweeps, nodes are pulled towards their neighbours, and each weakly
// connected group is laid out separately and packed, riskiest first.
// Edge routing (routeEdges) is separate so callers can re-route on every animation frame.

import type { GraphEdge, GraphNode, ReviewGraph } from '../types';

export type FlowDirection = 'TB' | 'LR';

export const FOCUS_DEPTH = 2;
export const NODE_HEIGHT = 40;
export const ARROW_LENGTH = 7;
const NODE_MIN_WIDTH = 96;
const NODE_MAX_WIDTH = 220;
const NAME_CHAR_WIDTH = 7.2;
const SUB_CHAR_WIDTH = 6.1;
const NODE_PADDING = 36;
const RANK_GAP: Record<FlowDirection, number> = { TB: 56, LR: 84 };
const SIBLING_GAP: Record<FlowDirection, number> = { TB: 16, LR: 14 };
const COMPONENT_GAP = 44;
const SUBROW_GAP = 18;
const LOOSE_MIN_WIDTH = 480;
const GROUP_LABEL_SPACE = 24;
const LOOP_OFFSET = 34;
const CROSSING_SWEEPS = 6;

// ---------- Model ----------

export interface FlowModel {
  nodes: Map<string, GraphNode>;
  /** Deduplicated edges between known, distinct nodes (weights summed) */
  edges: GraphEdge[];
  /** Direct recursion */
  selfLoops: GraphEdge[];
  callers: Map<string, string[]>;
  callees: Map<string, string[]>;
}

export function buildFlowModel(graph: ReviewGraph): FlowModel {
  const nodes = new Map((graph.nodes ?? []).map((node) => [node.id, node]));
  const merged = new Map<string, GraphEdge>();
  for (const edge of graph.edges ?? []) {
    if (!nodes.has(edge.from) || !nodes.has(edge.to)) continue;
    const key = edgeKey(edge);
    const existing = merged.get(key);
    const weight = Math.max(1, edge.weight || 1);
    merged.set(key, existing ? { ...existing, weight: existing.weight + weight } : { from: edge.from, to: edge.to, weight });
  }
  const all = [...merged.values()];
  const edges = all.filter((edge) => edge.from !== edge.to);
  const callers = new Map<string, string[]>([...nodes.keys()].map((id) => [id, []]));
  const callees = new Map<string, string[]>([...nodes.keys()].map((id) => [id, []]));
  for (const edge of edges) {
    callees.get(edge.from)?.push(edge.to);
    callers.get(edge.to)?.push(edge.from);
  }
  return { nodes, edges, selfLoops: all.filter((edge) => edge.from === edge.to), callers, callees };
}

export function edgeKey(edge: { from: string; to: string }): string {
  return `${edge.from}->${edge.to}`;
}

/** Signed call distance from the focus: negative for callers, positive for callees */
export function signedDistances(model: FlowModel, focusId: string, depth = FOCUS_DEPTH): Map<string, number> {
  const result = new Map<string, number>([[focusId, 0]]);
  const walk = (adjacency: Map<string, string[]>, sign: number) => {
    let frontier = [focusId];
    for (let level = 1; level <= depth; level++) {
      const next: string[] = [];
      for (const id of frontier) {
        for (const neighbour of adjacency.get(id) ?? []) {
          if (result.has(neighbour)) continue;
          result.set(neighbour, sign * level);
          next.push(neighbour);
        }
      }
      frontier = next;
    }
  };
  walk(model.callers, -1);
  walk(model.callees, 1);
  return result;
}

/**
 * Node IDs to draw: the focus neighbourhood, or every node. Without a focus,
 * hideExternal drops library functions (fmt.Sprintf...): they would otherwise tie
 * unrelated flows together through shared calls.
 */
export function visibleNodeIds(model: FlowModel, focusId: string | null, options: { depth?: number; hideExternal?: boolean } = {}): Set<string> {
  if (focusId && model.nodes.has(focusId)) return new Set(signedDistances(model, focusId, options.depth ?? FOCUS_DEPTH).keys());
  const all = [...model.nodes.values()];
  return new Set((options.hideExternal ? all.filter((node) => node.kind !== 'external') : all).map((node) => node.id));
}

// ---------- Layout types ----------

export interface LaidOutNode extends GraphNode {
  rank: number;
  x: number;
  y: number;
  width: number;
  height: number;
  label: string;
  subLabel: string;
}

export type EdgeKind = 'forward' | 'lateral' | 'back' | 'self';

export interface LaidOutEdge extends GraphEdge {
  key: string;
  kind: EdgeKind;
}

export interface FlowGroup {
  key: string;
  label: string;
  detail: string;
  x: number;
  y: number;
}

export interface Bounds { x: number; y: number; width: number; height: number }

export interface FlowLayout {
  nodes: LaidOutNode[];
  edges: LaidOutEdge[];
  groups: FlowGroup[];
  direction: FlowDirection;
  bounds: Bounds;
}

export interface LayoutOptions {
  focusId?: string | null;
  direction?: FlowDirection;
  /** Viewport width / height; groups are packed towards this shape */
  aspect?: number;
  /** TB only: wrap a rank onto extra rows when it is wider than this */
  maxRankExtent?: number;
}

// ---------- Layout ----------

export function layoutFlow(model: FlowModel, visible: ReadonlySet<string>, options: LayoutOptions = {}): FlowLayout {
  const direction = options.direction ?? 'TB';
  const focusId = options.focusId && visible.has(options.focusId) ? options.focusId : null;
  const nodes = [...visible].map((id) => model.nodes.get(id)).filter((node): node is GraphNode => Boolean(node));
  const edges = model.edges.filter((edge) => visible.has(edge.from) && visible.has(edge.to));
  const allComponents = focusId ? [nodes] : connectedComponents(nodes, edges);
  // Functions with no calls inside the PR are gathered into one grid rather than a group each
  const components = allComponents.filter((component) => component.length > 1 || allComponents.length === 1);
  const loose = allComponents.length > 1 ? allComponents.filter((component) => component.length === 1).flat() : [];
  const labelled = !focusId && allComponents.length > 1;
  const distances = focusId ? signedDistances(model, focusId) : null;
  const topSpace = labelled ? GROUP_LABEL_SPACE : 0;

  const placedComponents = components.map((component) => {
    const ids = new Set(component.map((node) => node.id));
    const componentEdges = edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to));
    const ranks = distances
      ? new Map(component.map((node) => [node.id, distances.get(node.id) ?? 0]))
      : longestPathRanks(component, componentEdges);
    return placeComponent(component, componentEdges, ranks, { direction, topSpace, maxRankExtent: options.maxRankExtent ?? Infinity });
  });
  const labels = components.map((component) => groupLabel(component));
  if (loose.length) {
    const widest = Math.max(LOOSE_MIN_WIDTH, ...placedComponents.map((placed) => placed.width));
    placedComponents.push(placeLoose(loose, Math.min(widest, options.maxRankExtent ?? Infinity), topSpace));
    labels.push({ key: 'loose', label: 'No calls within the PR', detail: `${loose.length} functions` });
  }

  const offsets = packComponents(placedComponents.map((placed) => ({ width: placed.width, height: placed.height })), options.aspect ?? 1);
  const placedNodes: LaidOutNode[] = [];
  const groups: FlowGroup[] = [];
  placedComponents.forEach((placed, order) => {
    const offset = offsets[order] ?? { x: 0, y: 0 };
    for (const node of placed.nodes) placedNodes.push({ ...node, x: node.x + offset.x, y: node.y + offset.y });
    const label = labels[order];
    if (labelled && label) groups.push({ ...label, x: offset.x, y: offset.y + 14 });
  });

  const rankOf = new Map(placedNodes.map((node) => [node.id, node.rank]));
  const laidOutEdges: LaidOutEdge[] = [...edges, ...model.selfLoops.filter((edge) => visible.has(edge.from))]
    .map((edge) => ({ ...edge, key: edgeKey(edge), kind: edgeKind(edge, rankOf) }));
  const loops = laidOutEdges.some((edge) => edge.kind === 'back' || edge.kind === 'self');
  return { nodes: placedNodes, edges: laidOutEdges, groups, direction, bounds: boundsOf(placedNodes, groups, direction, loops) };
}

function edgeKind(edge: GraphEdge, rankOf: Map<string, number>): EdgeKind {
  if (edge.from === edge.to) return 'self';
  const from = rankOf.get(edge.from) ?? 0;
  const to = rankOf.get(edge.to) ?? 0;
  if (to > from) return 'forward';
  return to === from ? 'lateral' : 'back';
}

/** What ranking needs from a node: flow functions and Overview packages both qualify */
export type RankableNode = Pick<GraphNode, 'id' | 'risk_score'>;

/** Longest-path layering with back edges (found by DFS) ignored */
export function longestPathRanks(nodes: readonly RankableNode[], edges: readonly GraphEdge[]): Map<string, number> {
  const backEdges = findBackEdges(nodes, edges);
  const forward = edges.filter((edge) => !backEdges.has(edgeKey(edge)));
  const ranks = new Map(nodes.map((node) => [node.id, 0]));
  for (let pass = 0; pass < nodes.length; pass++) {
    let changed = false;
    for (const edge of forward) {
      const candidate = (ranks.get(edge.from) ?? 0) + 1;
      if (candidate > (ranks.get(edge.to) ?? 0)) {
        ranks.set(edge.to, candidate);
        changed = true;
      }
    }
    if (!changed) break;
  }
  return ranks;
}

/** DFS from uncalled nodes (riskiest first) so cycles break in a natural place */
export function findBackEdges(nodes: readonly RankableNode[], edges: readonly GraphEdge[]): Set<string> {
  const outgoing = new Map<string, string[]>(nodes.map((node) => [node.id, []]));
  const incoming = new Map<string, number>(nodes.map((node) => [node.id, 0]));
  for (const edge of edges) {
    outgoing.get(edge.from)?.push(edge.to);
    incoming.set(edge.to, (incoming.get(edge.to) ?? 0) + 1);
  }
  const backEdges = new Set<string>();
  const state = new Map<string, 'active' | 'done'>();
  const visit = (id: string) => {
    state.set(id, 'active');
    for (const next of outgoing.get(id) ?? []) {
      if (state.get(next) === 'active') backEdges.add(edgeKey({ from: id, to: next }));
      else if (!state.has(next)) visit(next);
    }
    state.set(id, 'done');
  };
  const starts = [...nodes].sort((left, right) =>
    (incoming.get(left.id) ?? 0) - (incoming.get(right.id) ?? 0) || (right.risk_score || 0) - (left.risk_score || 0));
  for (const node of starts) if (!state.has(node.id)) visit(node.id);
  return backEdges;
}

export function connectedComponents(nodes: readonly GraphNode[], edges: readonly GraphEdge[]): GraphNode[][] {
  const parent = new Map(nodes.map((node) => [node.id, node.id]));
  const find = (id: string): string => {
    const up = parent.get(id) ?? id;
    if (up === id) return id;
    const root = find(up);
    parent.set(id, root);
    return root;
  };
  for (const edge of edges) parent.set(find(edge.from), find(edge.to));
  const groups = new Map<string, GraphNode[]>();
  for (const node of nodes) {
    const root = find(node.id);
    groups.set(root, [...(groups.get(root) ?? []), node]);
  }
  const riskOf = (group: GraphNode[]) => Math.max(...group.map((node) => node.risk_score || 0));
  const changedCount = (group: GraphNode[]) => group.filter((node) => node.kind === 'changed').length;
  return [...groups.values()].sort((left, right) =>
    riskOf(right) - riskOf(left) || changedCount(right) - changedCount(left) || right.length - left.length);
}

// ---------- Placement of one component ----------

interface PlacedComponent { nodes: LaidOutNode[]; width: number; height: number }

interface PlacementOptions { direction: FlowDirection; topSpace: number; maxRankExtent: number }

function placeComponent(
  nodes: readonly GraphNode[], edges: readonly GraphEdge[], ranks: Map<string, number>, options: PlacementOptions,
): PlacedComponent {
  const { direction, topSpace } = options;
  const layers = buildLayers(nodes, ranks);
  const rankKeys = [...layers.keys()].sort((left, right) => left - right);
  reduceCrossings(layers, rankKeys, edges);
  const vertical = direction === 'TB';
  const continues = new Set(edges.filter((edge) => (ranks.get(edge.to) ?? 0) > (ranks.get(edge.from) ?? 0)).map((edge) => edge.from));
  const rows = wrapRanks(rankKeys.map((rank) => layers.get(rank) ?? []), vertical ? options.maxRankExtent : Infinity, SIBLING_GAP[direction],
    (node) => continues.has(node.id));
  const centres = assignMinorCentres(rows.map((row) => row.nodes), edges, direction);
  const majors = rowStarts(rows, direction);
  if (vertical) openCorridors(rows, majors, centres, edges.filter((edge) => continues.has(edge.to) || edge.weight > 1));

  const placed: LaidOutNode[] = [];
  rows.forEach((row, rowIndex) => {
    const major = majors[rowIndex] ?? 0;
    const thickness = vertical ? NODE_HEIGHT : Math.max(...row.nodes.map((node) => node.width));
    for (const node of row.nodes) {
      const centre = centres.get(node.id) ?? 0;
      if (vertical) placed.push({ ...node, x: centre - node.width / 2, y: major + topSpace });
      else placed.push({ ...node, x: major + (thickness - node.width) / 2, y: centre - NODE_HEIGHT / 2 + topSpace });
    }
  });
  const minorStart = Math.min(...placed.map((node) => (vertical ? node.x : node.y - topSpace)));
  for (const node of placed) {
    if (vertical) node.x -= minorStart;
    else node.y -= minorStart;
  }
  const lastRow = rows[rows.length - 1];
  const majorEnd = (majors[majors.length - 1] ?? 0) + (vertical ? NODE_HEIGHT : Math.max(0, ...(lastRow?.nodes ?? []).map((node) => node.width)));
  const minorExtent = Math.max(0, ...placed.map((node) => (vertical ? node.x + node.width : node.y + NODE_HEIGHT - topSpace)));
  return vertical
    ? { nodes: placed, width: minorExtent, height: majorEnd + topSpace }
    : { nodes: placed, width: Math.max(majorEnd, 120), height: minorExtent + topSpace };
}

/** Where each row starts along the major axis */
function rowStarts(rows: readonly RankRow<LaidOutNode>[], direction: FlowDirection): number[] {
  const starts: number[] = [];
  let major = 0;
  rows.forEach((row, rowIndex) => {
    if (rowIndex > 0) major += row.continued ? SUBROW_GAP : RANK_GAP[direction];
    starts.push(major);
    major += direction === 'TB' ? NODE_HEIGHT : Math.max(...row.nodes.map((node) => node.width));
  });
  return starts;
}

const CORRIDOR_MARGIN = 10;

/**
 * TB only. When a rank wraps, edges into its later rows pass the earlier rows: push
 * the nodes of those rows aside so each such edge runs through a clear corridor. Only
 * called for the flow's spine (edges into nodes that call further, or repeated calls)
 * so wrapping still keeps narrow ranks narrow.
 */
function openCorridors(rows: readonly RankRow<LaidOutNode>[], majors: readonly number[], centres: Map<string, number>, edges: readonly GraphEdge[]) {
  const rowOf = new Map<string, number>();
  rows.forEach((row, rowIndex) => row.nodes.forEach((node) => rowOf.set(node.id, rowIndex)));
  rows.forEach((row, rowIndex) => {
    const rowCentreY = (majors[rowIndex] ?? 0) + NODE_HEIGHT / 2;
    const crossings: number[] = [];
    for (const edge of edges) {
      const sourceRow = rowOf.get(edge.from);
      const targetRow = rowOf.get(edge.to);
      if (sourceRow === undefined || targetRow === undefined || !(sourceRow < rowIndex && rowIndex < targetRow)) continue;
      const sourceY = (majors[sourceRow] ?? 0) + NODE_HEIGHT;
      const targetY = majors[targetRow] ?? 0;
      const fraction = (rowCentreY - sourceY) / Math.max(1, targetY - sourceY);
      const eased = fraction * fraction * (3 - 2 * fraction);
      const sourceX = centres.get(edge.from) ?? 0;
      crossings.push(sourceX + ((centres.get(edge.to) ?? 0) - sourceX) * eased);
    }
    for (const crossing of crossings.sort((left, right) => left - right)) clearCorridor(row.nodes, centres, crossing);
  });
}

function clearCorridor(nodes: readonly LaidOutNode[], centres: Map<string, number>, crossing: number) {
  const ordered = [...nodes].sort((left, right) => (centres.get(left.id) ?? 0) - (centres.get(right.id) ?? 0));
  const leftSide = ordered.filter((node) => (centres.get(node.id) ?? 0) < crossing).reverse();
  const rightSide = ordered.filter((node) => (centres.get(node.id) ?? 0) >= crossing);
  let limit = crossing - CORRIDOR_MARGIN;
  for (const node of leftSide) {
    const centre = Math.min(centres.get(node.id) ?? 0, limit - node.width / 2);
    centres.set(node.id, centre);
    limit = centre - node.width / 2 - SIBLING_GAP.TB;
  }
  limit = crossing + CORRIDOR_MARGIN;
  for (const node of rightSide) {
    const centre = Math.max(centres.get(node.id) ?? 0, limit + node.width / 2);
    centres.set(node.id, centre);
    limit = centre + node.width / 2 + SIBLING_GAP.TB;
  }
}

export interface RankRow<T> { nodes: T[]; rank: number; continued: boolean }

/**
 * Split ranks wider than maxExtent (along the minor axis) into several rows, so a
 * function with many callees stays readable in a narrow panel. Continuation rows sit
 * closer together than ranks. Nodes whose calls continue to the next rank go to the
 * last row, so those edges never cut through the rank's other rows.
 */
export function wrapRanks<T extends { width: number; rank: number }>(
  layers: readonly T[][], maxExtent: number, gap: number, continuesDown: (node: T) => boolean = () => false,
): RankRow<T>[] {
  const rows: RankRow<T>[] = [];
  for (const unordered of layers) {
    const fits = unordered.reduce((total, node) => total + node.width, 0) + gap * Math.max(0, unordered.length - 1) <= maxExtent;
    const layer = fits ? unordered : [...unordered.filter((node) => !continuesDown(node)), ...unordered.filter(continuesDown)];
    let current: T[] = [];
    let extent = 0;
    for (const node of layer) {
      const added = current.length ? gap + node.width : node.width;
      if (current.length && extent + added > maxExtent) {
        rows.push({ nodes: current, rank: node.rank, continued: rows.length > 0 && rows[rows.length - 1]?.rank === node.rank });
        current = [];
        extent = 0;
      }
      current.push(node);
      extent += current.length > 1 ? gap + node.width : node.width;
    }
    const rank = layer[0]?.rank ?? 0;
    if (current.length) rows.push({ nodes: current, rank, continued: rows.length > 0 && rows[rows.length - 1]?.rank === rank });
  }
  return rows;
}

function buildLayers(nodes: readonly GraphNode[], ranks: Map<string, number>): Map<number, LaidOutNode[]> {
  const layers = new Map<number, LaidOutNode[]>();
  for (const node of nodes) {
    const rank = ranks.get(node.id) ?? 0;
    const laidOut: LaidOutNode = {
      ...node, rank, x: 0, y: 0, width: nodeWidth(node), height: NODE_HEIGHT, label: nodeLabel(node), subLabel: nodeSubLabel(node),
    };
    layers.set(rank, [...(layers.get(rank) ?? []), laidOut]);
  }
  for (const layer of layers.values()) {
    layer.sort((left, right) =>
      kindOrder(left) - kindOrder(right) || (right.risk_score || 0) - (left.risk_score || 0) || left.name.localeCompare(right.name));
  }
  return layers;
}

/** Barycentre heuristic: alternately sweep down and up, ordering each rank by neighbour positions */
export function reduceCrossings(layers: Map<number, { id: string }[]>, rankKeys: readonly number[], edges: readonly GraphEdge[]): void {
  const position = new Map<string, number>();
  const reindex = () => {
    for (const rank of rankKeys) (layers.get(rank) ?? []).forEach((node, order) => position.set(node.id, order));
  };
  reindex();
  for (let sweep = 0; sweep < CROSSING_SWEEPS; sweep++) {
    const downward = sweep % 2 === 0;
    const order = downward ? rankKeys.slice(1) : rankKeys.slice(0, -1).reverse();
    for (const rank of order) {
      const layer = layers.get(rank) ?? [];
      const adjacentRank = downward ? rank - 1 : rank + 1;
      const inAdjacent = new Set((layers.get(adjacentRank) ?? []).map((node) => node.id));
      const weight = new Map(layer.map((node, current) => {
        const related = neighbours(node.id, edges).filter((id) => inAdjacent.has(id));
        const centre = related.length ? related.reduce((sum, id) => sum + (position.get(id) ?? 0), 0) / related.length : current;
        return [node.id, centre];
      }));
      layer.sort((left, right) => (weight.get(left.id) ?? 0) - (weight.get(right.id) ?? 0));
      reindex();
    }
  }
}

function neighbours(id: string, edges: readonly GraphEdge[]): string[] {
  const result: string[] = [];
  for (const edge of edges) {
    if (edge.from === id) result.push(edge.to);
    else if (edge.to === id) result.push(edge.from);
  }
  return result;
}

/**
 * Centre of each node along the minor axis (x in TB, y in LR). Ranks start centred,
 * then each rank is pulled towards the mean centre of its neighbours in the previous
 * rank (down, then up), keeping order and minimum gaps.
 */
function assignMinorCentres(layers: LaidOutNode[][], edges: readonly GraphEdge[], direction: FlowDirection): Map<string, number> {
  const size = (node: LaidOutNode) => (direction === 'TB' ? node.width : NODE_HEIGHT);
  const gap = SIBLING_GAP[direction];
  const centres = new Map<string, number>();
  const extents = layers.map((layer) => layer.reduce((total, node) => total + size(node), 0) + gap * Math.max(0, layer.length - 1));
  const widest = Math.max(0, ...extents);
  layers.forEach((layer, index) => {
    let cursor = (widest - (extents[index] ?? 0)) / 2;
    for (const node of layer) {
      centres.set(node.id, cursor + size(node) / 2);
      cursor += size(node) + gap;
    }
  });

  const pull = (layer: LaidOutNode[], reference: LaidOutNode[]) => {
    const inReference = new Set(reference.map((node) => node.id));
    const desired = layer.map((node) => {
      const related = neighbours(node.id, edges).filter((id) => inReference.has(id));
      if (!related.length) return centres.get(node.id) ?? 0;
      return related.reduce((sum, id) => sum + (centres.get(id) ?? 0), 0) / related.length;
    });
    const resolved = separate(desired, layer.map(size), gap);
    layer.forEach((node, order) => centres.set(node.id, resolved[order] ?? 0));
  };
  for (let index = 1; index < layers.length; index++) pull(layers[index] ?? [], layers[index - 1] ?? []);
  for (let index = layers.length - 2; index >= 0; index--) pull(layers[index] ?? [], layers[index + 1] ?? []);

  const minimum = Math.min(...layers.flat().map((node) => (centres.get(node.id) ?? 0) - size(node) / 2));
  for (const [id, centre] of centres) centres.set(id, centre - minimum);
  return centres;
}

/**
 * Keep ordered items at least `gap` apart while staying as close as possible (on
 * average) to their desired centres: push right where they overlap, then shift the
 * whole run back by the mean displacement.
 */
export function separate(desired: readonly number[], sizes: readonly number[], gap: number): number[] {
  const result: number[] = [];
  desired.forEach((want, index) => {
    const previous = result[index - 1];
    const minimum = previous === undefined ? -Infinity : previous + ((sizes[index - 1] ?? 0) + (sizes[index] ?? 0)) / 2 + gap;
    result.push(Math.max(want, minimum));
  });
  if (!result.length) return result;
  const drift = result.reduce((sum, value, index) => sum + (value - (desired[index] ?? 0)), 0) / result.length;
  return result.map((value) => value - drift);
}

// ---------- Packing components ----------

/** Shelf-pack component boxes in order (riskiest first), rows sized towards the viewport aspect */
export function packComponents(sizes: readonly { width: number; height: number }[], aspect: number): { x: number; y: number }[] {
  if (!sizes.length) return [];
  const area = sizes.reduce((sum, size) => sum + (size.width + COMPONENT_GAP) * (size.height + COMPONENT_GAP), 0);
  const widest = Math.max(...sizes.map((size) => size.width));
  const rowLimit = Math.max(widest, Math.sqrt(area * Math.max(0.05, aspect)));
  const offsets: { x: number; y: number }[] = [];
  let rowX = 0;
  let rowY = 0;
  let rowHeight = 0;
  for (const size of sizes) {
    if (rowX > 0 && rowX + size.width > rowLimit) {
      rowY += rowHeight + COMPONENT_GAP;
      rowX = 0;
      rowHeight = 0;
    }
    offsets.push({ x: rowX, y: rowY });
    rowX += size.width + COMPONENT_GAP;
    rowHeight = Math.max(rowHeight, size.height);
  }
  return offsets;
}

function groupLabel(component: readonly GraphNode[]): Omit<FlowGroup, 'x' | 'y'> {
  const lead = [...component].sort((left, right) => (right.risk_score || 0) - (left.risk_score || 0))[0];
  const changed = component.filter((node) => node.kind === 'changed').length;
  const file = lead?.file ? lead.file.split('/').pop() ?? '' : 'external';
  return {
    key: lead?.id ?? file,
    label: file,
    detail: `${changed} changed${component.length > changed ? ` · ${component.length - changed} context` : ''}`,
  };
}

/** Unconnected functions: rows of boxes, riskiest first */
function placeLoose(nodes: readonly GraphNode[], maxWidth: number, topSpace: number): PlacedComponent {
  const ordered = [...nodes].sort((left, right) => (right.risk_score || 0) - (left.risk_score || 0) || left.name.localeCompare(right.name));
  const placed: LaidOutNode[] = [];
  let x = 0;
  let y = topSpace;
  let width = 0;
  for (const node of ordered) {
    const boxWidth = nodeWidth(node);
    if (x > 0 && x + boxWidth > maxWidth) {
      x = 0;
      y += NODE_HEIGHT + SUBROW_GAP;
    }
    placed.push({ ...node, rank: 0, x, y, width: boxWidth, height: NODE_HEIGHT, label: nodeLabel(node), subLabel: nodeSubLabel(node) });
    width = Math.max(width, x + boxWidth);
    x += boxWidth + SIBLING_GAP.TB;
  }
  return { nodes: placed, width, height: y + NODE_HEIGHT };
}

function boundsOf(nodes: readonly LaidOutNode[], groups: readonly FlowGroup[], direction: FlowDirection, loops: boolean): Bounds {
  if (!nodes.length) return { x: 0, y: 0, width: 1, height: 1 };
  const minX = Math.min(...nodes.map((node) => node.x), ...groups.map((group) => group.x));
  const minY = Math.min(...nodes.map((node) => node.y), ...groups.map((group) => group.y - 14)) - 12;
  let maxX = Math.max(...nodes.map((node) => node.x + node.width));
  let maxY = Math.max(...nodes.map((node) => node.y + node.height));
  if (loops && direction === 'TB') maxX += LOOP_OFFSET;
  if (loops && direction === 'LR') maxY += LOOP_OFFSET;
  return { x: minX - 12, y: minY, width: maxX - minX + 24, height: maxY - minY + 12 };
}

// ---------- Node text ----------

function kindOrder(node: GraphNode): number {
  return { changed: 0, caller: 1, callee: 1, external: 2 }[node.kind] ?? 1;
}

function displayName(node: GraphNode): string {
  return `${node.focus ? '★ ' : ''}${node.name || node.id}`;
}

export function nodeLabel(node: GraphNode): string {
  const maxChars = Math.floor((NODE_MAX_WIDTH - NODE_PADDING) / NAME_CHAR_WIDTH);
  const name = node.name || node.id;
  const room = maxChars - (node.focus ? 2 : 0);
  return name.length > room ? `${name.slice(0, room - 1)}…` : name;
}

export function nodeSubLabel(node: GraphNode): string {
  if (!node.file) return 'external';
  const text = `${node.file.split('/').pop() ?? node.file}:${node.line}`;
  const maxChars = Math.floor((NODE_MAX_WIDTH - NODE_PADDING) / SUB_CHAR_WIDTH);
  return text.length > maxChars ? `…${text.slice(text.length - maxChars + 1)}` : text;
}

export function nodeWidth(node: GraphNode): number {
  const nameChars = displayName(node).length;
  const subChars = nodeSubLabel(node).length;
  const wanted = Math.max(nameChars * NAME_CHAR_WIDTH, subChars * SUB_CHAR_WIDTH) + NODE_PADDING;
  return Math.round(Math.max(NODE_MIN_WIDTH, Math.min(NODE_MAX_WIDTH, wanted)));
}

// ---------- Edge routing ----------

export interface Point { x: number; y: number }
export interface Bezier { start: Point; control1: Point; control2: Point; end: Point }

export interface RoutedEdge extends LaidOutEdge {
  curve: Bezier;
  path: string;
  /** Where the arrowhead's tip touches the target box */
  tip: Point;
  label: Point;
}

export interface NodeBox { id: string; x: number; y: number; width: number; height: number }

/**
 * Route edges between (possibly mid-animation) node boxes. Forward edges leave the
 * source's downstream side and enter the target's upstream side; ports are spread so
 * fan-out reads clearly. The path stops ARROW_LENGTH short of the box so the arrowhead
 * (drawn from the path's end) touches it exactly, whatever the stroke width.
 */
export function routeEdges(boxes: ReadonlyMap<string, NodeBox>, edges: readonly LaidOutEdge[], direction: FlowDirection): RoutedEdge[] {
  const ports = portOffsets(boxes, edges, direction);
  const routed: RoutedEdge[] = [];
  for (const edge of edges) {
    const source = boxes.get(edge.from);
    const target = boxes.get(edge.to);
    if (!source || !target) continue;
    const offsets = ports.get(edge.key) ?? { out: 0, in: 0 };
    const { curve, tip } = curveFor(edge.kind, source, target, direction, offsets);
    routed.push({ ...edge, curve, tip, path: bezierPath(curve), label: bezierPoint(curve, 0.5) });
  }
  return routed;
}

/** Port spreading: offsets along each box's side, ordered by the other end's position */
function portOffsets(boxes: ReadonlyMap<string, NodeBox>, edges: readonly LaidOutEdge[], direction: FlowDirection) {
  const minorCentre = (id: string) => {
    const box = boxes.get(id);
    if (!box) return 0;
    return direction === 'TB' ? box.x + box.width / 2 : box.y + box.height / 2;
  };
  const sideLength = (id: string) => {
    const box = boxes.get(id);
    return box ? (direction === 'TB' ? box.width : box.height) : 0;
  };
  const result = new Map<string, { out: number; in: number }>();
  const spread = (byNode: Map<string, LaidOutEdge[]>, end: 'out' | 'in') => {
    for (const [id, list] of byNode) {
      const other = (edge: LaidOutEdge) => (end === 'out' ? edge.to : edge.from);
      list.sort((left, right) => minorCentre(other(left)) - minorCentre(other(right)));
      const span = Math.min(sideLength(id) * 0.5, (list.length - 1) * (direction === 'TB' ? 12 : 7));
      list.forEach((edge, order) => {
        const offset = list.length === 1 ? 0 : -span / 2 + (span * order) / (list.length - 1);
        result.set(edge.key, { ...(result.get(edge.key) ?? { out: 0, in: 0 }), [end]: offset });
      });
    }
  };
  const outgoing = new Map<string, LaidOutEdge[]>();
  const incoming = new Map<string, LaidOutEdge[]>();
  for (const edge of edges) {
    if (edge.kind !== 'forward') continue;
    outgoing.set(edge.from, [...(outgoing.get(edge.from) ?? []), edge]);
    incoming.set(edge.to, [...(incoming.get(edge.to) ?? []), edge]);
  }
  spread(outgoing, 'out');
  spread(incoming, 'in');
  return result;
}

function curveFor(kind: EdgeKind, source: NodeBox, target: NodeBox, direction: FlowDirection, offsets: { out: number; in: number }) {
  if (kind === 'self') return selfLoop(source, direction);
  if (kind === 'back') return backCurve(source, target, direction);
  if (kind === 'lateral') return lateralCurve(source, target, direction);
  return forwardCurve(source, target, direction, offsets);
}

function forwardCurve(source: NodeBox, target: NodeBox, direction: FlowDirection, offsets: { out: number; in: number }) {
  if (direction === 'TB') {
    const start = { x: source.x + source.width / 2 + offsets.out, y: source.y + source.height };
    const tip = { x: target.x + target.width / 2 + offsets.in, y: target.y };
    const end = { x: tip.x, y: tip.y - ARROW_LENGTH };
    const bend = Math.max(18, (end.y - start.y) / 2);
    return { tip, curve: { start, control1: { x: start.x, y: start.y + bend }, control2: { x: end.x, y: end.y - bend }, end } };
  }
  const start = { x: source.x + source.width, y: source.y + source.height / 2 + offsets.out };
  const tip = { x: target.x, y: target.y + target.height / 2 + offsets.in };
  const end = { x: tip.x - ARROW_LENGTH, y: tip.y };
  const bend = Math.max(24, (end.x - start.x) / 2);
  return { tip, curve: { start, control1: { x: start.x + bend, y: start.y }, control2: { x: end.x - bend, y: end.y }, end } };
}

/** Against the flow: leave and re-enter on the outer side (right in TB, bottom in LR) */
function backCurve(source: NodeBox, target: NodeBox, direction: FlowDirection) {
  if (direction === 'TB') {
    const start = { x: source.x + source.width, y: source.y + source.height / 2 };
    const tip = { x: target.x + target.width, y: target.y + target.height / 2 };
    const end = { x: tip.x + ARROW_LENGTH, y: tip.y };
    const outer = Math.max(start.x, end.x) + LOOP_OFFSET;
    return { tip, curve: { start, control1: { x: outer, y: start.y }, control2: { x: outer, y: end.y }, end } };
  }
  const start = { x: source.x + source.width / 2, y: source.y + source.height };
  const tip = { x: target.x + target.width / 2, y: target.y + target.height };
  const end = { x: tip.x, y: tip.y + ARROW_LENGTH };
  const outer = Math.max(start.y, end.y) + LOOP_OFFSET;
  return { tip, curve: { start, control1: { x: start.x, y: outer }, control2: { x: end.x, y: outer }, end } };
}

/** Same rank: side to side with a slight bow against the flow so it clears other edges */
function lateralCurve(source: NodeBox, target: NodeBox, direction: FlowDirection) {
  const bow = 18;
  if (direction === 'TB') {
    const rightward = target.x >= source.x;
    const start = { x: rightward ? source.x + source.width : source.x, y: source.y + source.height / 2 };
    const tip = { x: rightward ? target.x : target.x + target.width, y: target.y + target.height / 2 };
    const end = { x: tip.x + (rightward ? -ARROW_LENGTH : ARROW_LENGTH), y: tip.y };
    const third = (end.x - start.x) / 3;
    return { tip, curve: { start, control1: { x: start.x + third, y: start.y - bow }, control2: { x: end.x - third, y: end.y - bow }, end } };
  }
  const downward = target.y >= source.y;
  const start = { x: source.x + source.width / 2, y: downward ? source.y + source.height : source.y };
  const tip = { x: target.x + target.width / 2, y: downward ? target.y : target.y + target.height };
  const end = { x: tip.x, y: tip.y + (downward ? -ARROW_LENGTH : ARROW_LENGTH) };
  const third = (end.y - start.y) / 3;
  return { tip, curve: { start, control1: { x: start.x - bow, y: start.y + third }, control2: { x: end.x - bow, y: end.y - third }, end } };
}

/** Recursion: a small loop on the outer side of the box */
function selfLoop(box: NodeBox, direction: FlowDirection) {
  if (direction === 'TB') {
    const start = { x: box.x + box.width, y: box.y + 10 };
    const tip = { x: box.x + box.width, y: box.y + box.height - 10 };
    const end = { x: tip.x + ARROW_LENGTH, y: tip.y };
    const outer = box.x + box.width + LOOP_OFFSET;
    return { tip, curve: { start, control1: { x: outer, y: start.y - 6 }, control2: { x: outer, y: end.y + 6 }, end } };
  }
  const start = { x: box.x + box.width - 16, y: box.y + box.height };
  const tip = { x: box.x + 16, y: box.y + box.height };
  const end = { x: tip.x, y: tip.y + ARROW_LENGTH };
  const outer = box.y + box.height + LOOP_OFFSET;
  return { tip, curve: { start, control1: { x: start.x + 6, y: outer }, control2: { x: end.x - 6, y: outer }, end } };
}

export function bezierPoint(curve: Bezier, t: number): Point {
  const inverse = 1 - t;
  const blend = (a: number, b: number, c: number, d: number) =>
    inverse ** 3 * a + 3 * inverse ** 2 * t * b + 3 * inverse * t ** 2 * c + t ** 3 * d;
  return {
    x: blend(curve.start.x, curve.control1.x, curve.control2.x, curve.end.x),
    y: blend(curve.start.y, curve.control1.y, curve.control2.y, curve.end.y),
  };
}

function bezierPath(curve: Bezier): string {
  const point = (value: Point) => `${round(value.x)},${round(value.y)}`;
  return `M${point(curve.start)} C${point(curve.control1)} ${point(curve.control2)} ${point(curve.end)}`;
}

function round(value: number): number {
  return Math.round(value * 10) / 10;
}

/** Stroke width by call weight: 1.25 for a single call, capped at 4 */
export function edgeStrokeWidth(weight: number): number {
  return Math.min(4, 1.25 + Math.log2(Math.max(1, weight || 1)));
}
