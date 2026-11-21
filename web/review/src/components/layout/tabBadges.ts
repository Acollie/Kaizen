// Pure badge logic for the context panel's tab bar.

import type { Finding } from '../../types';

export type BadgeTone = 'critical' | 'warning' | 'neutral';

export interface TabBadge {
  text: string;
  tone: BadgeTone;
  title: string;
}

const MAX_SHOWN = 99;

export function badgeText(count: number): string {
  return count > MAX_SHOWN ? `${MAX_SHOWN}+` : String(count);
}

/**
 * Review tab: the urgent count (critical + warning) in the colour of the worst of
 * them; with nothing urgent, the total in neutral; nothing when there are no findings.
 */
export function reviewBadge(findings: readonly Finding[]): TabBadge | null {
  const critical = findings.filter((finding) => finding.severity === 'critical').length;
  const warning = findings.filter((finding) => finding.severity === 'warning').length;
  const urgent = critical + warning;
  if (urgent > 0) {
    return {
      text: badgeText(urgent),
      tone: critical > 0 ? 'critical' : 'warning',
      title: `${urgent} to fix (${critical} must fix, ${warning} should fix)`,
    };
  }
  if (findings.length === 0) return null;
  return { text: badgeText(findings.length), tone: 'neutral', title: `${findings.length} findings, none urgent` };
}

/** Activity tab: events that arrived since the tab was last viewed */
export function activityBadge(total: number, seen: number): TabBadge | null {
  const unseen = Math.max(0, total - seen);
  if (unseen === 0) return null;
  return { text: badgeText(unseen), tone: 'neutral', title: `${unseen} new ${unseen === 1 ? 'event' : 'events'}` };
}
