// Scrolls the code view to the selection whenever selection.reveal changes: loads the
// file, expands context so the target line exists, then (after the rows render) moves
// the cursor there, scrolls it into a comfortable position and flashes it.

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';
import { useReview, useReviewStore } from '../../state/StoreProvider';
import type { ReviewState, Selection } from '../../state/store';
import type { CursorStore } from './cursor';
import { findingKey } from './FindingCard';
import { findingAnchorLine } from './placement';
import { flashKeys, scrollToKey, type Placement } from './scrolling';
import type { FileModelApi } from './useFileModel';
import type { Row } from './rows';

export interface ScrollRequest {
  /** Row or card key to put the cursor on; null scrolls to the top */
  key: string | null;
  placement: Placement;
  flash: string[];
  /** The file changed: start from the top-left */
  reset?: boolean;
  /** Wait until this file's rows have rendered before applying */
  path?: string;
}

function functionRowKeys(rows: readonly Row[], start: number, end: number): string[] {
  return rows.filter((row) => row.type === 'line' && row.anchorLine >= start && row.anchorLine <= end).map((row) => row.key);
}

async function resolveTarget(state: ReviewState, selection: Selection, api: FileModelApi): Promise<ScrollRequest> {
  const finding = selection.findingId ? state.findings[selection.findingId] : undefined;
  if (finding && finding.file === selection.file) {
    if (finding.line && !(await api.revealLine(findingAnchorLine(finding)))) await api.revealLine(finding.line);
    const key = findingKey(finding.id);
    return { key, placement: 'focus', flash: [key] };
  }
  const fn = selection.functionId ? state.index.functions.get(selection.functionId) : undefined;
  if (fn && fn.file === selection.file && !selection.callSite && (!selection.line || selection.line === fn.start_line)) {
    const startKey = await api.revealLine(fn.start_line);
    const keys = functionRowKeys(api.rowsRef.current, fn.start_line, fn.end_line);
    return { key: startKey ?? keys[0] ?? null, placement: 'focus', flash: keys };
  }
  if (selection.line) {
    const key = await api.revealLine(selection.line);
    if (key) return { key, placement: 'focus', flash: [key] };
  }
  return { key: api.rowsRef.current[0]?.key ?? null, placement: 'start', flash: [] };
}

/** Apply scroll requests after the rows they refer to have rendered */
function usePendingScroll(containerRef: RefObject<HTMLElement>, cursor: CursorStore, rows: readonly Row[], path: string | null) {
  const [request, setRequest] = useState<ScrollRequest | null>(null);
  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!request || !container) return;
    if (request.path && request.path !== path) return;
    if (request.reset) {
      container.scrollTop = 0;
      container.scrollLeft = 0;
    }
    if (request.key) {
      cursor.set(request.key);
      if (request.placement === 'start' && request.key === rows[0]?.key) container.scrollTop = 0;
      else scrollToKey(container, request.key, request.placement);
      flashKeys(container, request.flash);
    }
    setRequest(null);
  }, [request, rows, path, containerRef, cursor]);
  return setRequest;
}

export function useReveal(containerRef: RefObject<HTMLElement>, cursor: CursorStore, api: FileModelApi) {
  const store = useReviewStore();
  const reveal = useReview((state) => state.selection.reveal);
  const requestScroll = usePendingScroll(containerRef, cursor, api.model.rows, api.model.path);
  const apiRef = useRef(api);
  apiRef.current = api;

  useEffect(() => {
    const selection = store.getState().selection;
    const path = selection.file;
    if (!path) return;
    let cancelled = false;
    void (async () => {
      const latest = apiRef.current;
      const reset = latest.currentPath() !== path;
      if (!(await latest.open(path, selection.line)) || cancelled) return;
      const target = await resolveTarget(store.getState(), selection, apiRef.current);
      if (!cancelled) requestScroll({ ...target, reset, path });
    })();
    return () => { cancelled = true; };
  }, [reveal, store, requestScroll]);

  return useCallback((request: ScrollRequest) => requestScroll(request), [requestScroll]);
}
