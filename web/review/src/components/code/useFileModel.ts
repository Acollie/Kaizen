// Owns the rows of the file in the code view: builds them from the diff (or fetches a
// window of a file outside the diff), expands gaps via /api/file, and caches each
// file's rows so expansions survive navigating away and back. A generated file's
// withheld diff can be replaced by its head version on request (showWithheld).

import { useCallback, useMemo, useRef, useState } from 'react';
import { fetchFileLines } from '../../lib/api';
import { useReview } from '../../state/StoreProvider';
import {
  buildDiffRows, buildPlainRows, expandGap, findGapContaining, gapRange, lineRowKey,
  type ExpandDirection, type Row, type SourceLine,
} from './rows';

const MAX_FETCH = 500;
const PLAIN_WINDOW = 40;
const REVEAL_RADIUS = 10;

export interface FileModel {
  path: string | null;
  /** True for a read-only file outside the diff */
  plain: boolean;
  /** A generated PR file whose diff was withheld, shown as its head version on request */
  withheld: boolean;
  rows: Row[];
  status: 'idle' | 'loading' | 'ready' | 'error';
  error: string;
  loadingGaps: ReadonlySet<string>;
  gapErrors: Readonly<Record<string, string>>;
}

export interface FileModelApi {
  model: FileModel;
  /** Make path the current file, loading it if needed; resolves false if it failed or was superseded */
  open(path: string, focusLine: number | null): Promise<boolean>;
  expand(gapKey: string, direction: ExpandDirection): Promise<void>;
  /** Expand context so the head-side line has a row; resolves its row key */
  revealLine(line: number): Promise<string | null>;
  expandAll(): Promise<void>;
  /** Load the head version of a generated file (diff withheld) as read-only lines */
  showWithheld(path: string): Promise<boolean>;
  /** Latest rows, for event handlers */
  rowsRef: { readonly current: Row[] };
  currentPath(): string | null;
}

async function fetchRange(path: string, from: number, to: number): Promise<{ lines: SourceLine[]; total: number }> {
  const lines: SourceLine[] = [];
  let total = 0;
  for (let start = from; start <= to; start += MAX_FETCH) {
    const slice = await fetchFileLines(path, start, Math.min(to, start + MAX_FETCH - 1));
    total = slice.total;
    lines.push(...(slice.lines ?? []));
    if (!slice.lines?.length) break;
  }
  return { lines, total };
}

const IDLE: FileModel = { path: null, plain: false, withheld: false, rows: [], status: 'idle', error: '', loadingGaps: new Set(), gapErrors: {} };

export function useFileModel(): FileModelApi {
  const files = useReview((state) => state.index.files);
  const [model, setModel] = useState<FileModel>(IDLE);
  const modelRef = useRef(model);
  const cache = useRef(new Map<string, Row[]>());
  const rowsRef = useRef<Row[]>([]);

  const update = useCallback((change: (current: FileModel) => FileModel) => {
    const next = change(modelRef.current);
    modelRef.current = next;
    rowsRef.current = next.rows;
    if (next.path && next.status === 'ready') cache.current.set(next.path, next.rows);
    setModel(next);
  }, []);

  const open = useCallback(async (path: string, focusLine: number | null) => {
    const plain = !files.has(path);
    if (modelRef.current.path === path && modelRef.current.status === 'ready') return true;
    const cached = cache.current.get(path);
    const diff = files.get(path);
    if (cached || diff) {
      const withheld = Boolean(cached && diff?.generated);
      update(() => ({ ...IDLE, path, plain, withheld, rows: cached ?? buildDiffRows(diff!), status: 'ready' }));
      return true;
    }
    update(() => ({ ...IDLE, path, plain, status: 'loading' }));
    const center = Math.max(1, focusLine ?? 1);
    try {
      const { lines, total } = await fetchRange(path, Math.max(1, center - PLAIN_WINDOW), center + PLAIN_WINDOW);
      if (modelRef.current.path !== path) return false;
      update((current) => ({ ...current, rows: buildPlainRows(lines, total), status: 'ready' }));
      return true;
    } catch (error) {
      if (modelRef.current.path === path) update((current) => ({ ...current, status: 'error', error: String((error as Error).message ?? error) }));
      return false;
    }
  }, [files, update]);

  const loadGap = useCallback(async (path: string, gapKey: string, from: number, to: number) => {
    update((current) => ({ ...current, loadingGaps: new Set([...current.loadingGaps, gapKey]) }));
    const done = (current: FileModel) => {
      const loadingGaps = new Set(current.loadingGaps);
      loadingGaps.delete(gapKey);
      return loadingGaps;
    };
    try {
      const { lines } = await fetchRange(path, from, to);
      if (modelRef.current.path !== path) return;
      update((current) => ({
        ...current, loadingGaps: done(current),
        rows: expandGap(current.rows, gapKey, from, to, lines, { plain: current.plain || current.withheld }),
      }));
    } catch (error) {
      if (modelRef.current.path !== path) return;
      update((current) => ({
        ...current, loadingGaps: done(current),
        gapErrors: { ...current.gapErrors, [gapKey]: String((error as Error).message ?? error) },
      }));
    }
  }, [update]);

  /** Expand part of a gap; a second request for a gap already loading waits for the first */
  const inflight = useRef(new Map<string, Promise<void>>());
  const expandRange = useCallback((gapKey: string, from: number, to: number): Promise<void> => {
    const path = modelRef.current.path;
    if (!path) return Promise.resolve();
    const flightKey = `${path}\n${gapKey}`;
    const pending = inflight.current.get(flightKey);
    if (pending) return pending;
    const promise = loadGap(path, gapKey, from, to).finally(() => inflight.current.delete(flightKey));
    inflight.current.set(flightKey, promise);
    return promise;
  }, [loadGap]);

  const expand = useCallback(async (gapKey: string, direction: ExpandDirection) => {
    const gap = rowsRef.current.find((row) => row.key === gapKey);
    if (!gap || gap.type !== 'gap') return;
    const [from, to] = gapRange(gap, direction);
    await expandRange(gapKey, from, to);
  }, [expandRange]);

  const revealLine = useCallback(async (line: number) => {
    // A second pass covers waiting on an in-flight expansion of a different part of the gap
    for (let attempt = 0; attempt < 2; attempt++) {
      const existing = lineRowKey(rowsRef.current, line);
      if (existing) return existing;
      const gap = findGapContaining(rowsRef.current, line);
      if (!gap) return null;
      await expandRange(gap.key, Math.max(gap.from, line - REVEAL_RADIUS), Math.min(gap.to, line + REVEAL_RADIUS));
    }
    return lineRowKey(rowsRef.current, line);
  }, [expandRange]);

  const expandAll = useCallback(async () => {
    const gaps = rowsRef.current.filter((row) => row.type === 'gap');
    await Promise.all(gaps.map((gap) => (gap.type === 'gap' ? expandRange(gap.key, gap.from, gap.to) : undefined)));
  }, [expandRange]);

  const showWithheld = useCallback(async (path: string) => {
    if (modelRef.current.path !== path) return false;
    update((current) => ({ ...current, withheld: true, status: 'loading', error: '' }));
    try {
      const { lines, total } = await fetchRange(path, 1, MAX_FETCH);
      if (modelRef.current.path !== path) return false;
      update((current) => ({ ...current, rows: buildPlainRows(lines, total), status: 'ready' }));
      return true;
    } catch (error) {
      if (modelRef.current.path === path) {
        update((current) => ({ ...current, withheld: false, status: 'error', error: String((error as Error).message ?? error) }));
      }
      return false;
    }
  }, [update]);

  const currentPath = useCallback(() => modelRef.current.path, []);
  return useMemo(
    () => ({ model, open, expand, revealLine, expandAll, showWithheld, rowsRef, currentPath }),
    [model, open, expand, revealLine, expandAll, showWithheld, currentPath],
  );
}
