// Hover card for a flow node (an overlay surface).

import type { Finding, FunctionRisk, GraphNode } from '../../types';
import { qualifiedName } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { SeverityCounts } from '../shared/bits';

export interface TooltipTarget { id: string; left: number; top: number }

interface FlowTooltipProps {
  node: GraphNode;
  fn: FunctionRisk | undefined;
  findings: readonly Finding[] | undefined;
  left: number;
  top: number;
}

export function FlowTooltip({ node, fn, findings, left, top }: FlowTooltipProps) {
  const reason = fn?.risk_reasons?.[0];
  const fanIn = fn ? `${fn.fan_in_approximate ? '≈' : ''}${fn.fan_in}` : String(node.fan_in);
  return (
    <div className="fl-tooltip" style={{ left, top }} role="tooltip">
      <div className="fl-tt-name">{fn ? qualifiedName(fn) : node.name}</div>
      <div className="fl-tt-loc">{node.file ? `${node.file}:${node.line}` : 'external'}</div>
      {node.kind === 'changed' && (
        <div className="fl-tt-stats">
          <span>risk <b style={{ color: riskColor(node.risk_score) }}>{Math.round(node.risk_score)}</b></span>
          <span>complexity <b>{node.complexity}</b></span>
          <span>fan-in <b>{fanIn}</b></span>
        </div>
      )}
      {node.kind !== 'changed' && <div className="fl-tt-kind">{node.kind === 'external' ? 'Outside this repository' : `Unchanged ${node.kind}`}</div>}
      {reason && <div className="fl-tt-reason">{reason}</div>}
      {findings && findings.length > 0 && (
        <div className="fl-tt-findings"><SeverityCounts findings={findings} skipPraise={false} /> <span>AI findings</span></div>
      )}
    </div>
  );
}

/** Keep the card inside the canvas: flip left/up near the edges */
export function placeTooltip(pointerX: number, pointerY: number, width: number, height: number): { left: number; top: number } {
  const cardWidth = 260;
  const cardHeight = 120;
  const left = pointerX + 14 + cardWidth > width ? Math.max(4, pointerX - cardWidth - 10) : pointerX + 14;
  const top = pointerY + 14 + cardHeight > height ? Math.max(4, pointerY - cardHeight - 10) : pointerY + 14;
  return { left, top };
}
