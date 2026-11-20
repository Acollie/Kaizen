// Pure logic for the post-review dialog. What gets posted is the human's review: their
// own comments first, then the AI findings they accepted (with their notes appended).
// Nothing else. Warnings say what they haven't checked yet; they never block.

import type { FileDiff, Finding, Severity } from '../../types';
import { formatLineRef, isInlineFinding, isUrgent, SEVERITIES, severityRank } from '../../lib/findings';
import { plural, qualifiedName } from '../../lib/format';
import { commentToFinding, type ReviewComment } from '../../state/comments';
import { acceptedFindings, untriagedUrgent, withReviewerNote, type TriageMap } from '../../state/triage';
import type { LookProgress } from '../../state/viewing';

const SUMMARY_LABEL: Record<Severity, string> = {
  critical: 'must fix', warning: 'should fix', suggestion: 'to consider', praise: 'good',
};

/** Severity first, then file and line */
export function sortForPost(findings: readonly Finding[]): Finding[] {
  return [...findings].sort((left, right) =>
    severityRank(left.severity) - severityRank(right.severity)
    || left.file.localeCompare(right.file)
    || (left.line ?? 0) - (right.line ?? 0));
}

export interface PostEntry {
  /** As it will be sent (notes appended, comments converted) */
  finding: Finding;
  /** On a diff line: posted as an inline review comment; otherwise listed in the review body */
  inline: boolean;
}

export interface PostPlan {
  /** The reviewer's own comments, in the order they wrote them */
  comments: PostEntry[];
  /** Accepted AI findings, most severe first */
  accepted: PostEntry[];
}

export function planPost(input: {
  comments: readonly ReviewComment[];
  findings: readonly Finding[];
  triage: TriageMap;
  files: ReadonlyMap<string, FileDiff>;
}): PostPlan {
  const entry = (finding: Finding): PostEntry => ({ finding, inline: isInlineFinding(finding, input.files.get(finding.file)) });
  const accepted = sortForPost(acceptedFindings(input.findings, input.triage))
    .map((finding) => withReviewerNote(finding, input.triage[finding.id]?.note));
  return {
    comments: input.comments.map(commentToFinding).map(entry),
    accepted: accepted.map(entry),
  };
}

/** Everything the request sends, comments first */
export function postedFindings(plan: PostPlan): Finding[] {
  return [...plan.comments, ...plan.accepted].map((entry) => entry.finding);
}

/** "2 yours · 3 accepted · 4 inline · 1 in body" */
export function selectionNote(plan: PostPlan): string {
  const entries = [...plan.comments, ...plan.accepted];
  const inline = entries.filter((entry) => entry.inline).length;
  return `${plan.comments.length} yours · ${plan.accepted.length} accepted · ${inline} inline · ${entries.length - inline} in body`;
}

/** "Post 3 accepted findings", "Post 2 comments and 1 accepted finding", "Post summary only" */
export function postButtonLabel(plan: PostPlan): string {
  const parts: string[] = [];
  if (plan.comments.length) parts.push(plural(plan.comments.length, 'comment'));
  if (plan.accepted.length) parts.push(plural(plan.accepted.length, 'accepted finding'));
  return parts.length ? `Post ${parts.join(' and ')}` : 'Post summary only';
}

export interface PostWarningItem {
  label: string;
  /** Set when the AI looked at it: a reminder that this is not a check */
  aiLooked: boolean;
}

export interface PostWarning {
  key: 'untriaged' | 'unviewed' | 'unviewed-packages' | 'ai-text';
  text: string;
  /** Named things to check (focus functions, packages); at most MAX_WARNING_ITEMS */
  items?: PostWarningItem[];
  /** How many more were left out of items */
  more?: number;
}

/** A risky package the reviewer hasn't opened, with whether the AI looked at any of it */
export interface UnviewedPackage {
  label: string;
  aiLooked: boolean;
}

export const MAX_WARNING_ITEMS = 6;

function capped(items: PostWarningItem[]): Pick<PostWarning, 'items' | 'more'> {
  const shown = items.slice(0, MAX_WARNING_ITEMS);
  return { items: shown, ...(items.length > shown.length ? { more: items.length - shown.length } : {}) };
}

/**
 * Things the reviewer may want to check first. Informative only, never blocking.
 * Unviewed focus functions and risky packages are listed by name whether or not the
 * AI looked at them: the AI looking at code is not a review.
 */
export function postWarnings(input: {
  findings: readonly Finding[];
  triage: TriageMap;
  look: LookProgress;
  acceptedCount: number;
  /** Changed functions the AI looked at (to say so next to unviewed ones) */
  aiFunctions?: ReadonlySet<string>;
  /** Risky or flagged packages the reviewer hasn't opened, most important first */
  unviewedPackages?: readonly UnviewedPackage[];
}): PostWarning[] {
  const warnings: PostWarning[] = [];
  const urgent = untriagedUrgent(input.findings, input.triage);
  if (urgent.length) {
    warnings.push({ key: 'untriaged', text: `${plural(urgent.length, 'must-fix or should-fix finding')} not triaged yet.` });
  }
  const unviewed = unviewedSentence(input.look);
  if (unviewed) {
    const aiFunctions = input.aiFunctions ?? new Set<string>();
    const items = input.look.focus.unseen.map((fn) => ({ label: qualifiedName(fn), aiLooked: aiFunctions.has(fn.id) }));
    warnings.push({ key: 'unviewed', text: unviewed, ...(items.length ? capped(items) : {}) });
  }
  const packages = input.unviewedPackages ?? [];
  if (packages.length) {
    warnings.push({
      key: 'unviewed-packages',
      text: `You haven't opened ${plural(packages.length, 'risky package')}:`,
      ...capped(packages.map((pkg) => ({ label: pkg.label, aiLooked: pkg.aiLooked }))),
    });
  }
  if (input.acceptedCount > 0) {
    warnings.push({ key: 'ai-text', text: "You're posting AI-written text as yourself. Make sure you've checked each one." });
  }
  return warnings;
}

function unviewedSentence(look: LookProgress): string {
  const parts: string[] = [];
  if (look.focus.unseen.length) parts.push(plural(look.focus.unseen.length, 'focus function'));
  if (look.files.unseen.length) parts.push(plural(look.files.unseen.length, 'file needing attention', 'files needing attention'));
  return parts.length ? `You haven't looked at ${parts.join(' or ')}.` : '';
}

/** A plain summary of what is being posted, for when the reviewer writes none */
export function generateSummary(accepted: readonly Finding[], commentCount = 0): string {
  const actionable = accepted.filter((finding) => finding.severity !== 'praise');
  const yours = commentCount > 0 ? ` ${plural(commentCount, 'comment')} from me.` : '';
  if (actionable.length === 0) return commentCount > 0 ? `Review:${yours}` : 'Kaizen review: no issues found.';
  const counts = SEVERITIES
    .map((severity) => [severity, actionable.filter((finding) => finding.severity === severity).length] as const)
    .filter(([, count]) => count > 0)
    .map(([severity, count]) => `${count} ${SUMMARY_LABEL[severity]}`);
  const urgent = sortForPost(actionable).filter(isUrgent);
  const lines = [`Kaizen review: ${counts.join(', ')}.${yours}`];
  if (urgent.length > 0) {
    lines.push('', ...urgent.map((finding) => `- **${finding.title}** (\`${finding.file}\` ${formatLineRef(finding)})`));
  }
  return lines.join('\n');
}
