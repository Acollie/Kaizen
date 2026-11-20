// Package map for the Overview: groups the PR's files into packages (directories,
// collapsed to a readable count), aggregates Kaizen risk, AI findings and call edges per
// package, and measures how much of each package the reviewer and the AI have looked at.
// Pure and DOM-free.
//
// Coverage is reported honestly: "you" comes from what the reviewer actually viewed, and
// the AI's share is null ("unknown") when the engine reported no tool targets (older
// cached reviews), never 0%.

import type { FileDiff, Finding, FunctionRisk, ReviewGraph, Severity } from '../types';
import { SEVERITIES, SEVERITY_LABEL } from './findings';
import { plural, qualifiedName } from './format';

/** The node budget: collapse directories until the map has at most this many packages */
export const MAX_PACKAGES = 20;

/** Packages whose changed lines are at least this share test code are drawn dimmed */
export const TEST_DIM_SHARE = 0.6;

const FIXTURE_SEGMENTS = new Set(['testdata', '__tests__', '__fixtures__', 'fixtures', '__snapshots__']);

/** `_test.go`, `testdata/`, `.test.ts`, `.spec.tsx`, `test_x.py`, `__tests__/`... */
export function isTestFile(path: string): boolean {
  const segments = path.split('/');
  const base = segments[segments.length - 1] ?? '';
  if (segments.slice(0, -1).some((segment) => FIXTURE_SEGMENTS.has(segment))) return true;
  return /_test\.go$/.test(base)
    || /\.(test|spec)\.[cm]?[jt]sx?$/.test(base)
    || /^test_.*\.py$/.test(base) || /_test\.py$/.test(base)
    || /Tests?\.(swift|kt)$/.test(base);
}

/** "pkg/review/context.go" -> "pkg/review"; root files -> "" */
export function dirOf(path: string): string {
  const index = path.lastIndexOf('/');
  return index === -1 ? '' : path.slice(0, index);
}

/** A fixture directory (testdata, __tests__...) belongs to the package above it */
function homeDir(path: string): string {
  const segments = dirOf(path).split('/').filter(Boolean);
  const fixture = segments.findIndex((segment) => FIXTURE_SEGMENTS.has(segment));
  return (fixture === -1 ? segments : segments.slice(0, fixture)).join('/');
}

function depthOf(dir: string): number {
  return dir === '' ? 0 : dir.split('/').length;
}

function isUnder(dir: string, ancestor: string): boolean {
  return ancestor === '' || dir === ancestor || dir.startsWith(`${ancestor}/`);
}

function ancestorsOf(dir: string): string[] {
  const segments = dir.split('/').filter(Boolean);
  const result = [''];
  for (let length = 1; length < segments.length; length++) result.push(segments.slice(0, length).join('/'));
  return result;
}

interface Unit {
  key: string;
  /** True once other directories were merged in: the unit covers its whole subtree */
  subtree: boolean;
  paths: string[];
}

/**
 * Group file paths into packages: one per directory, then merge directories into an
 * ancestor until there are at most `max`. Merges that keep Go packages apart come
 * first (TS components, scripts), then the deepest ancestor, then the smallest.
 */
export function groupPackages(paths: readonly string[], weightOf: (path: string) => number = () => 1, max = MAX_PACKAGES): Unit[] {
  let units = new Map<string, Unit>();
  for (const path of paths) {
    const key = homeDir(path);
    const unit = units.get(key) ?? { key, subtree: false, paths: [] };
    unit.paths.push(path);
    units.set(key, unit);
  }
  while (units.size > max) {
    const target = bestMerge([...units.values()], weightOf);
    if (target === null) break;
    units = mergeInto(units, target);
  }
  return [...units.values()].sort((left, right) => left.key.localeCompare(right.key));
}

function bestMerge(units: readonly Unit[], weightOf: (path: string) => number): string | null {
  const candidates = new Set(units.flatMap((unit) => [...ancestorsOf(unit.key), unit.key]));
  let best: { key: string; score: [number, number, number] } | null = null;
  for (const key of candidates) {
    const inside = units.filter((unit) => isUnder(unit.key, key));
    if (inside.length < 2) continue;
    const goUnits = inside.filter((unit) => unit.paths.some((path) => path.endsWith('.go'))).length;
    const weight = inside.reduce((sum, unit) => sum + unit.paths.reduce((total, path) => total + weightOf(path), 0), 0);
    // Lower is better: [breaks Go package boundaries, -depth, weight]
    const score: [number, number, number] = [goUnits >= 2 ? 1 : 0, -depthOf(key), weight];
    if (!best || compareScores(score, best.score) < 0) best = { key, score };
  }
  return best?.key ?? null;
}

function compareScores(left: readonly number[], right: readonly number[]): number {
  for (let index = 0; index < left.length; index++) {
    const difference = (left[index] ?? 0) - (right[index] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}

function mergeInto(units: Map<string, Unit>, target: string): Map<string, Unit> {
  const next = new Map<string, Unit>();
  const merged: Unit = { key: target, subtree: true, paths: [] };
  for (const unit of units.values()) {
    if (isUnder(unit.key, target)) merged.paths.push(...unit.paths);
    else next.set(unit.key, unit);
  }
  next.set(target, merged);
  return next;
}

// ---------- Aggregation ----------

export interface SeenShare {
  seen: number;
  total: number;
}

export interface PackageCoverage {
  functions: SeenShare;
  files: SeenShare;
  /** 0-1 over changed functions (or files, for packages without functions); null = unknown */
  share: number | null;
}

export interface PackageInfo {
  /** Directory path, "" for the repository root */
  key: string;
  label: string;
  files: FileDiff[];
  /** Changed functions, riskiest first */
  functions: FunctionRisk[];
  added: number;
  removed: number;
  maxRisk: number;
  totalRisk: number;
  focusCount: number;
  findings: Finding[];
  severity: Record<Severity, number>;
  testFiles: number;
  /** Share of changed lines (or files, when there are none) that are test code */
  testShare: number;
  /** Mostly tests or fixtures (and no focus code or urgent findings): drawn dimmed */
  dim: boolean;
  you: PackageCoverage;
  ai: PackageCoverage;
}

export interface PackageEdge {
  from: string;
  to: string;
  /** Calls between the two packages within the PR's call subgraph */
  weight: number;
}

export interface PackageMap {
  packages: PackageInfo[];
  edges: PackageEdge[];
}

export interface CoverageSource {
  /** Changed functions / files the reviewer viewed */
  viewedFunctions: Readonly<Record<string, true>>;
  viewedFiles: Readonly<Record<string, true>>;
  /** What the AI examined; known=false means unknown, not zero */
  aiFunctions: ReadonlySet<string>;
  aiFiles: ReadonlySet<string>;
  aiKnown: boolean;
}

export interface PackageMapInput extends CoverageSource {
  files: readonly FileDiff[];
  functions: readonly FunctionRisk[];
  graph: ReviewGraph;
  /** Visible AI findings (none while "Think first" hides them) */
  findings: readonly Finding[];
  max?: number;
}

export function packageLabel(key: string): string {
  return key === '' ? '(root)' : key;
}

function changedLines(file: FileDiff): number {
  return Math.max(1, file.additions + file.deletions);
}

export function buildPackageMap(input: PackageMapInput): PackageMap {
  const fileByPath = new Map(input.files.map((file) => [file.path, file]));
  const units = groupPackages(input.files.map((file) => file.path), (path) => changedLines(fileByPath.get(path)!), input.max);
  const packageOfFile = new Map<string, string>();
  for (const unit of units) for (const path of unit.paths) packageOfFile.set(path, unit.key);

  const functionsByPackage = new Map<string, FunctionRisk[]>();
  for (const fn of input.functions) {
    const key = packageOfFile.get(fn.file);
    if (key !== undefined) functionsByPackage.set(key, [...(functionsByPackage.get(key) ?? []), fn]);
  }
  const findingsByPackage = new Map<string, Finding[]>();
  for (const finding of input.findings) {
    const key = packageOfFile.get(finding.file);
    if (key !== undefined) findingsByPackage.set(key, [...(findingsByPackage.get(key) ?? []), finding]);
  }

  const packages = units.map((unit) => summarise(
    unit,
    unit.paths.map((path) => fileByPath.get(path)!),
    functionsByPackage.get(unit.key) ?? [],
    findingsByPackage.get(unit.key) ?? [],
    input,
  ));
  return { packages, edges: packageEdges(input.graph, units, packageOfFile) };
}

function summarise(unit: Unit, files: FileDiff[], functions: FunctionRisk[], findings: Finding[], coverage: CoverageSource): PackageInfo {
  const severity = Object.fromEntries(SEVERITIES.map((level) => [level, 0])) as Record<Severity, number>;
  for (const finding of findings) if (finding.severity in severity) severity[finding.severity] += 1;
  const tests = files.filter((file) => isTestFile(file.path));
  const lines = files.reduce((sum, file) => sum + file.additions + file.deletions, 0);
  const testLines = tests.reduce((sum, file) => sum + file.additions + file.deletions, 0);
  const testShare = lines > 0 ? testLines / lines : (files.length ? tests.length / files.length : 0);
  const risks = [...functions.map((fn) => fn.risk_score), ...files.map((file) => file.risk_score)];
  return {
    key: unit.key,
    label: packageLabel(unit.key),
    files,
    functions: [...functions].sort((left, right) => right.risk_score - left.risk_score),
    added: files.reduce((sum, file) => sum + file.additions, 0),
    removed: files.reduce((sum, file) => sum + file.deletions, 0),
    maxRisk: risks.length ? Math.max(...risks) : 0,
    totalRisk: functions.reduce((sum, fn) => sum + fn.risk_score, 0),
    focusCount: functions.filter((fn) => fn.focus).length,
    findings,
    severity,
    testFiles: tests.length,
    testShare,
    // Never dim a package that carries focus code or urgent findings, whatever its tests
    dim: testShare >= TEST_DIM_SHARE && functions.every((fn) => !fn.focus) && severity.critical + severity.warning === 0,
    you: shareOf(functions, files, (fn) => Boolean(coverage.viewedFunctions[fn.id]), (file) => Boolean(coverage.viewedFiles[file.path]), true),
    ai: shareOf(functions, files, (fn) => coverage.aiFunctions.has(fn.id), (file) => coverage.aiFiles.has(file.path), coverage.aiKnown),
  };
}

function shareOf(
  functions: readonly FunctionRisk[], files: readonly FileDiff[],
  seenFunction: (fn: FunctionRisk) => boolean, seenFile: (file: FileDiff) => boolean, known: boolean,
): PackageCoverage {
  const functionShare = { seen: functions.filter(seenFunction).length, total: functions.length };
  const fileShare = { seen: files.filter(seenFile).length, total: files.length };
  const basis = functionShare.total > 0 ? functionShare : fileShare;
  return { functions: functionShare, files: fileShare, share: known && basis.total > 0 ? basis.seen / basis.total : (known ? 0 : null) };
}

/** Resolve any graph node's file (a PR file or an unchanged caller's) to a package */
function packageForFile(file: string, units: readonly Unit[], packageOfFile: ReadonlyMap<string, string>): string | undefined {
  const direct = packageOfFile.get(file);
  if (direct !== undefined) return direct;
  const dir = homeDir(file);
  const exact = units.find((unit) => unit.key === dir);
  if (exact) return exact.key;
  const covering = units.filter((unit) => unit.subtree && isUnder(dir, unit.key));
  return covering.sort((left, right) => depthOf(right.key) - depthOf(left.key))[0]?.key;
}

function packageEdges(graph: ReviewGraph, units: readonly Unit[], packageOfFile: ReadonlyMap<string, string>): PackageEdge[] {
  const packageOfNode = new Map<string, string>();
  for (const node of graph.nodes ?? []) {
    if (node.kind === 'external' || !node.file) continue;
    const key = packageForFile(node.file, units, packageOfFile);
    if (key !== undefined) packageOfNode.set(node.id, key);
  }
  const weights = new Map<string, PackageEdge>();
  for (const edge of graph.edges ?? []) {
    const from = packageOfNode.get(edge.from);
    const to = packageOfNode.get(edge.to);
    if (from === undefined || to === undefined || from === to) continue;
    const key = `${from}\u0000${to}`;
    const existing = weights.get(key);
    const weight = Math.max(1, edge.weight || 1);
    weights.set(key, existing ? { ...existing, weight: existing.weight + weight } : { from, to, weight });
  }
  return [...weights.values()].sort((left, right) => right.weight - left.weight || left.from.localeCompare(right.from));
}

// ---------- Summary ----------

export interface OverviewSummary {
  packages: number;
  files: number;
  functions: number;
  focus: SeenShare;
  /** Viewed share of focus code (or of all changed functions when nothing is focus) */
  youShare: number;
  youBasis: 'focus' | 'changed';
  /** Null when the AI's coverage is unknown */
  aiShare: number | null;
  aiComplete: boolean;
}

export function overviewSummary(map: PackageMap, input: CoverageSource & { functions: readonly FunctionRisk[]; aiComplete: boolean }): OverviewSummary {
  const focus = input.functions.filter((fn) => fn.focus);
  const basis = focus.length ? focus : input.functions;
  const seen = basis.filter((fn) => input.viewedFunctions[fn.id]).length;
  const examined = input.functions.filter((fn) => input.aiFunctions.has(fn.id)).length;
  return {
    packages: map.packages.length,
    files: map.packages.reduce((sum, pkg) => sum + pkg.files.length, 0),
    functions: input.functions.length,
    focus: { seen: focus.filter((fn) => input.viewedFunctions[fn.id]).length, total: focus.length },
    youShare: basis.length ? seen / basis.length : 0,
    youBasis: focus.length ? 'focus' : 'changed',
    aiShare: input.aiKnown && input.functions.length ? examined / input.functions.length : (input.aiKnown ? 0 : null),
    aiComplete: input.aiComplete,
  };
}

export function percent(share: number): string {
  if (share > 0 && share < 0.01) return '<1%';
  return `${Math.round(share * 100)}%`;
}

/** "15 packages · 256 files · 712 functions · You've viewed 42% of focus code · AI examined 6% of changed functions" */
export function summaryParts(summary: OverviewSummary): string[] {
  const you = summary.functions === 0
    ? 'No changed functions to view'
    : `You've viewed ${percent(summary.youShare)} of ${summary.youBasis === 'focus' ? 'focus code' : 'changed functions'}`;
  const ai = summary.aiShare === null
    ? 'AI coverage unknown'
    : `AI examined ${percent(summary.aiShare)} of changed functions${summary.aiComplete ? '' : ' so far'}`;
  return [plural(summary.packages, 'package'), plural(summary.files, 'file'), plural(summary.functions, 'function'), you, ai];
}

// ---------- Where to look ----------

export interface LookTarget {
  pkg: PackageInfo;
  score: number;
  /** Clauses of the one-line reason, e.g. ["2 should-fix findings", "you haven't opened it"] */
  reasons: string[];
}

/** How much a package deserves a human's attention, before coverage */
export function packageImportance(pkg: PackageInfo): number {
  const urgent = pkg.severity.critical * 30 + pkg.severity.warning * 14;
  return pkg.maxRisk + urgent + Math.min(pkg.focusCount, 4) * 8 + Math.min(pkg.severity.suggestion, 5) * 2;
}

function youGap(pkg: PackageInfo): number {
  return 1 - (pkg.you.share ?? 0);
}

function aiGap(pkg: PackageInfo): number {
  // Unknown AI coverage counts as half a gap: it may or may not have looked
  return pkg.ai.share === null ? 0.5 : 1 - pkg.ai.share;
}

function worthLooking(pkg: PackageInfo): boolean {
  const urgent = pkg.severity.critical + pkg.severity.warning;
  if (pkg.dim && urgent === 0) return false;
  return urgent > 0 || pkg.focusCount > 0 || pkg.maxRisk >= 12;
}

/**
 * Packages that are risky or carry urgent findings but that the reviewer and/or the AI
 * have barely looked at. Ranked by importance weighted by the coverage gap.
 */
export function whereToLook(map: PackageMap, limit = 5): LookTarget[] {
  return map.packages
    .filter(worthLooking)
    .map((pkg) => ({ pkg, score: packageImportance(pkg) * (0.65 * youGap(pkg) + 0.35 * aiGap(pkg)), reasons: lookReasons(pkg) }))
    .filter((target) => target.score >= 4)
    .sort((left, right) => right.score - left.score || left.pkg.key.localeCompare(right.pkg.key))
    .slice(0, limit);
}

function findingClause(pkg: PackageInfo): string | null {
  const parts = (['critical', 'warning'] as const)
    .filter((level) => pkg.severity[level] > 0)
    .map((level) => `${pkg.severity[level]} ${SEVERITY_LABEL[level].toLowerCase().replace(' ', '-')}`);
  if (!parts.length) return null;
  const total = pkg.severity.critical + pkg.severity.warning;
  return `${parts.join(', ')} ${total === 1 ? 'finding' : 'findings'}`;
}

function riskClause(pkg: PackageInfo): string | null {
  const top = pkg.functions[0];
  if (top && top.risk_score >= 12) {
    const reason = top.risk_reasons?.[0];
    const short = reason ? `: ${reason.split(':')[0]}` : '';
    return `${pkg.focusCount ? `★ ${pkg.focusCount} focus · ` : ''}${qualifiedName(top)} risk ${Math.round(top.risk_score)}${short}`;
  }
  const file = [...pkg.files].sort((left, right) => right.risk_score - left.risk_score)[0];
  if (file && file.risk_score >= 12) return `${file.path.split('/').pop()} risk ${Math.round(file.risk_score)}${file.risk_reasons?.[0] ? `: ${file.risk_reasons[0].split(':')[0]}` : ''}`;
  return null;
}

function youClause(pkg: PackageInfo): string {
  const { functions, files } = pkg.you;
  if (functions.seen === 0 && files.seen === 0) return "you haven't opened it";
  if (functions.total > 0) return `you've viewed ${functions.seen} of ${plural(functions.total, 'function')}`;
  return `you've opened ${files.seen} of ${plural(files.total, 'file')}`;
}

function aiClause(pkg: PackageInfo): string {
  if (pkg.ai.share === null) return 'AI coverage unknown';
  const basis = pkg.ai.functions.total > 0 ? { ...pkg.ai.functions, noun: 'function' } : { ...pkg.ai.files, noun: 'file' };
  return `AI examined ${basis.seen} of ${plural(basis.total, basis.noun)}`;
}

export function lookReasons(pkg: PackageInfo): string[] {
  return [riskClause(pkg), findingClause(pkg), youClause(pkg), aiClause(pkg)].filter((part): part is string => Boolean(part));
}
