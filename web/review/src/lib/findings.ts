// Pure helpers for severities, finding ownership and clustering.

import type { FileDiff, Finding, FunctionRisk, Severity } from '../types';

export const SEVERITIES: readonly Severity[] = ['critical', 'warning', 'suggestion', 'praise'];

/** What a reviewer should do about each severity; used for headings and chips */
export const SEVERITY_LABEL: Record<Severity, string> = {
  critical: 'Must fix',
  warning: 'Should fix',
  suggestion: 'Consider',
  praise: 'Good',
};

export function severityRank(severity: Severity | string): number {
  const index = SEVERITIES.indexOf(severity as Severity);
  return index === -1 ? SEVERITIES.length : index;
}

/** Severities a reviewer must act on before approving */
export function isUrgent(finding: Finding): boolean {
  return finding.severity === 'critical' || finding.severity === 'warning';
}

/** Highest severity among findings, or null */
export function topSeverity(findings: readonly Finding[]): Severity | null {
  let best: Severity | null = null;
  for (const finding of findings) {
    if (best === null || severityRank(finding.severity) < severityRank(best)) best = finding.severity;
  }
  return best;
}

/** Non-zero counts in display order */
export function severityCounts(findings: readonly Finding[]): [Severity, number][] {
  const counts = new Map<Severity, number>(SEVERITIES.map((severity) => [severity, 0]));
  for (const finding of findings) {
    if (counts.has(finding.severity)) counts.set(finding.severity, (counts.get(finding.severity) ?? 0) + 1);
  }
  return [...counts.entries()].filter(([, count]) => count > 0);
}

export function findingBelongsTo(finding: Finding, fn: FunctionRisk): boolean {
  if (finding.function_id) return finding.function_id === fn.id;
  const line = finding.line ?? 0;
  return finding.file === fn.file && line >= fn.start_line && line <= fn.end_line;
}

/** Index of the hunk holding a commentable (added or context) head-side line, or -1 */
function commentableHunk(file: FileDiff, line: number): number {
  // Later hunks win, like the Go index built by overwriting a map
  let found = -1;
  file.hunks.forEach((hunk, hunkNumber) => {
    if (hunk.lines.some((diffLine) => diffLine.kind !== 'removed' && diffLine.new_line === line)) found = hunkNumber;
  });
  return found;
}

/**
 * True when the finding will be posted as an inline GitHub comment. Mirrors Go's
 * inlineComment (pkg/github/post.go) exactly: `line` must be a commentable head-side
 * line, and when end_line > line, end_line must be commentable too and in the same hunk.
 * Anything else goes into the review body.
 */
export function isInlineFinding(finding: Finding, file: FileDiff | undefined): boolean {
  if (!file || !finding.file || !finding.line || finding.line <= 0) return false;
  const startHunk = commentableHunk(file, finding.line);
  if (startHunk === -1) return false;
  const endLine = finding.end_line ?? 0;
  if (endLine > finding.line) return commentableHunk(file, endLine) === startHunk;
  return true;
}

export function formatLineRef(finding: Finding): string {
  if (!finding.line) return 'file';
  if (finding.end_line && finding.end_line !== finding.line) return `L${finding.line}–${finding.end_line}`;
  return `L${finding.line}`;
}

export interface FindingCluster {
  lead: Finding;
  similar: Finding[];
}

const SIMILAR_LINE_DISTANCE = 3;

/**
 * Groups findings on (nearly) the same lines of the same file with the same category:
 * different reviewer agents often report one problem in different words. The most
 * severe, most confident finding of each cluster leads it.
 */
export function clusterFindings(findings: readonly Finding[]): FindingCluster[] {
  const ordered = [...findings].sort((left, right) =>
    severityRank(left.severity) - severityRank(right.severity) || (right.confidence || 0) - (left.confidence || 0));
  const clusters: FindingCluster[] = [];
  for (const finding of ordered) {
    const home = clusters.find((cluster) => isSimilar(cluster.lead, finding));
    if (home) home.similar.push(finding);
    else clusters.push({ lead: finding, similar: [] });
  }
  return clusters;
}

function isSimilar(left: Finding, right: Finding): boolean {
  if (left.file !== right.file || left.category !== right.category) return false;
  if (!left.line || !right.line) return left.line === right.line;
  return Math.abs(left.line - right.line) <= SIMILAR_LINE_DISTANCE;
}
