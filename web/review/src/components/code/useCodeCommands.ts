// Keyboard commands owned by the code view: hunk navigation, scrolling, context
// expansion and go-to-definition. They act on the cursor (see cursor.ts).

import { useRef, type RefObject } from 'react';
import { useCommands } from '../../state/commands';
import { useReviewStore } from '../../state/StoreProvider';
import type { FunctionRef, FunctionRisk } from '../../types';
import type { CursorStore } from './cursor';
import { findCalledFunction, type FindingPlacement, type FunctionRails } from './placement';
import { gapForExpansion, lineRowKey, stepHunk, type Row } from './rows';
import { keyAtTop, scrollToKey } from './scrolling';
import type { FileModelApi } from './useFileModel';
import type { ScrollRequest } from './useReveal';

export interface CodeCommandDeps {
  containerRef: RefObject<HTMLElement>;
  cursor: CursorStore;
  api: FileModelApi;
  placement: FindingPlacement;
  rails: FunctionRails;
  requestScroll: (request: ScrollRequest) => void;
  /** Moves the cursor (and syncs the current finding) */
  activate: (key: string) => void;
}

/** Row index of the cursor; a finding card counts as the line it hangs under */
export function cursorRowIndex(rows: readonly Row[], key: string | null, placement: FindingPlacement): number {
  if (!key) return -1;
  let rowKey: string | null = key;
  if (key.startsWith('f:')) {
    const id = key.slice(2);
    rowKey = null;
    for (const [line, findings] of placement.byLine) {
      if (findings.some((finding) => finding.id === id)) rowKey = lineRowKey(rows, line);
    }
    if (!rowKey) return -1;
  }
  return rows.findIndex((row) => row.key === rowKey);
}

/** Callee called on a line that can be opened: the enclosing function's callees first, then any changed function */
export function calleeOnLine(text: string, enclosing: FunctionRisk | undefined, changed: readonly FunctionRisk[]): FunctionRef | null {
  const local = (enclosing?.callees ?? []).filter((ref) => ref.id !== enclosing?.id && !ref.external && ref.file);
  const fromCallees = findCalledFunction(text, local);
  if (fromCallees) return fromCallees;
  const refs = changed.map((fn) => ({ id: fn.id, name: fn.name, file: fn.file, line: fn.start_line, external: false }));
  return findCalledFunction(text, refs.filter((ref) => ref.id !== enclosing?.id));
}

export function useCodeCommands(deps: CodeCommandDeps): void {
  const store = useReviewStore();
  const latest = useRef(deps);
  latest.current = deps;

  const rows = () => latest.current.api.rowsRef.current;
  const cursorIndex = () => cursorRowIndex(rows(), latest.current.cursor.get(), latest.current.placement);
  const container = () => latest.current.containerRef.current;

  const moveTo = (key: string | undefined, placement: ScrollRequest['placement'], flash = false) => {
    if (!key) return;
    latest.current.requestScroll({ key, placement, flash: flash ? [key] : [] });
  };

  const scrollPage = (fraction: number) => (count: number) => {
    const element = container();
    if (!element) return;
    element.scrollTop += element.clientHeight * fraction * count;
    window.requestAnimationFrame(() => {
      const key = keyAtTop(element);
      if (key) latest.current.activate(key);
    });
  };

  const expandAround = (direction: 'above' | 'below') => () => {
    const gap = gapForExpansion(rows(), cursorIndex(), direction);
    if (gap) void latest.current.api.expand(gap.key, direction);
  };

  const gotoDefinition = () => {
    const row = rows()[cursorIndex()];
    if (!row || row.type !== 'line') return;
    const state = store.getState();
    const enclosing = latest.current.rails.byRow.get(row.key);
    const callee = calleeOnLine(row.text, enclosing, state.context.functions ?? []);
    if (!callee) return;
    if (state.index.functions.has(callee.id)) state.selectFunction(callee.id);
    else if (callee.file && !callee.external) state.openLocation(callee.file, callee.line || 1);
  };

  const commentOnCursorLine = () => {
    const row = rows()[cursorIndex()];
    const { api } = latest.current;
    const path = api.currentPath();
    if (!row || row.type !== 'line' || !path || api.model.plain) return;
    const state = store.getState();
    if (row.kind === 'removed') state.setBlockedComment(row.key);
    else state.openComposer({ file: path, line: row.newLine });
  };

  useCommands({
    comment: commentOnCursorLine,
    nextHunk: (count) => moveTo(rows()[stepHunk(rows(), cursorIndex(), count)]?.key, 'start', true),
    prevHunk: (count) => moveTo(rows()[stepHunk(rows(), cursorIndex(), -count)]?.key, 'start', true),
    top: () => {
      const element = container();
      const first = rows()[0];
      if (element) element.scrollTop = 0;
      if (first) latest.current.activate(first.key);
    },
    bottom: () => {
      const last = rows()[rows().length - 1];
      const element = container();
      if (!last || !element) return;
      latest.current.activate(last.key);
      if (!scrollToKey(element, last.key, 'end')) element.scrollTop = element.scrollHeight;
    },
    halfPageDown: scrollPage(0.5),
    halfPageUp: scrollPage(-0.5),
    pageDown: scrollPage(0.9),
    pageUp: scrollPage(-0.9),
    expandAbove: expandAround('above'),
    expandBelow: expandAround('below'),
    expandAll: () => { void latest.current.api.expandAll(); },
    gotoDefinition,
  });
}
