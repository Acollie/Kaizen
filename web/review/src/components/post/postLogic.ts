// Pure logic for the post-review dialog: ordering, the inline/body split, the
// selection and a generated summary when the engine produced none.

import type { FileDiff, Finding, Severity } from '../../types';
import { formatLineRef, isInlineFinding, isUrgent, SEVERITIES, severityRank } from '../../lib/findings';

const SUMMARY_LABEL: Record<Severity, string> = {
  critical: 'must fix', warning: 'should fix', suggestion: 'to consider', praise: 'good',
};

export interface PostPlan {
  /** Findings on a diff line: posted as inline review comments */
  inline: Finding[];
  /** File-level or outside the diff: listed in the review body */
  body: Finding[];
}

/** Severity first, then file and line */
export function sortForPost(findings: readonly Finding[]): Finding[] {
  return [...findings].sort((left, right) =>
    severityRank(left.severity) - severityRank(right.severity)
    || left.file.localeCompare(right.file)
    || (left.line ?? 0) - (right.line ?? 0));
}

export function planPost(findings: readonly Finding[], files: ReadonlyMap<string, FileDiff>): PostPlan {
  const inline: Finding[] = [];
  const body: Finding[] = [];
  for (const finding of sortForPost(findings)) {
    (isInlineFinding(finding, files.get(finding.file)) ? inline : body).push(finding);
  }
  return { inline, body };
}

export function selectedFindings(findings: readonly Finding[], included: Readonly<Record<string, boolean>>): Finding[] {
  return sortForPost(findings.filter((finding) => included[finding.id]));
}

/** "3 selected · 2 inline · 1 in body" */
export function selectionNote(plan: PostPlan, included: Readonly<Record<string, boolean>>): string {
  const inline = plan.inline.filter((finding) => included[finding.id]).length;
  const body = plan.body.filter((finding) => included[finding.id]).length;
  return `${inline + body} selected · ${inline} inline · ${body} in body`;
}

/** A plain summary of the findings for when the engine gave none */
export function generateSummary(findings: readonly Finding[]): string {
  const actionable = findings.filter((finding) => finding.severity !== 'praise');
  if (actionable.length === 0) return 'Kaizen review: no issues found.';
  const counts = SEVERITIES
    .map((severity) => [severity, actionable.filter((finding) => finding.severity === severity).length] as const)
    .filter(([, count]) => count > 0)
    .map(([severity, count]) => `${count} ${SUMMARY_LABEL[severity]}`);
  const urgent = sortForPost(actionable).filter(isUrgent);
  const lines = [`Kaizen review: ${counts.join(', ')}.`];
  if (urgent.length > 0) {
    lines.push('', ...urgent.map((finding) => `- **${finding.title}** (\`${finding.file}\` ${formatLineRef(finding)})`));
  }
  return lines.join('\n');
}
