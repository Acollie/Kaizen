// Call edges and their arrowhead markers (SVG).

import { ARROW_LENGTH, edgeStrokeWidth, type RoutedEdge } from '../../lib/flowLayout';

/**
 * Arrowheads use userSpaceOnUse so they keep one size whatever the stroke width;
 * the path stops ARROW_LENGTH short of the box and the marker spans the gap.
 */
export function ArrowMarkers({ idPrefix }: { idPrefix: string }) {
  const marker = (suffix: string, className: string) => (
    <marker
      id={`${idPrefix}-${suffix}`}
      viewBox={`0 0 ${ARROW_LENGTH} 8`}
      refX={0}
      refY={4}
      markerUnits="userSpaceOnUse"
      markerWidth={ARROW_LENGTH}
      markerHeight={8}
      orient="auto"
    >
      <path d={`M0,0 L${ARROW_LENGTH},4 L0,8 z`} className={className} />
    </marker>
  );
  return (
    <defs>
      {marker('arrow', 'fl-arrowhead')}
      {marker('arrow-on', 'fl-arrowhead fl-arrowhead-on')}
    </defs>
  );
}

interface FlowEdgesProps {
  edges: readonly RoutedEdge[];
  /** Edges touching these nodes are drawn in the accent colour */
  activeIds: ReadonlySet<string>;
  /** Edges touching the hovered node are brightened */
  hoverId: string | null;
  idPrefix: string;
}

export function FlowEdges({ edges, activeIds, hoverId, idPrefix }: FlowEdgesProps) {
  // Active edges last so they draw on top
  const isActive = (edge: RoutedEdge) => activeIds.has(edge.from) || activeIds.has(edge.to);
  const ordered = [...edges].sort((left, right) => Number(isActive(left)) - Number(isActive(right)));
  return (
    <g className="fl-edges">
      {ordered.map((edge) => {
        const active = isActive(edge);
        const looped = edge.kind === 'back' || edge.kind === 'self';
        const hovered = !active && hoverId !== null && (edge.from === hoverId || edge.to === hoverId);
        const className = ['fl-edge', active ? 'fl-edge-on' : '', hovered ? 'fl-edge-hover' : '', looped ? 'fl-edge-back' : ''];
        return (
          <g key={edge.key} className={className.filter(Boolean).join(' ')}>
            <path
              d={edge.path}
              strokeWidth={edgeStrokeWidth(edge.weight)}
              markerEnd={`url(#${idPrefix}-${active ? 'arrow-on' : 'arrow'})`}
            />
            {edge.weight > 1 && <text className="fl-weight" x={edge.label.x + 5} y={edge.label.y - 3}>×{edge.weight}</text>}
          </g>
        );
      })}
    </g>
  );
}
