// A compact, read-only code snippet for the verify queue: ±8 head-side lines around a
// finding with the diff's colouring. Lines in the diff come from its hunks (removed
// lines included where they were); other head lines come from /api/file. Pure.

import type { DiffLineKind, FileDiff, Finding } from '../../types';

export const SNIPPET_CONTEXT = 8;
/** Rows the snippet box always reserves, so moving between findings never shifts the layout */
export const SNIPPET_ROWS = 2 * SNIPPET_CONTEXT + 3;

export interface SnippetRange {
  from: number;
  to: number;
  /** The finding's own lines */
  markFrom: number;
  markTo: number;
}

export interface SnippetRow {
  key: string;
  kind: DiffLineKind;
  oldLine: number;
  newLine: number;
  text: string;
  /** A head line not in the diff whose text has not loaded yet */
  pending: boolean;
  marked: boolean;
}

/** The head-side lines to show for a finding; file-level findings show the first hunk */
export function snippetRange(finding: Pick<Finding, 'line' | 'end_line'>, file: FileDiff | undefined): SnippetRange | null {
  const lineCount = file?.line_count && file.line_count > 0 ? file.line_count : Number.POSITIVE_INFINITY;
  if (finding.line && finding.line > 0) {
    const end = finding.end_line && finding.end_line > finding.line ? finding.end_line : finding.line;
    return {
      from: Math.max(1, finding.line - SNIPPET_CONTEXT),
      to: Math.min(lineCount, end + SNIPPET_CONTEXT),
      markFrom: finding.line,
      markTo: end,
    };
  }
  const first = file?.hunks[0];
  if (!first) return null;
  const from = Math.max(1, first.new_start);
  return { from, to: Math.min(lineCount, from + 2 * SNIPPET_CONTEXT), markFrom: 0, markTo: -1 };
}

/** Rows in display order: diff lines in range (with removals), the gaps filled from `lines` */
export function snippetRows(file: FileDiff | undefined, range: SnippetRange, lines: ReadonlyMap<number, string> | null): SnippetRow[] {
  const entries: { sort: number; row: SnippetRow }[] = [];
  const covered = new Set<number>();
  const marked = (line: number) => line >= range.markFrom && line <= range.markTo;

  for (const hunk of file?.hunks ?? []) {
    let lastNew = hunk.new_start - 1;
    hunk.lines.forEach((line, index) => {
      if (line.kind === 'removed') {
        if (lastNew >= range.from - 1 && lastNew <= range.to) {
          entries.push({ sort: lastNew * 2 + 1, row: { key: `r${hunk.new_start}:${index}`, kind: 'removed', oldLine: line.old_line, newLine: 0, text: line.text, pending: false, marked: false } });
        }
        return;
      }
      lastNew = line.new_line;
      if (line.new_line < range.from || line.new_line > range.to || covered.has(line.new_line)) return;
      covered.add(line.new_line);
      entries.push({ sort: line.new_line * 2, row: { key: `n${line.new_line}`, kind: line.kind, oldLine: line.old_line, newLine: line.new_line, text: line.text, pending: false, marked: marked(line.new_line) } });
    });
  }

  for (let line = range.from; line <= range.to; line++) {
    if (covered.has(line)) continue;
    const text = lines?.get(line);
    if (lines && text === undefined) continue; // past the end of the file
    entries.push({ sort: line * 2, row: { key: `n${line}`, kind: 'context', oldLine: 0, newLine: line, text: text ?? '', pending: text === undefined, marked: marked(line) } });
  }

  return entries.sort((left, right) => left.sort - right.sort).map((entry) => entry.row);
}

/** True when some head line in range is not in the diff, so /api/file must fill it */
export function needsFetch(file: FileDiff | undefined, range: SnippetRange): boolean {
  const inDiff = new Set<number>();
  for (const hunk of file?.hunks ?? []) for (const line of hunk.lines) if (line.kind !== 'removed') inDiff.add(line.new_line);
  for (let line = range.from; line <= range.to; line++) if (!inDiff.has(line)) return true;
  return false;
}
