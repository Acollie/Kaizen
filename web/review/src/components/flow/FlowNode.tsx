// One function box in the flow diagram (SVG).

import { memo, type MouseEvent } from 'react';
import type { Finding } from '../../types';
import type { LaidOutNode } from '../../lib/flowLayout';
import { topSeverity } from '../../lib/findings';
import { riskColor } from '../../lib/risk';
import type { AnimatedBox } from './useTween';

interface FlowNodeProps {
  node: LaidOutNode;
  box: AnimatedBox;
  selected: boolean;
  findings: readonly Finding[] | undefined;
  onSelect(id: string): void;
  onHover(id: string | null, event?: MouseEvent): void;
}

const TEXT_X = 14;

export const FlowNode = memo(function FlowNode({ node, box, selected, findings, onSelect, onHover }: FlowNodeProps) {
  const changed = node.kind === 'changed';
  const className = ['fl-node', `fl-${node.kind}`, selected ? 'fl-selected' : ''].filter(Boolean).join(' ');
  return (
    <g
      className={className}
      transform={`translate(${box.x},${box.y})`}
      opacity={box.opacity}
      role="button"
      aria-label={`${node.name}${node.file ? `, ${node.file}:${node.line}` : ', external'}`}
      onClick={(event) => { event.stopPropagation(); onSelect(node.id); }}
      onMouseEnter={(event) => onHover(node.id, event)}
      onMouseMove={(event) => onHover(node.id, event)}
      onMouseLeave={() => onHover(null)}
    >
      <rect className="fl-box" width={box.width} height={box.height} rx={6} />
      {changed && <rect className="fl-stripe" x={4} y={7} width={3} height={box.height - 14} rx={1.5} fill={riskColor(node.risk_score)} />}
      <text className="fl-name" x={TEXT_X} y={17}>
        {node.focus && <tspan className="fl-star" fill={riskColor(node.risk_score)}>★ </tspan>}
        {node.label}
      </text>
      <text className="fl-sub" x={TEXT_X} y={31}>{node.subLabel}</text>
      {findings && findings.length > 0 && <SeverityBadge findings={findings} x={box.width - 2} />}
    </g>
  );
});

function SeverityBadge({ findings, x }: { findings: readonly Finding[]; x: number }) {
  const severity = topSeverity(findings) ?? 'suggestion';
  return (
    <g className="fl-badge" transform={`translate(${x},2)`}>
      <circle r={8} fill={`var(--sev-${severity})`} />
      <text textAnchor="middle" dy="0.35em">{findings.length}</text>
    </g>
  );
}
