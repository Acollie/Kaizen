// Small shared presentational pieces used across panels.

import type { CSSProperties } from 'react';
import type { Finding, Severity } from '../../types';
import { severityCounts, SEVERITY_LABEL } from '../../lib/findings';

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
