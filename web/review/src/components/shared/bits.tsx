// Small shared presentational pieces used across panels.

import type { CSSProperties } from 'react';
import type { Finding, Severity } from '../../types';
import { severityCounts, SEVERITY_LABEL } from '../../lib/findings';
import { confidenceView } from '../../lib/confidence';

const AGENT_COLORS: Record<string, string> = {
  lead: 'var(--agent-lead)',
  'hotspot-reviewer': 'var(--agent-hotspot)',
  'security-reviewer': 'var(--agent-security)',
  'impact-analyst': 'var(--agent-impact)',
};

export function agentColor(agent: string | undefined): string {
  return AGENT_COLORS[agent || 'lead'] ?? 'var(--agent-other)';
}

/** "● hotspot-reviewer" in the agent's colour */
export function AgentTag({ agent }: { agent: string | undefined }) {
  const name = agent || 'lead';
  return <span className="agent" style={{ '--agent-color': agentColor(name) } as CSSProperties}>{name}</span>;
}

export function SeverityDot({ severity }: { severity: Severity }) {
  return <span className={`sev-dot sev-${severity}`} aria-label={SEVERITY_LABEL[severity]} />;
}

/** Compact per-severity counts, e.g. [2] [1], coloured by severity */
export function SeverityCounts({ findings, skipPraise = true }: { findings: readonly Finding[]; skipPraise?: boolean }) {
  return (
    <>
      {severityCounts(findings)
        .filter(([severity]) => !(skipPraise && severity === 'praise'))
        .map(([severity, count]) => (
          <span key={severity} className={`count sev-${severity}`} title={`${count} ${SEVERITY_LABEL[severity].toLowerCase()}`}>{count}</span>
        ))}
    </>
  );
}

/** Severity first and prominent: "Should fix" in the severity colour */
export function SeverityChip({ severity }: { severity: Severity }) {
  return <span className={`sev-chip sev-${severity}`}>{SEVERITY_LABEL[severity] ?? severity}</span>;
}

/** Confidence as a quiet low / medium / high word; the number is in the tooltip */
export function ConfidenceWord({ confidence }: { confidence: number | undefined }) {
  const view = confidenceView(confidence);
  if (!view) return null;
  return <span className={`conf-word conf-${view.level}`} title={view.title}>{view.word} conf.</span>;
}

/** After the reveal: where an AI finding stands against the reviewer's own pass */
export function PassTag({ kind }: { kind: 'overlap' | 'outside' | null }) {
  if (kind === 'overlap') return <span className="pass-tag pass-tag-overlap" title="One of your comments from before the reveal is at the same spot (±3 lines)">you also found this</span>;
  if (kind === 'outside') return <span className="pass-tag pass-tag-outside" title="You hadn't viewed this file before the reveal">outside your pass</span>;
  return null;
}
