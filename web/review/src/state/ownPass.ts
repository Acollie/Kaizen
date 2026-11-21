// "Your pass": the checklist the reviewer works through in Think first, before seeing
// the AI's findings. Built only from Kaizen's own data (the Overview's "Where to look"
// packages, focus functions, risky non-code files), never from anything the AI did, so
// it cannot anchor the reviewer on the AI. Progress comes from the existing viewed state.

import type { FileDiff, FunctionRisk, ReviewGraph } from '../types';
import { buildPackageMap, whereToLook, type PackageInfo } from '../lib/packageMap';
import { qualifiedName, splitPath } from '../lib/format';
import { focusFunctions, riskyFiles, type ViewedState } from './viewing';

export const MAX_PASS_PACKAGES = 5;
export const MAX_PASS_FUNCTIONS = 8;

export type PassItem =
  | { kind: 'package'; key: string; label: string; reason: string; risk: number; functionIds: string[]; files: string[]; open: PassTarget }
  | { kind: 'function'; key: string; label: string; reason: string; risk: number; id: string }
  | { kind: 'file'; key: string; label: string; reason: string; risk: number; path: string };

export type PassTarget = { kind: 'function'; id: string } | { kind: 'file'; path: string };

export interface PassInput {
  files: readonly FileDiff[];
  functions: readonly FunctionRisk[];
  graph: ReviewGraph;
  functionsByFile: ReadonlyMap<string, readonly FunctionRisk[]>;
}

const NOTHING_VIEWED: Readonly<Record<string, true>> = {};

/** The checklist, in order: packages to look at, focus functions, risky non-code files */
export function buildPass(input: PassInput): PassItem[] {
  return [...packageItems(input), ...functionItems(input.functions), ...fileItems(input)];
}

function packageItems(input: PassInput): PassItem[] {
  // No findings, no AI coverage and nothing viewed: a stable, Kaizen-only ranking
  const map = buildPackageMap({
    files: input.files, functions: input.functions, graph: input.graph, findings: [],
    viewedFunctions: NOTHING_VIEWED, viewedFiles: NOTHING_VIEWED, aiFunctions: new Set(), aiFiles: new Set(), aiKnown: false,
  });
  return whereToLook(map, MAX_PASS_PACKAGES).map(({ pkg }) => ({
    kind: 'package' as const,
    key: `pkg:${pkg.key}`,
    label: pkg.label,
    reason: packageReason(pkg),
    risk: pkg.maxRisk,
    functionIds: pkg.functions.map((fn) => fn.id),
    files: pkg.files.map((file) => file.path),
    open: packageTarget(pkg),
  }));
}

function packageTarget(pkg: PackageInfo): PassTarget {
  const top = pkg.functions[0];
  if (top) return { kind: 'function', id: top.id };
  const file = [...pkg.files].sort((left, right) => right.risk_score - left.risk_score)[0];
  return { kind: 'file', path: file?.path ?? '' };
}

/** "3 focus · riskiest: Load (risk 42)"; Kaizen metrics only */
function packageReason(pkg: PackageInfo): string {
  const parts: string[] = [];
  if (pkg.focusCount) parts.push(`${pkg.focusCount} focus`);
  const top = pkg.functions[0];
  if (top) parts.push(`riskiest: ${qualifiedName(top)} (risk ${Math.round(top.risk_score)})`);
  else parts.push(`${pkg.files.length} ${pkg.files.length === 1 ? 'file' : 'files'}, risk ${Math.round(pkg.maxRisk)}`);
  return parts.join(' · ');
}

function functionItems(functions: readonly FunctionRisk[]): PassItem[] {
  return focusFunctions(functions).slice(0, MAX_PASS_FUNCTIONS).map((fn) => ({
    kind: 'function' as const,
    key: `fn:${fn.id}`,
    label: qualifiedName(fn),
    reason: `${splitPath(fn.file).base} · ${(fn.risk_reasons ?? []).find(Boolean) ?? `risk ${Math.round(fn.risk_score)}`}`,
    risk: fn.risk_score,
    id: fn.id,
  }));
}

function fileItems(input: PassInput): PassItem[] {
  return riskyFiles(input.files, input.functionsByFile).map((file) => ({
    kind: 'file' as const,
    key: `file:${file.path}`,
    label: file.path,
    reason: (file.risk_reasons ?? []).filter(Boolean).join(' · '),
    risk: file.risk_score,
    path: file.path,
  }));
}

/** A package counts as viewed once you've opened any of its functions or files */
export function isPassItemViewed(item: PassItem, viewed: ViewedState): boolean {
  switch (item.kind) {
    case 'package':
      return item.functionIds.some((id) => viewed.functions[id]) || item.files.some((path) => viewed.files[path]);
    case 'function':
      return Boolean(viewed.functions[item.id]);
    case 'file':
      return Boolean(viewed.files[item.path]);
  }
}

export interface PassProgress {
  total: number;
  viewed: number;
  /** Item keys viewed */
  viewedKeys: ReadonlySet<string>;
  done: boolean;
}

export function passProgress(items: readonly PassItem[], viewed: ViewedState): PassProgress {
  const viewedKeys = new Set(items.filter((item) => isPassItemViewed(item, viewed)).map((item) => item.key));
  return { total: items.length, viewed: viewedKeys.size, viewedKeys, done: items.length > 0 && viewedKeys.size === items.length };
}

/** "4 of 7 viewed" */
export function passSentence(progress: Pick<PassProgress, 'total' | 'viewed'>): string {
  return `${progress.viewed} of ${progress.total} viewed`;
}

/** Where clicking an item goes */
export function passTarget(item: PassItem): PassTarget {
  if (item.kind === 'package') return item.open;
  if (item.kind === 'function') return { kind: 'function', id: item.id };
  return { kind: 'file', path: item.path };
}

/** The first item not viewed yet, or null */
export function nextPassItem(items: readonly PassItem[], progress: PassProgress): PassItem | null {
  return items.find((item) => !progress.viewedKeys.has(item.key)) ?? null;
}

/** The reveal button's wording: never a gate, always says how many are waiting */
export function revealLabel(ready: number): string {
  return `Reveal AI findings (${ready} ready)`;
}

export function passPrompt(progress: PassProgress, running: boolean): string {
  if (progress.done) return "You've done your pass. Compare with the AI.";
  if (progress.total === 0) return 'Read the diff yourself first. Write down what you notice, then compare with the AI.';
  const lead = running ? 'The AI is still reviewing. ' : '';
  return `${lead}Look at these yourself first and comment on anything you notice.`;
}
