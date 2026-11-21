// Pure view-model helpers for the review panel: verdict wording, counts sentence,
// severity sections of clustered findings, and finding excerpts.

import type { Finding, Severity, Verdict } from '../../types';
import { clusterFindings, SEVERITIES, SEVERITY_LABEL, severityCounts, type FindingCluster } from '../../lib/findings';
import { splitPath } from '../../lib/format';

export interface VerdictView {
  label: string;
  tone: 'critical' | 'warning' | 'praise';
}

export function verdictView(verdict: Verdict | string): VerdictView {
  if (verdict === 'request_changes') return { label: 'Changes requested', tone: 'critical' };
  if (verdict === 'approve') return { label: 'Approved', tone: 'praise' };
  return { label: 'Comments', tone: 'warning' };
}

const COUNT_WORDS: Record<Severity, [string, string]> = {
  critical: ['must fix', 'must fix'],
  warning: ['should fix', 'should fix'],
  suggestion: ['suggestion', 'suggestions'],
  praise: ['good point', 'good points'],
};

/** "2 must fix, 6 should fix and 3 suggestions"; "No findings" when empty */
export function countsSentence(findings: readonly Finding[]): string {
  const parts = severityCounts(findings).map(([severity, count]) => {
    const [singular, plural] = COUNT_WORDS[severity];
    return `${count} ${count === 1 ? singular : plural}`;
  });
  if (parts.length === 0) return 'No findings';
  if (parts.length === 1) return parts[0]!;
  return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;
}

export interface SeveritySection {
  severity: Severity;
  title: string;
  clusters: FindingCluster[];
}

/** Clustered findings split into Must fix / Should fix / Consider / Good, empty sections dropped */
export function severitySections(findings: readonly Finding[]): SeveritySection[] {
  const clusters = clusterFindings(findings);
  return SEVERITIES
    .map((severity) => ({ severity, title: SEVERITY_LABEL[severity], clusters: clusters.filter((cluster) => cluster.lead.severity === severity) }))
    .filter((section) => section.clusters.length > 0);
}

const EXCERPT_LIMIT = 220;

/** First sentence of a Markdown body as plain text (markup stripped), for a 2-line preview */
export function firstSentence(markdown: string): string {
  const firstParagraph = markdown.replace(/\r\n/g, '\n').split(/\n\s*\n|```/).map((part) => part.trim()).find(Boolean) ?? '';
  const plain = firstParagraph
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/(\*\*|__|`)/g, '')
    .replace(/^\s*[-*]\s+/gm, '')
    .replace(/\s+/g, ' ')
    .trim();
  const match = plain.match(/^.+?[.!?](?=\s|$)/);
  const sentence = match ? match[0] : plain;
  return sentence.length > EXCERPT_LIMIT ? `${sentence.slice(0, EXCERPT_LIMIT - 1)}…` : sentence;
}

/** "config.go:118" or "config.go" for file-level findings */
export function locationLabel(finding: Pick<Finding, 'file' | 'line'>): string {
  const base = splitPath(finding.file).base || 'PR';
  return finding.line ? `${base}:${finding.line}` : base;
}

/** Distinct agents behind a cluster, lead first */
export function clusterAgents(cluster: FindingCluster): string[] {
  const agents = [cluster.lead, ...cluster.similar].map((finding) => finding.source || 'lead');
  return [...new Set(agents)];
}

export { focusFunctions, riskyFiles } from '../../state/viewing';
