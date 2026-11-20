// Pan and zoom for the flow SVG. d3-zoom owns only the viewport <g>'s transform
// attribute; React owns everything inside it.

import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { select } from 'd3-selection';
import 'd3-transition';
import { zoom, zoomIdentity, type ZoomBehavior } from 'd3-zoom';
import type { Bounds } from '../../lib/flowLayout';
import { fitTransform } from './flowData';
import { MOVE_MS } from './useTween';

export function useFlowZoom(svgRef: RefObject<SVGSVGElement>, viewportRef: RefObject<SVGGElement>) {
  const behaviour = useRef<ZoomBehavior<SVGSVGElement, unknown> | null>(null);

  useEffect(() => {
    const svg = svgRef.current;
    const viewport = viewportRef.current;
    if (!svg || !viewport) return undefined;
    const zoomBehaviour = zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.2, 2.5])
      .on('zoom', (event: { transform: { toString(): string } }) => {
        viewport.setAttribute('transform', event.transform.toString());
      });
    behaviour.current = zoomBehaviour;
    const selection = select(svg);
    selection.call(zoomBehaviour).on('dblclick.zoom', null);
    return () => {
      selection.on('.zoom', null);
      behaviour.current = null;
    };
  }, [svgRef, viewportRef]);

  return useCallback((bounds: Bounds, animate: boolean) => {
    const svg = svgRef.current;
    const zoomBehaviour = behaviour.current;
    if (!svg || !zoomBehaviour || !svg.clientWidth) return;
    const { x, y, k } = fitTransform(bounds, svg.clientWidth, svg.clientHeight);
    const transform = zoomIdentity.translate(x, y).scale(k);
    const selection = select(svg);
    if (animate) selection.transition().duration(MOVE_MS).call(zoomBehaviour.transform, transform);
    else selection.interrupt().call(zoomBehaviour.transform, transform);
  }, [svgRef]);
}

/** Observed content-box size of an element */
export function useElementSize(ref: RefObject<HTMLElement>): { width: number; height: number } {
  const [size, setSize] = useState({ width: 0, height: 0 });
  useEffect(() => {
    const element = ref.current;
    if (!element) return undefined;
    const update = () => setSize((previous) =>
      (previous.width === element.clientWidth && previous.height === element.clientHeight
        ? previous
        : { width: element.clientWidth, height: element.clientHeight }));
    update();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, [ref]);
  return size;
}
