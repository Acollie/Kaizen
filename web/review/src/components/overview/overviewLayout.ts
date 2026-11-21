// Left-to-right layered layout of the package map. Reuses the flow diagram's ranking,
// crossing reduction, separation and edge routing (lib/flowLayout.ts) with larger,
// fixed-size package boxes. Packages with no calls to or from other packages in the PR
// are laid out in a grid under the layered part instead of piling into one rank.

import {
  edgeKey, longestPathRanks, reduceCrossings, routeEdges, separate,
  type Bounds, type EdgeKind, type LaidOutEdge, type NodeBox, type RoutedEdge,
} from '../../lib/flowLayout';
import type { PackageEdge, PackageInfo } from '../../lib/packageMap';

export const PACKAGE_WIDTH = 208;
export const PACKAGE_HEIGHT = 112;
const RANK_GAP = 64;
const SIBLING_GAP = 22;
const GRID_GAP_X = 28;
const GRID_GAP_Y = 22;
const SECTION_GAP = 56;
const LABEL_SPACE = 26;
const PULL_SWEEPS = 4;

export interface PlacedPackage extends NodeBox {
  pkg: PackageInfo;
  rank: number;
  connected: boolean;
}

export interface OverviewLayout {
  nodes: PlacedPackage[];
  edges: RoutedEdge[];
  /** Label over the grid of unconnected packages, when there is one */
  gridLabel: { x: number; y: number; text: string } | null;
  bounds: Bounds;
}

function edgeKind(edge: PackageEdge, ranks: ReadonlyMap<string, number>): EdgeKind {
  const from = ranks.get(edge.from) ?? 0;
  const to = ranks.get(edge.to) ?? 0;
  if (to > from) return 'forward';
  return to === from ? 'lateral' : 'back';
}

/** Riskier, more urgent packages first */
function prominence(pkg: PackageInfo): number {
  return pkg.maxRisk + (pkg.severity.critical + pkg.severity.warning) * 20 + pkg.focusCount * 8;
}

function byProminence(left: PackageInfo, right: PackageInfo): number {
  return prominence(right) - prominence(left) || left.key.localeCompare(right.key);
}

export function layoutOverview(packages: readonly PackageInfo[], edges: readonly PackageEdge[], aspect = 1.4): OverviewLayout {
  const keys = new Set(packages.map((pkg) => pkg.key));
  const visibleEdges = edges.filter((edge) => keys.has(edge.from) && keys.has(edge.to) && edge.from !== edge.to);
  const linked = new Set(visibleEdges.flatMap((edge) => [edge.from, edge.to]));
  const connected = packages.filter((pkg) => linked.has(pkg.key));
  const loose = packages.filter((pkg) => !linked.has(pkg.key)).sort(byProminence);

  const layered = placeLayered(connected, visibleEdges);
  const layeredHeight = layered.nodes.length ? Math.max(...layered.nodes.map((node) => node.y + node.height)) : 0;
  const gridTop = layered.nodes.length ? layeredHeight + SECTION_GAP : 0;
  const columns = gridColumns(loose.length, layered.rankCount, aspect);
  const labelled = layered.nodes.length > 0 && loose.length > 0;
  const grid = placeGrid(loose, columns, gridTop + (labelled ? LABEL_SPACE : 0));
  const nodes = [...layered.nodes, ...grid];

  const boxes = new Map(nodes.map((node) => [node.id, node]));
  const laidOut: LaidOutEdge[] = visibleEdges.map((edge) => ({ ...edge, key: edgeKey(edge), kind: edgeKind(edge, layered.ranks) }));
  const gridLabel = labelled ? { x: 0, y: gridTop + 14, text: `No calls to or from other packages in the PR · ${loose.length}` } : null;
  return { nodes, edges: routeEdges(boxes, laidOut, 'LR'), gridLabel, bounds: boundsOf(nodes, laidOut.some((edge) => edge.kind === 'back')) };
}

function gridColumns(count: number, rankCount: number, aspect: number): number {
  if (count === 0) return 1;
  const shaped = Math.ceil(Math.sqrt(count * Math.max(0.5, aspect) * (PACKAGE_HEIGHT / PACKAGE_WIDTH)));
  return Math.max(1, Math.min(count, Math.max(rankCount, shaped), 6));
}

function placeGrid(packages: readonly PackageInfo[], columns: number, top: number): PlacedPackage[] {
  return packages.map((pkg, order) => ({
    id: pkg.key, pkg, rank: 0, connected: false,
    x: (order % columns) * (PACKAGE_WIDTH + GRID_GAP_X),
    y: top + Math.floor(order / columns) * (PACKAGE_HEIGHT + GRID_GAP_Y),
    width: PACKAGE_WIDTH, height: PACKAGE_HEIGHT,
  }));
}

interface Layered { nodes: PlacedPackage[]; ranks: Map<string, number>; rankCount: number }

function placeLayered(packages: readonly PackageInfo[], edges: readonly PackageEdge[]): Layered {
  if (!packages.length) return { nodes: [], ranks: new Map(), rankCount: 0 };
  const ranks = longestPathRanks(packages.map((pkg) => ({ id: pkg.key, risk_score: pkg.maxRisk })), edges);
  const layers = new Map<number, { id: string; pkg: PackageInfo }[]>();
  for (const pkg of [...packages].sort(byProminence)) {
    const rank = ranks.get(pkg.key) ?? 0;
    layers.set(rank, [...(layers.get(rank) ?? []), { id: pkg.key, pkg }]);
  }
  const rankKeys = [...layers.keys()].sort((left, right) => left - right);
  reduceCrossings(layers, rankKeys, edges);
  const centres = minorCentres(rankKeys.map((rank) => layers.get(rank) ?? []), edges);
  const nodes: PlacedPackage[] = [];
  rankKeys.forEach((rank, column) => {
    for (const { id, pkg } of layers.get(rank) ?? []) {
      nodes.push({
        id, pkg, rank, connected: true,
        x: column * (PACKAGE_WIDTH + RANK_GAP), y: (centres.get(id) ?? 0) - PACKAGE_HEIGHT / 2,
        width: PACKAGE_WIDTH, height: PACKAGE_HEIGHT,
      });
    }
  });
  const top = Math.min(...nodes.map((node) => node.y));
  for (const node of nodes) node.y -= top;
  return { nodes, ranks, rankCount: rankKeys.length };
}

/** Vertical centre per package: ranks start centred, then are pulled towards their neighbours */
function minorCentres(layers: readonly { id: string }[][], edges: readonly PackageEdge[]): Map<string, number> {
  const centres = new Map<string, number>();
  const extent = (layer: readonly unknown[]) => layer.length * PACKAGE_HEIGHT + Math.max(0, layer.length - 1) * SIBLING_GAP;
  const tallest = Math.max(...layers.map(extent));
  for (const layer of layers) {
    let cursor = (tallest - extent(layer)) / 2 + PACKAGE_HEIGHT / 2;
    for (const node of layer) {
      centres.set(node.id, cursor);
      cursor += PACKAGE_HEIGHT + SIBLING_GAP;
    }
  }
  const neighbours = (id: string) => edges.flatMap((edge) => (edge.from === id ? [edge.to] : edge.to === id ? [edge.from] : []));
  const pull = (layer: readonly { id: string }[], reference: readonly { id: string }[]) => {
    const inReference = new Set(reference.map((node) => node.id));
    const desired = layer.map((node) => {
      const related = neighbours(node.id).filter((id) => inReference.has(id));
      return related.length ? related.reduce((sum, id) => sum + (centres.get(id) ?? 0), 0) / related.length : centres.get(node.id) ?? 0;
    });
    separate(desired, layer.map(() => PACKAGE_HEIGHT), SIBLING_GAP).forEach((centre, order) => centres.set(layer[order]!.id, centre));
  };
  for (let sweep = 0; sweep < PULL_SWEEPS; sweep++) {
    if (sweep % 2 === 0) for (let index = 1; index < layers.length; index++) pull(layers[index]!, layers[index - 1]!);
    else for (let index = layers.length - 2; index >= 0; index--) pull(layers[index]!, layers[index + 1]!);
  }
  return centres;
}

function boundsOf(nodes: readonly NodeBox[], loops: boolean): Bounds {
  if (!nodes.length) return { x: 0, y: 0, width: 1, height: 1 };
  const minX = Math.min(...nodes.map((node) => node.x));
  const minY = Math.min(...nodes.map((node) => node.y));
  const maxX = Math.max(...nodes.map((node) => node.x + node.width));
  const maxY = Math.max(...nodes.map((node) => node.y + node.height)) + (loops ? 40 : 0);
  return { x: minX - 16, y: minY - 16, width: maxX - minX + 32, height: maxY - minY + 32 };
}
