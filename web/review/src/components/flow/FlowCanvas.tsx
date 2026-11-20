// The flow diagram's SVG: group labels, edges and nodes, plus the hover card.
// React renders everything; d3-zoom only sets the viewport group's transform.

import { useCallback, useId, useMemo, useRef, useState, type MouseEvent, type RefObject } from 'react';
import { useReview } from '../../state/StoreProvider';
import { routeEdges, type FlowLayout } from '../../lib/flowLayout';
import type { Finding } from '../../types';
import { FlowNode } from './FlowNode';
import { ArrowMarkers, FlowEdges } from './FlowEdges';
import { FlowTooltip, placeTooltip, type TooltipTarget } from './FlowTooltip';
import { useTweenedBoxes } from './useTween';

interface FlowCanvasProps {
  layout: FlowLayout;
  highlightId: string | null;
  findings: Map<string, Finding[]>;
  svgRef: RefObject<SVGSVGElement>;
  viewportRef: RefObject<SVGGElement>;
  animate: boolean;
  onSelect(id: string): void;
  onBackground(): void;
}

export function FlowCanvas({ layout, highlightId, findings, svgRef, viewportRef, animate, onSelect, onBackground }: FlowCanvasProps) {
  const idPrefix = `fl${useId().replace(/[^a-zA-Z0-9]/g, '')}`;
  const boxes = useTweenedBoxes(layout.nodes, animate);
  const edges = useMemo(() => routeEdges(boxes, layout.edges, layout.direction), [boxes, layout.edges, layout.direction]);
  const activeIds = useMemo(() => new Set(highlightId ? [highlightId] : []), [highlightId]);
  const { tooltip, onHover } = useHoverCard(svgRef);

  return (
    <>
      <svg ref={svgRef} className="fl-svg" role="img" aria-label="Call flow of the changed functions" onClick={onBackground}>
        <ArrowMarkers idPrefix={idPrefix} />
        <g ref={viewportRef}>
          <g className="fl-groups">
            {layout.groups.map((group) => (
              <text key={group.key} className="fl-group" style={{ transform: `translate(${group.x}px, ${group.y}px)` }}>
                <tspan className="fl-group-name">{group.label}</tspan>
                <tspan className="fl-group-detail" dx={8}>{group.detail}</tspan>
              </text>
            ))}
          </g>
          <FlowEdges edges={edges} activeIds={activeIds} hoverId={tooltip?.id ?? null} idPrefix={idPrefix} />
          <g className="fl-nodes">
            {layout.nodes.map((node) => {
              const box = boxes.get(node.id);
              return box && (
                <FlowNode key={node.id} node={node} box={box} selected={node.id === highlightId}
                  findings={findings.get(node.id)} onSelect={onSelect} onHover={onHover} />
              );
            })}
          </g>
        </g>
      </svg>
      {tooltip && <HoverCard target={tooltip} findings={findings.get(tooltip.id)} />}
    </>
  );
}

function HoverCard({ target, findings }: { target: TooltipTarget; findings: Finding[] | undefined }) {
  const node = useReview((state) => state.index.nodes.get(target.id));
  const fn = useReview((state) => state.index.functions.get(target.id));
  if (!node) return null;
  return <FlowTooltip node={node} fn={fn} findings={findings} left={target.left} top={target.top} />;
}

function useHoverCard(svgRef: RefObject<SVGSVGElement>) {
  const [tooltip, setTooltip] = useState<TooltipTarget | null>(null);
  const frame = useRef(0);
  const onHover = useCallback((id: string | null, event?: MouseEvent) => {
    const svg = svgRef.current;
    if (!id || !event || !svg) {
      setTooltip(null);
      return;
    }
    const bounds = svg.getBoundingClientRect();
    const pointer = { x: event.clientX - bounds.left, y: event.clientY - bounds.top };
    cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(() => {
      setTooltip({ id, ...placeTooltip(pointer.x, pointer.y, bounds.width, bounds.height) });
    });
  }, [svgRef]);
  return { tooltip, onHover };
}
