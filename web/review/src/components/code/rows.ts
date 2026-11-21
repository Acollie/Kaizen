// Pure row model for the code view: a file becomes a flat list of hunk headers, diff
// lines and "gap" rows (hidden context that can be expanded). Expanding a gap splices
// fetched lines in and keeps any remainder as smaller gaps. Row objects are never
// mutated, so unchanged rows keep their identity and memoised components skip them.

import type { DiffLineKind, FileDiff, Hunk } from '../../types';

export const EXPAND_STEP = 20;

export interface LineRow {
  type: 'line';
  key: string;
  kind: DiffLineKind;
  /** 0 when the line has no base-side number */
  oldLine: number;
  /** 0 for removed lines */
  newLine: number;
  /** Head-side line used for function membership (the previous head line for removals) */
  anchorLine: number;
  text: string;
}

export interface HunkRow {
  type: 'hunk';
  key: string;
  hunkIndex: number;
  range: string;
  header: string;
  /** First and last head-side lines the hunk covers */
  begin: number;
  end: number;
}

export type GapEdge = 'top' | 'middle' | 'bottom';

export interface GapRow {
  type: 'gap';
  key: string;
  /** Head-side line range hidden by this gap (inclusive) */
  from: number;
  to: number;
  /** old line = new line + offset for context lines in this gap */
  offset: number;
  edge: GapEdge;
}

export type Row = LineRow | HunkRow | GapRow;
export type ExpandDirection = 'above' | 'below' | 'all';
export interface SourceLine { n: number; text: string }

const hunkBegin = (hunk: Hunk) => (hunk.new_count === 0 ? hunk.new_start + 1 : hunk.new_start);
const hunkEnd = (hunk: Hunk) => hunkBegin(hunk) + hunk.new_count - 1;
const oldBegin = (hunk: Hunk) => (hunk.old_count === 0 ? hunk.old_start + 1 : hunk.old_start);
const oldEnd = (hunk: Hunk) => oldBegin(hunk) + hunk.old_count - 1;

export function gapRow(from: number, to: number, offset: number, edge: GapEdge): GapRow {
  return { type: 'gap', key: `g${from}-${to}`, from, to, offset, edge };
}

function contextRow(line: SourceLine, offset: number | null): LineRow {
  return {
    type: 'line', key: `n${line.n}`, kind: 'context',
    oldLine: offset === null ? 0 : line.n + offset, newLine: line.n, anchorLine: line.n, text: line.text,
  };
}

function hunkRows(hunk: Hunk, hunkIndex: number, previousNew: number): Row[] {
  const begin = hunkBegin(hunk);
  const rows: Row[] = [{
    type: 'hunk', key: `h${hunkIndex}`, hunkIndex, header: hunk.header ?? '',
    range: `@@ -${hunk.old_start},${hunk.old_count} +${hunk.new_start},${hunk.new_count} @@`,
    begin, end: Math.max(begin, hunkEnd(hunk)),
  }];
  let lastNew = previousNew;
  for (const line of hunk.lines ?? []) {
    if (line.new_line) lastNew = line.new_line;
    rows.push({
      type: 'line', key: line.kind === 'removed' ? `d${line.old_line}` : `n${line.new_line}`,
      kind: line.kind, oldLine: line.old_line, newLine: line.new_line,
      anchorLine: line.new_line || lastNew, text: line.text,
    });
  }
  return rows;
}

/** Rows for a PR file: hunks with expandable gaps between them and at both ends */
export function buildDiffRows(file: FileDiff): Row[] {
  const rows: Row[] = [];
  const expandable = file.status !== 'deleted' && !file.binary;
  file.hunks.forEach((hunk, index) => {
    const previous = file.hunks[index - 1];
    const gapFrom = previous ? hunkEnd(previous) + 1 : 1;
    const gapTo = hunkBegin(hunk) - 1;
    if (expandable && gapTo >= gapFrom) {
      const offset = previous ? oldEnd(previous) - hunkEnd(previous) : oldBegin(hunk) - hunkBegin(hunk);
      rows.push(gapRow(gapFrom, gapTo, offset, previous ? 'middle' : 'top'));
    }
    rows.push(...hunkRows(hunk, index, previous ? hunkEnd(previous) : 0));
  });
  const last = file.hunks[file.hunks.length - 1];
  if (expandable && last && file.line_count > hunkEnd(last)) {
    rows.push(gapRow(hunkEnd(last) + 1, file.line_count, oldEnd(last) - hunkEnd(last), 'bottom'));
  }
  return rows;
}

/** Rows for a read-only file window (a file outside the diff) */
export function buildPlainRows(lines: readonly SourceLine[], total: number): Row[] {
  const rows: Row[] = [];
  const first = lines[0]?.n ?? 1;
  const last = lines[lines.length - 1]?.n ?? 0;
  if (first > 1) rows.push(gapRow(1, first - 1, 0, 'top'));
  for (const line of lines) rows.push(contextRow(line, null));
  if (total > last) rows.push(gapRow(last + 1, total, 0, 'bottom'));
  return rows;
}

/** The head-side lines an expander should fetch */
export function gapRange(gap: GapRow, direction: ExpandDirection, step = EXPAND_STEP): [number, number] {
  if (direction === 'above') return [Math.max(gap.from, gap.to - step + 1), gap.to];
  if (direction === 'below') return [gap.from, Math.min(gap.to, gap.from + step - 1)];
  return [gap.from, gap.to];
}

/** Replace lines from..to of a gap with fetched lines; remainders stay as gaps */
export function expandGap(rows: readonly Row[], gapKey: string, from: number, to: number,
  lines: readonly SourceLine[], options: { plain: boolean }): Row[] {
  const position = rows.findIndex((row) => row.key === gapKey);
  const gap = rows[position];
  if (!gap || gap.type !== 'gap') return [...rows];
  const replacement: Row[] = [];
  if (from > gap.from) replacement.push(gapRow(gap.from, from - 1, gap.offset, gap.edge === 'top' ? 'top' : 'middle'));
  for (const line of lines) {
    if (line.n >= from && line.n <= to) replacement.push(contextRow(line, options.plain ? null : gap.offset));
  }
  if (to < gap.to) replacement.push(gapRow(to + 1, gap.to, gap.offset, gap.edge === 'bottom' ? 'bottom' : 'middle'));
  return [...rows.slice(0, position), ...replacement, ...rows.slice(position + 1)];
}

export function findGapContaining(rows: readonly Row[], line: number): GapRow | null {
  for (const row of rows) if (row.type === 'gap' && line >= row.from && line <= row.to) return row;
  return null;
}

/** Row index of the hunk header at or before rowIndex, or -1 */
export function hunkRowAtOrBefore(rows: readonly Row[], rowIndex: number): number {
  for (let index = Math.min(rowIndex, rows.length - 1); index >= 0; index--) if (rows[index]?.type === 'hunk') return index;
  return -1;
}

export function hunkRowIndices(rows: readonly Row[]): number[] {
  const indices: number[] = [];
  rows.forEach((row, index) => { if (row.type === 'hunk') indices.push(index); });
  return indices;
}

/** The hunk header the cursor moves to for ]c / [c (count hunks away), or -1 */
export function stepHunk(rows: readonly Row[], cursorIndex: number, delta: number): number {
  const hunks = hunkRowIndices(rows);
  if (!hunks.length) return -1;
  const current = hunks.lastIndexOf(hunkRowAtOrBefore(rows, cursorIndex));
  if (current === -1) return delta > 0 ? hunks[Math.min(hunks.length - 1, delta - 1)] ?? -1 : -1;
  return hunks[Math.max(0, Math.min(hunks.length - 1, current + delta))] ?? -1;
}

function scanForGap(rows: readonly Row[], start: number, step: 1 | -1, stopAtHunk: boolean): GapRow | null {
  for (let index = start; index >= 0 && index < rows.length; index += step) {
    const row = rows[index];
    if (!row) break;
    if (row.type === 'gap') return row;
    if (stopAtHunk && row.type === 'hunk') return null;
  }
  return null;
}

/**
 * The gap zo/zc (expandAbove/expandBelow) act on: the nearest gap above or below the
 * hunk at the cursor. Without hunks (a read-only file) it is the nearest gap above or
 * below the cursor itself.
 */
export function gapForExpansion(rows: readonly Row[], cursorIndex: number, direction: 'above' | 'below'): GapRow | null {
  const hunks = hunkRowIndices(rows);
  if (!hunks.length) {
    const from = Math.max(0, cursorIndex);
    return direction === 'above'
      ? scanForGap(rows, from, -1, false) ?? scanForGap(rows, from, 1, false)
      : scanForGap(rows, from, 1, false) ?? scanForGap(rows, from, -1, false);
  }
  let hunkIndex = hunkRowAtOrBefore(rows, cursorIndex);
  if (hunkIndex === -1) hunkIndex = hunks[0] ?? 0;
  return direction === 'above' ? scanForGap(rows, hunkIndex - 1, -1, true) : scanForGap(rows, hunkIndex + 1, 1, true);
}

export function lineRowKey(rows: readonly Row[], line: number): string | null {
  for (const row of rows) if (row.type === 'line' && row.newLine === line) return row.key;
  return null;
}

/** Number of hidden lines across all gaps */
export function hiddenLineCount(rows: readonly Row[]): number {
  return rows.reduce((total, row) => total + (row.type === 'gap' ? row.to - row.from + 1 : 0), 0);
}
