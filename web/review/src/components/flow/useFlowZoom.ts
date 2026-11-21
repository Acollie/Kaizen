// Pan and zoom for the flow SVG. d3-zoom owns only the viewport <g>'s transform
// attribute; React owns everything inside it.

import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { select } from 'd3-selection';
import 'd3-transition';
import { zoom, zoomIdentity, type ZoomBehavior } from 'd3-zoom';
import type { Bounds } from '../../lib/flowLayout';
import { fitTransform } from './flowData';
import { MOVE_MS } from './useTween';

export interface FlowZoomOptions {
  /**
   * 'zoom' (default): the wheel zooms. 'modified': the wheel scrolls the page and only
   * Ctrl/Cmd + wheel (and trackpad pinch) zooms, for diagrams inside a scrolling view.
   */
  wheel?: 'zoom' | 'modified';
}

export function useFlowZoom(svgRef: RefObject<SVGSVGElement>, viewportRef: RefObject<SVGGElement>, options: FlowZoomOptions = {}) {
  const wheel = options.wheel ?? 'zoom';
  const behaviour = useRef<ZoomBehavior<SVGSVGElement, unknown> | null>(null);

  useEffect(() => {
    const svg = svgRef.current;
    const viewport = viewportRef.current;
    if (!svg || !viewport) return undefined;
    const zoomBehaviour = zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.2, 2.5])
      .filter((event: MouseEvent | WheelEvent) => {
        if (event.type === 'wheel') return wheel === 'zoom' || event.ctrlKey || event.metaKey;
        return (!event.ctrlKey || event.type === 'wheel') && !event.button;
      })
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
  }, [svgRef, viewportRef, wheel]);

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
