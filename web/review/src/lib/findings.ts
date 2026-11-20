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

/** True when a finding sits on a head-side line shown in the diff (it can become an inline GitHub comment) */
export function isInlineFinding(finding: Finding, file: FileDiff | undefined): boolean {
  const line = finding.end_line || finding.line;
  if (!file || !finding.line || !line) return false;
  return file.hunks.some((hunk) => hunk.lines.some((diffLine) => diffLine.kind !== 'removed' && diffLine.new_line === line));
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
