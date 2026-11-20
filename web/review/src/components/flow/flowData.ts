// Pure helpers behind the flow panel: findings per node, the hint line, layout
// direction and fit-to-view maths.

import type { FileDiff, Finding, FunctionRisk } from '../../types';
import { findingBelongsTo } from '../../lib/findings';
import type { Bounds, FlowDirection, FlowLayout } from '../../lib/flowLayout';
import { plural } from '../../lib/format';

/** Panels at least this wide lay the flow out left to right */
export const WIDE_PANEL = 720;

/** AI findings per changed-function node */
export function findingsByNode(functions: readonly FunctionRisk[], findings: readonly Finding[]): Map<string, Finding[]> {
  const result = new Map<string, Finding[]>();
  for (const fn of functions) {
    const owned = findings.filter((finding) => findingBelongsTo(finding, fn));
    if (owned.length) result.set(fn.id, owned);
  }
  return result;
}

export function chooseDirection(width: number, height: number, expanded: boolean): FlowDirection {
  if (expanded || width >= WIDE_PANEL) return 'LR';
  return width > height * 1.6 ? 'LR' : 'TB';
}

/** Widest rank (layout units) before it wraps, so a narrow panel still renders at a readable ~0.9x */
export function rankExtentFor(panelWidth: number): number {
  if (panelWidth <= 0) return Infinity;
  return Math.max(240, Math.round((panelWidth - 40) / 0.92 / 20) * 20);
}

export interface HintInput {
  layout: FlowLayout;
  scope: 'selection' | 'all';
  /** Node the layout is centred on (selection scope) */
  focusName: string | null;
  /** Highlighted node in whole-PR scope */
  highlightName: string | null;
}

/** "ValidateConfiguration · 1 caller → 5 callees" or "Whole PR · 6 call flows" */
export function flowHint({ layout, scope, focusName, highlightName }: HintInput): string {
  if (scope === 'selection' && focusName) {
    const callers = layout.nodes.filter((node) => node.rank < 0).length;
    const callees = layout.nodes.filter((node) => node.rank > 0).length;
    return `${focusName} · ${plural(callers, 'caller')} → ${plural(callees, 'callee')}`;
  }
  const flows = Math.max(1, layout.groups.filter((group) => group.key !== 'loose').length);
  const prefix = scope === 'selection' ? 'Nothing selected' : 'Whole PR';
  const tail = highlightName ? `${highlightName} highlighted` : 'click a function to follow its flow';
  return `${prefix} · ${plural(flows, 'call flow')} · ${tail}`;
}

export interface ViewTransform { x: number; y: number; k: number }

/** Scale and translate so the bounds fit the viewport, centred, never zooming in past maxScale */
export function fitTransform(bounds: Bounds, width: number, height: number, padding = 20, maxScale = 1.15, minScale = 0.2): ViewTransform {
  const scale = Math.min(
    maxScale,
    (width - padding * 2) / Math.max(1, bounds.width),
    (height - padding * 2) / Math.max(1, bounds.height),
  );
  const k = Math.max(minScale, scale);
  return {
    k,
    x: (width - bounds.width * k) / 2 - bounds.x * k,
    // Tall content that cannot fit starts at the top rather than the middle
    y: bounds.height * k > height - padding * 2 ? padding - bounds.y * k : (height - bounds.height * k) / 2 - bounds.y * k,
  };
}

/** Languages of the changed files, for the empty state ("Python, Swift") */
export function changedLanguages(files: readonly FileDiff[]): string[] {
  return [...new Set(files.map((file) => file.language).filter(Boolean))].sort();
}
