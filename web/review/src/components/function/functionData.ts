// Pure helpers for the Function tab: the metric rows and what clicking a caller or
// callee does.

import type { FunctionRef, FunctionRisk, ReviewContext } from '../../types';
import { plural } from '../../lib/format';

export type MetricLevel = 'ok' | 'notable' | 'high';

export interface MetricRow {
  label: string;
  value: string;
  level: MetricLevel;
  title?: string;
}

/** Above the first threshold is notable, above the second high */
function level(value: number | undefined, notable: number, high: number): MetricLevel {
  if (value === undefined) return 'ok';
  if (value >= high) return 'high';
  return value >= notable ? 'notable' : 'ok';
}

/** Maintainability: lower is worse */
function maintainabilityLevel(value: number | undefined): MetricLevel {
  if (value === undefined) return 'ok';
  if (value < 20) return 'high';
  return value < 40 ? 'notable' : 'ok';
}

const show = (value: number | undefined) => (value === undefined ? '—' : String(Math.round(value * 10) / 10));
const numberOrUndefined = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? value : undefined);

export function metricRows(fn: FunctionRisk): MetricRow[] {
  const metrics = fn.metrics ?? null;
  const read = (key: string) => numberOrUndefined(metrics?.[key]);
  const churn = numberOrUndefined(metrics?.churn?.total_commits);
  const rows: MetricRow[] = [];
  if (metrics) {
    rows.push(
      { label: 'Cyclomatic', value: show(read('cyclomatic_complexity')), level: level(read('cyclomatic_complexity'), 10, 20) },
      { label: 'Cognitive', value: show(read('cognitive_complexity')), level: level(read('cognitive_complexity'), 15, 25) },
      { label: 'Nesting', value: show(read('nesting_depth')), level: level(read('nesting_depth'), 4, 6) },
      { label: 'Length', value: show(read('length')), level: level(read('length'), 50, 100) },
      { label: 'Params', value: show(read('parameter_count')), level: level(read('parameter_count'), 5, 7) },
      { label: 'Maintainability', value: show(read('maintainability_index')), level: maintainabilityLevel(read('maintainability_index')) },
    );
  }
  rows.push({
    label: 'Fan-in',
    value: `${fn.fan_in_approximate ? '≈' : ''}${fn.fan_in}`,
    level: level(fn.fan_in, 8, 15),
    title: fn.fan_in_approximate
      ? `Approximate: some calls were matched by name, not by the call graph. ${callerSummary(fn)}`
      : callerSummary(fn),
  });
  if (metrics) rows.push({ label: 'Churn', value: churn === undefined ? '—' : `${churn}`, level: level(churn, 10, 20), title: 'Commits touching this function' });
  return rows;
}

/**
 * Distinct calling functions. Fan-in counts call sites (one caller calling twice
 * adds 2), and the callers list is capped, so neither is this number.
 */
export function callerCount(fn: FunctionRisk): number {
  return fn.caller_count ?? (fn.callers ?? []).length;
}

/** "11 functions, 36 call sites", or just the call sites when callers are unknown (text-search fan-in) */
export function callerSummary(fn: FunctionRisk): string {
  const sites = `${fn.fan_in_approximate ? '≈' : ''}${plural(fn.fan_in, 'call site')}`;
  const callers = callerCount(fn);
  return callers > 0 ? `${plural(callers, 'function')}, ${sites}` : sites;
}

/** Section heading for the callers list: "Callers (11 functions, 36 call sites)" */
export function callersHeading(fn: FunctionRisk): string {
  return fn.fan_in > 0 || callerCount(fn) > 0 ? `Callers (${callerSummary(fn)})` : 'Callers';
}

/** "showing 25 of 40" when the server capped the callers list, else null */
export function callersCapNote(fn: FunctionRisk): string | null {
  const shown = (fn.callers ?? []).length;
  const total = callerCount(fn);
  return shown < total ? `showing ${shown} of ${total}` : null;
}

export type RefDirection = 'callers' | 'callees';

export type RefAction =
  | { type: 'selectFunction'; id: string }
  | { type: 'openLocation'; file: string; line: number; callSite: boolean }
  | { type: 'none' };

/** Changed functions select; other callers open at their call site, other callees at their definition */
export function refAction(ref: FunctionRef, direction: RefDirection, changedIds: ReadonlySet<string>): RefAction {
  if (changedIds.has(ref.id)) return { type: 'selectFunction', id: ref.id };
  if (ref.external || !ref.file) return { type: 'none' };
  return { type: 'openLocation', file: ref.file, line: ref.line, callSite: direction === 'callers' };
}

/** Changed functions that call or are called by a node */
export function connectedFunctions(context: ReviewContext, nodeId: string): FunctionRisk[] {
  return (context.functions ?? []).filter((fn) =>
    (fn.callers ?? []).some((ref) => ref.id === nodeId) || (fn.callees ?? []).some((ref) => ref.id === nodeId));
}
