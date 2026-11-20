// Pure placement logic: where finding cards go, which rows carry a changed function's
// risk rail, and which callee a line of code calls (for gd).

import type { Finding, FunctionRef, FunctionRisk } from '../../types';
import { severityRank } from '../../lib/findings';
import type { Row } from './rows';

export interface FindingPlacement {
  /** Head-side line -> findings shown under that line */
  byLine: Map<number, Finding[]>;
  /** Findings with no line (file-level) */
  fileLevel: Finding[];
  /** Findings on lines not currently shown */
  outside: Finding[];
}

/** Findings sit under their last line (end_line), falling back to their first */
export function findingAnchorLine(finding: Finding): number {
  return finding.end_line || finding.line || 0;
}

export function visibleHeadLines(rows: readonly Row[]): Set<number> {
  const lines = new Set<number>();
  for (const row of rows) if (row.type === 'line' && row.newLine) lines.add(row.newLine);
  return lines;
}

const bySeverity = (left: Finding, right: Finding) =>
  severityRank(left.severity) - severityRank(right.severity) || (left.line ?? 0) - (right.line ?? 0);

export function placeFindings(rows: readonly Row[], findings: readonly Finding[]): FindingPlacement {
  const visible = visibleHeadLines(rows);
  const byLine = new Map<number, Finding[]>();
  const fileLevel: Finding[] = [];
  const outside: Finding[] = [];
  for (const finding of findings) {
    if (!finding.line) {
      fileLevel.push(finding);
      continue;
    }
    const anchor = [findingAnchorLine(finding), finding.line].find((line) => visible.has(line));
    if (anchor === undefined) {
      outside.push(finding);
      continue;
    }
    const list = byLine.get(anchor) ?? [];
    list.push(finding);
    byLine.set(anchor, list);
  }
  for (const list of byLine.values()) list.sort(bySeverity);
  return { byLine, fileLevel: fileLevel.sort(bySeverity), outside: outside.sort(bySeverity) };
}

export interface FunctionRails {
  /** Row key -> the changed function whose range contains the row */
  byRow: Map<string, FunctionRisk>;
  /** Row key -> function whose label is drawn on that row (its first visible row) */
  labels: Map<string, FunctionRisk>;
}

/** Map rows to the changed functions that contain them (functions are in line order) */
export function functionRails(rows: readonly Row[], functions: readonly FunctionRisk[]): FunctionRails {
  const byRow = new Map<string, FunctionRisk>();
  const labels = new Map<string, FunctionRisk>();
  const labelled = new Set<string>();
  for (const row of rows) {
    if (row.type !== 'line' || !row.anchorLine) continue;
    const fn = innermost(functions, row.anchorLine);
    if (!fn) continue;
    byRow.set(row.key, fn);
    if (!labelled.has(fn.id)) {
      labelled.add(fn.id);
      labels.set(row.key, fn);
    }
  }
  return { byRow, labels };
}

/** The smallest changed function containing the line (closures nest inside functions) */
function innermost(functions: readonly FunctionRisk[], line: number): FunctionRisk | null {
  let best: FunctionRisk | null = null;
  for (const fn of functions) {
    if (line < fn.start_line || line > fn.end_line) continue;
    if (!best || fn.end_line - fn.start_line < best.end_line - best.start_line) best = fn;
  }
  return best;
}

const escapeRegExp = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/** The candidate whose `name(` appears earliest in the line, or null */
export function findCalledFunction<T extends Pick<FunctionRef, 'name'>>(text: string, candidates: readonly T[]): T | null {
  let best: T | null = null;
  let bestPosition = Infinity;
  for (const candidate of candidates) {
    if (!candidate.name) continue;
    const match = new RegExp(`(?:^|[^\\w])${escapeRegExp(candidate.name)}\\s*\\(`).exec(text);
    if (!match) continue;
    const position = match.index;
    if (position < bestPosition || (position === bestPosition && best && candidate.name.length > best.name.length)) {
      best = candidate;
      bestPosition = position;
    }
  }
  return best;
}
