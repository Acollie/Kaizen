// Sidebar grouping and ordering. Shared by the sidebar (what it shows) and the keymap
// (j/k and J/K walk the same order), so it lives here as pure functions.
//
// Files are grouped by what they need from the reviewer: "attention" when the AI found
// something to fix (critical or warning), "focus" when Kaizen's metrics flag them, and
// "other". Within a group, AI severity outranks Kaizen risk.

import type { FileDiff, Finding, FunctionRisk } from '../types';
import { isUrgent } from './findings';

export type GroupKey = 'attention' | 'focus' | 'other';

export const GROUPS: readonly { key: GroupKey; title: string; hint: string }[] = [
  { key: 'attention', title: 'Needs attention', hint: 'AI found issues to fix' },
  { key: 'focus', title: 'Kaizen focus', hint: 'riskiest by metrics' },
  { key: 'other', title: 'Other changes', hint: '' },
];

/** Files at or above this risk are in "focus" even without a focus function */
export const FOCUS_FILE_SCORE = 20;

export interface SidebarInput {
  files: readonly FileDiff[];
  functionsByFile: ReadonlyMap<string, readonly FunctionRisk[]>;
  findings: readonly Finding[];
  filter: string;
}

export interface SidebarGroup {
  key: GroupKey;
  title: string;
  hint: string;
  files: { file: FileDiff; functions: FunctionRisk[]; findings: Finding[] }[];
}

export function groupOf(file: FileDiff, functions: readonly FunctionRisk[], findings: readonly Finding[]): GroupKey {
  if (findings.some(isUrgent)) return 'attention';
  if (functions.some((fn) => fn.focus) || file.risk_score >= FOCUS_FILE_SCORE) return 'focus';
  return 'other';
}

function fileWeight(file: FileDiff, findings: readonly Finding[]): number {
  const critical = findings.filter((finding) => finding.severity === 'critical').length;
  const warning = findings.filter((finding) => finding.severity === 'warning').length;
  return critical * 1000 + warning * 100 + file.risk_score;
}

function matchesFilter(filter: string, file: FileDiff, fn?: FunctionRisk): boolean {
  if (!filter) return true;
  const needle = filter.toLowerCase();
  return file.path.toLowerCase().includes(needle) || Boolean(fn && fn.name.toLowerCase().includes(needle));
}

/** The sidebar's groups, in display order, with the filter applied */
export function buildSidebarGroups({ files, functionsByFile, findings, filter }: SidebarInput): SidebarGroup[] {
  const findingsByFile = new Map<string, Finding[]>();
  for (const finding of findings) {
    const list = findingsByFile.get(finding.file) ?? [];
    list.push(finding);
    findingsByFile.set(finding.file, list);
  }

  const entries = files
    .map((file) => {
      const allFunctions = [...(functionsByFile.get(file.path) ?? [])];
      const fileFindings = findingsByFile.get(file.path) ?? [];
      const fileMatches = matchesFilter(filter, file);
      const functions = allFunctions
        .filter((fn) => fileMatches || matchesFilter(filter, file, fn))
        .sort((left, right) => right.risk_score - left.risk_score || left.start_line - right.start_line);
      const visible = fileMatches || functions.length > 0;
      return { file, functions, findings: fileFindings, group: groupOf(file, allFunctions, fileFindings), visible };
    })
    .filter((entry) => entry.visible)
    .sort((left, right) => fileWeight(right.file, right.findings) - fileWeight(left.file, left.findings) || left.file.path.localeCompare(right.file.path));

  return GROUPS
    .map((group) => ({
      ...group,
      files: entries.filter((entry) => entry.group === group.key).map(({ file, functions, findings: fileFindings }) => ({ file, functions, findings: fileFindings })),
    }))
    .filter((group) => group.files.length > 0);
}

/** Files in sidebar order */
export function sidebarFiles(groups: readonly SidebarGroup[]): FileDiff[] {
  return groups.flatMap((group) => group.files.map((entry) => entry.file));
}

/** Changed functions in sidebar order */
export function sidebarFunctions(groups: readonly SidebarGroup[]): FunctionRisk[] {
  return groups.flatMap((group) => group.files.flatMap((entry) => entry.functions));
}
