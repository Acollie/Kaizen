// One package box in the Overview map (SVG): path, size bar, risk stripe, finding
// counts, focus count and the two coverage meters: "you" (what you reviewed, the only
// verified coverage) and "AI" (what the AI looked at, which verifies nothing).

import { memo, type MouseEvent } from 'react';
import type { PackageInfo } from '../../lib/packageMap';
import { percent } from '../../lib/packageMap';
import { riskColor } from '../../lib/risk';
import { AI_LOOKED_HINT, YOU_REVIEWED_HINT } from '../../lib/coverageCopy';
import type { Severity } from '../../types';
import type { PlacedPackage } from './overviewLayout';

const TEXT_X = 14;
const RIGHT_PAD = 12;
const TITLE_CHAR_WIDTH = 7.9;
const METER_X = 38;
const METER_VALUE_WIDTH = 58;
const COUNTED: readonly Severity[] = ['critical', 'warning', 'suggestion'];

/** Size bar scale: log of changed lines, relative to the largest package */
export function sizeScale(packages: readonly PackageInfo[]): number {
  return Math.max(1, ...packages.map((pkg) => Math.log10(1 + pkg.added + pkg.removed)));
}

/** "web/review/src/components" -> dir "web/review/src/", base "components"; long dirs lose leading segments */
export function titleParts(label: string, maxChars: number): { dir: string; base: string } {
  const index = label.lastIndexOf('/');
  const base = index === -1 ? label : label.slice(index + 1);
  let dir = index === -1 ? '' : label.slice(0, index + 1);
  if (base.length >= maxChars) return { dir: '', base: `${base.slice(0, maxChars - 1)}…` };
  while (dir.length + base.length > maxChars && dir.includes('/')) {
    const segments = dir.replace(/^…\//, '').split('/').filter(Boolean);
    if (segments.length <= 1) {
      dir = '…/';
      break;
    }
    dir = `…/${segments.slice(1).join('/')}/`;
  }
  return { dir, base };
}

interface PackageNodeProps {
  node: PlacedPackage;
  selected: boolean;
  scale: number;
  onSelect(key: string): void;
  onHover(key: string | null, event?: MouseEvent): void;
}

export const PackageNode = memo(function PackageNode({ node, selected, scale, onSelect, onHover }: PackageNodeProps) {
  const { pkg, width, height } = node;
  const inner = width - TEXT_X - RIGHT_PAD;
  const title = titleParts(pkg.label, Math.floor(inner / TITLE_CHAR_WIDTH));
  const className = ['ov-node', selected ? 'ov-selected' : '', pkg.dim ? 'ov-dim' : ''].filter(Boolean).join(' ');
  const functionsText = pkg.functions.length ? ` · ${pkg.functions.length} fn` : '';
  return (
    <g
      className={className}
      transform={`translate(${node.x},${node.y})`}
      role="button"
      tabIndex={0}
      aria-label={`${pkg.label}: ${pkg.files.length} files, ${pkg.functions.length} changed functions, max risk ${Math.round(pkg.maxRisk)}`}
      onClick={(event) => { event.stopPropagation(); onSelect(pkg.key); }}
      onKeyDown={(event) => { if (event.key === 'Enter') onSelect(pkg.key); }}
      onMouseEnter={(event) => onHover(pkg.key, event)}
      onMouseMove={(event) => onHover(pkg.key, event)}
      onMouseLeave={() => onHover(null)}
    >
      <rect className="ov-box" width={width} height={height} rx={6} />
      <rect x={4} y={8} width={3} height={height - 16} rx={1.5} fill={riskColor(pkg.maxRisk)} />
      <text className="ov-title" x={TEXT_X} y={22}>
        {title.dir && <tspan className="ov-title-dir">{title.dir}</tspan>}
        <tspan className="ov-title-base">{title.base}</tspan>
      </text>
      <text className="ov-sub" x={TEXT_X} y={40}>{pkg.files.length} {pkg.files.length === 1 ? 'file' : 'files'}{functionsText}</text>
      <text className="ov-sub ov-diffstat" x={width - RIGHT_PAD} y={40} textAnchor="end">
        <tspan className="ov-add">+{pkg.added}</tspan> <tspan className="ov-del">−{pkg.removed}</tspan>
      </text>
      <SizeBar pkg={pkg} x={TEXT_X} y={47} width={inner} scale={scale} />
      <FocusAndFindings pkg={pkg} y={70} right={width - RIGHT_PAD} />
      <Meter label="you" share={pkg.you.share} x={TEXT_X} y={84} width={inner} tone="you"
        hint={`You reviewed ${pkg.you.share === null ? 'unknown' : percent(pkg.you.share)} (verified). ${YOU_REVIEWED_HINT}`} />
      <Meter label="AI" share={pkg.ai.share} x={TEXT_X} y={99} width={inner} tone="ai"
        hint={`AI looked at ${pkg.ai.share === null ? 'an unknown share' : percent(pkg.ai.share)} (not verified). ${AI_LOOKED_HINT}`} />
    </g>
  );
});

function SizeBar({ pkg, x, y, width, scale }: { pkg: PackageInfo; x: number; y: number; width: number; scale: number }) {
  const lines = pkg.added + pkg.removed;
  const total = Math.max(2, (Math.log10(1 + lines) / scale) * width);
  const added = lines ? (pkg.added / lines) * total : 0;
  return (
    <g aria-hidden="true">
      <rect x={x} y={y} width={width} height={3} rx={1.5} className="ov-track" />
      {added > 0 && <rect x={x} y={y} width={added} height={3} rx={1.5} fill="var(--diff-add-mark)" />}
      {total - added > 0.5 && <rect x={x + added} y={y} width={total - added} height={3} rx={1.5} fill="var(--diff-del-mark)" />}
    </g>
  );
}

function FocusAndFindings({ pkg, y, right }: { pkg: PackageInfo; y: number; right: number }) {
  const counts = COUNTED.filter((severity) => pkg.severity[severity] > 0);
  return (
    <g>
      <text className="ov-meta" x={TEXT_X} y={y}>
        {pkg.focusCount > 0 && <tspan className="ov-star" fill={riskColor(pkg.maxRisk)}>★ {pkg.focusCount}  </tspan>}
        <tspan>risk </tspan><tspan className="ov-meta-value">{Math.round(pkg.maxRisk)}</tspan>
        {pkg.dim && <tspan className="ov-meta-test">  · {percent(pkg.testShare)} tests</tspan>}
      </text>
      {counts.map((severity, order) => {
        const x = right - (counts.length - 1 - order) * 26 - 9;
        return (
          <g key={severity} transform={`translate(${x},${y - 4})`} className="ov-count">
            <title>{`${pkg.severity[severity]} ${severity} findings`}</title>
            <circle r={8} fill={`var(--sev-${severity}-soft)`} />
            <text textAnchor="middle" dy="0.35em" fill={`var(--sev-${severity})`}>{pkg.severity[severity]}</text>
          </g>
        );
      })}
    </g>
  );
}

interface MeterProps {
  label: string;
  share: number | null;
  x: number;
  y: number;
  width: number;
  tone: 'you' | 'ai';
  hint: string;
}

function Meter({ label, share, x, y, width, tone, hint }: MeterProps) {
  const barX = x + METER_X - TEXT_X;
  const barWidth = width - (METER_X - TEXT_X) - METER_VALUE_WIDTH;
  const value = share === null ? 'unknown' : percent(share);
  return (
    <g className={`ov-meter ov-meter-${tone}`}>
      <title>{hint}</title>
      <text className="ov-meter-label" x={x} y={y + 4}>{label}</text>
      <rect x={barX} y={y} width={barWidth} height={3} rx={1.5} className={share === null ? 'ov-track ov-track-unknown' : 'ov-track'} />
      {share !== null && share > 0 && <rect className="ov-meter-fill" x={barX} y={y} width={Math.max(2, barWidth * share)} height={3} rx={1.5} />}
      <text className={`ov-meter-value${share === null ? ' ov-unknown' : ''}`} x={x + width} y={y + 4} textAnchor="end">{value}</text>
    </g>
  );
}
