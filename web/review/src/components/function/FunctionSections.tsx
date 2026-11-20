// Building blocks of the Function tab: header, reasons, metrics, findings and
// caller/callee lists.

import type { ReactNode, Ref } from 'react';
import type { Finding, FunctionRef, FunctionRisk } from '../../types';
import { formatLineRef } from '../../lib/findings';
import { qualifiedName, splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { AgentTag, SeverityDot } from '../shared/bits';
import { metricRows, type RefAction } from './functionData';

export function Section({ title, count, children, sectionRef }: { title: string; count?: number; children: ReactNode; sectionRef?: Ref<HTMLElement> }) {
  return (
    <section className="fn-section" ref={sectionRef}>
      <h3 className="label fn-section-title">{title}{count !== undefined && <span className="fn-section-count">{count}</span>}</h3>
      {children}
    </section>
  );
}

export function FunctionHeader({ fn }: { fn: FunctionRisk }) {
  return (
    <header className="fn-header">
      <div className="fn-title-row">
        <h2 className="fn-name">{fn.focus && <span className="fn-star" style={{ color: riskColor(fn.risk_score) }} title="Kaizen focus">★ </span>}{qualifiedName(fn)}</h2>
        <span className="fn-risk" style={{ color: riskColor(fn.risk_score) }} title="Kaizen risk score (0–100)">
          {Math.round(fn.risk_score)}<span className="fn-risk-unit">risk</span>
        </span>
      </div>
      <div className="fn-meta">
        <span className="fn-loc" title={fn.file}>{splitPath(fn.file).base}:{fn.start_line}–{fn.end_line}</span>
        <span className="fn-diff"><span className="fn-add">+{fn.lines_added}</span> <span className="fn-del">−{fn.lines_removed}</span></span>
        {fn.is_new && <span className="fn-new">new</span>}
      </div>
    </header>
  );
}

export function RiskReasons({ reasons }: { reasons: readonly string[] }) {
  if (!reasons.length) return null;
  return (
    <Section title="Why it's risky">
      <ul className="fn-reasons">{reasons.map((reason) => <li key={reason}>{reason}</li>)}</ul>
    </Section>
  );
}

export function MetricsTable({ fn }: { fn: FunctionRisk }) {
  const rows = metricRows(fn);
  return (
    <Section title="Metrics">
      {!fn.metrics && <p className="fn-note">Detailed metrics are unavailable for this language.</p>}
      <dl className="fn-metrics">
        {rows.map((row) => (
          <div key={row.label} className="fn-metric" title={row.title}>
            <dt>{row.label}</dt>
            <dd className={`fn-level-${row.level}`}>{row.value}</dd>
          </div>
        ))}
      </dl>
    </Section>
  );
}

export function FindingList({ findings, onOpen }: { findings: readonly Finding[]; onOpen(id: string): void }) {
  if (!findings.length) return null;
  return (
    <Section title="AI findings" count={findings.length}>
      <ul className="fn-list">
        {findings.map((finding) => (
          <li key={finding.id}>
            <button type="button" className="fn-row fn-finding" onClick={() => onOpen(finding.id)}>
              <SeverityDot severity={finding.severity} />
              <span className="fn-finding-body">
                <span className="fn-finding-title">{finding.title}</span>
                <span className="fn-finding-meta"><span className="fn-row-loc">{formatLineRef(finding)}</span><AgentTag agent={finding.source} /></span>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </Section>
  );
}

interface RefListProps {
  title: string;
  refs: readonly FunctionRef[];
  actionFor(ref: FunctionRef): RefAction;
  onAction(action: RefAction): void;
  sectionRef?: Ref<HTMLElement>;
}

export function RefList({ title, refs, actionFor, onAction, sectionRef }: RefListProps) {
  return (
    <Section title={title} count={refs.length} sectionRef={sectionRef}>
      {!refs.length && <p className="fn-note">No {title.toLowerCase()} found.</p>}
      <ul className="fn-list">
        {refs.map((ref) => {
          const action = actionFor(ref);
          const changed = action.type === 'selectFunction';
          return (
            <li key={`${ref.id}:${ref.line}`}>
              <button type="button" className={`fn-row${changed ? ' fn-row-changed' : ''}`} title={ref.id}
                disabled={action.type === 'none'} onClick={() => onAction(action)}>
                <span className="fn-row-name">{ref.name}</span>
                <span className="fn-row-loc">{ref.external || !ref.file ? 'external' : `${splitPath(ref.file).base}:${ref.line}`}</span>
              </button>
            </li>
          );
        })}
      </ul>
    </Section>
  );
}
