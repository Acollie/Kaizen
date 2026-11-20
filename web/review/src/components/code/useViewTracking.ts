// Records what the reviewer actually looked at (store.markViewed). A file counts once it
// has been open in the code view for VIEW_DWELL_MS; the selected changed function once
// its code has been on screen for that long. Time in a hidden tab does not count.

import { useEffect, type RefObject } from 'react';
import { useReviewStore } from '../../state/StoreProvider';
import { VIEW_DWELL_MS } from '../../state/viewing';

const TICK_MS = 250;

export interface Dwell {
  id: string | null;
  elapsed: number;
}

/** Add one tick of on-screen time for `id`; switching target restarts the count */
export function accumulateDwell(dwell: Dwell, id: string | null, visible: boolean, tick: number): Dwell {
  if (id !== dwell.id) return { id, elapsed: id && visible ? tick : 0 };
  return visible && id ? { id, elapsed: dwell.elapsed + tick } : dwell;
}

/** True when any head-side row in [start, end] intersects the container's visible area */
export function rangeOnScreen(container: HTMLElement, start: number, end: number): boolean {
  const box = container.getBoundingClientRect();
  for (let line = start; line <= end; line++) {
    const row = container.querySelector(`[data-key="n${line}"]`);
    if (!row) continue;
    const rect = row.getBoundingClientRect();
    if (rect.bottom > box.top && rect.top < box.bottom) return true;
  }
  return false;
}

const pageVisible = () => typeof document === 'undefined' || document.visibilityState !== 'hidden';

/** `path` is the PR file open in the code view, or null (loading, outside the diff) */
export function useViewTracking(containerRef: RefObject<HTMLElement>, path: string | null): void {
  const store = useReviewStore();

  useEffect(() => {
    if (!path) return undefined;
    let fileTime = 0;
    let dwell: Dwell = { id: null, elapsed: 0 };

    const tick = () => {
      const state = store.getState();
      if (!pageVisible()) return;
      fileTime += TICK_MS;
      if (fileTime >= VIEW_DWELL_MS) state.markViewed('file', path);
      const fn = state.selection.functionId ? state.index.functions.get(state.selection.functionId) : undefined;
      const target = fn && fn.file === path && !state.viewed.functions[fn.id] ? fn : undefined;
      const container = containerRef.current;
      const visible = Boolean(target && container && rangeOnScreen(container, target.start_line, target.end_line));
      dwell = accumulateDwell(dwell, target?.id ?? null, visible, TICK_MS);
      if (dwell.id && dwell.elapsed >= VIEW_DWELL_MS) state.markViewed('function', dwell.id);
    };

    const timer = window.setInterval(tick, TICK_MS);
    return () => window.clearInterval(timer);
  }, [containerRef, path, store]);
}
