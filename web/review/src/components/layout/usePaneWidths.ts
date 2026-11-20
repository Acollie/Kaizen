// Workspace layout hooks: persisted side-pane widths, the workspace's live width and
// media-query tracking for responsive defaults.

import { useCallback, useEffect, useState, type RefObject } from 'react';
import { loadWidths, saveWidths, type PaneWidths, type Side } from './layoutMath';

export interface PaneWidthsApi {
  preferred: PaneWidths;
  /** Update a width during a drag (not persisted yet) */
  setWidth(side: Side, width: number): void;
  /** Persist the current widths (end of a drag or a keyboard nudge) */
  commit(): void;
}

export function usePaneWidths(): PaneWidthsApi {
  const [preferred, setPreferred] = useState<PaneWidths>(loadWidths);

  const setWidth = useCallback((side: Side, width: number) => {
    setPreferred((current) => (current[side] === width ? current : { ...current, [side]: width }));
  }, []);

  const commit = useCallback(() => {
    setPreferred((current) => {
      saveWidths(current);
      return current;
    });
    // Panes that measure themselves on window resize (graphs, virtual lists) re-layout
    window.dispatchEvent(new Event('resize'));
  }, []);

  return { preferred, setWidth, commit };
}

/** The element's current content width, tracked with a ResizeObserver */
export function useElementWidth(ref: RefObject<HTMLElement>): number {
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const element = ref.current;
    if (!element) return undefined;
    setWidth(element.clientWidth);
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) setWidth(Math.round(entry.contentRect.width));
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [ref]);
  return width;
}

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => typeof window.matchMedia === 'function' && window.matchMedia(query).matches);
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return undefined;
    const list = window.matchMedia(query);
    const update = () => setMatches(list.matches);
    update();
    list.addEventListener('change', update);
    return () => list.removeEventListener('change', update);
  }, [query]);
  return matches;
}
