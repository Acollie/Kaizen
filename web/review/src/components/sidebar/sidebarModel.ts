// Pure helpers for the changes sidebar: expansion defaults, filter matches, status letters.

import type { FileDiff, FileStatus, Finding, FunctionRisk, Severity } from '../../types';
import type { SidebarGroup } from '../../lib/grouping';
import { topSeverity } from '../../lib/findings';

/** Above this many changed functions, only files that need a look start expanded */
export const COLLAPSE_FUNCTION_LIMIT = 24;

export interface SidebarEntry {
  file: FileDiff;
  functions: FunctionRisk[];
  findings: Finding[];
}

export function totalFunctions(groups: readonly SidebarGroup[]): number {
  return groups.reduce((sum, group) => sum + group.files.reduce((inner, entry) => inner + entry.functions.length, 0), 0);
}

/**
 * Whether a file's function rows show when the user has not toggled it. Small PRs show
 * everything; large PRs show files with focus functions, findings or the active selection.
 * A non-empty filter expands every match so the matching functions are visible.
 */
export function isExpandedByDefault(entry: SidebarEntry, context: { total: number; filter: string; activeFile: string | null }): boolean {
  if (entry.functions.length === 0) return false;
  if (context.filter) return true;
  if (context.total <= COLLAPSE_FUNCTION_LIMIT) return true;
  if (entry.file.path === context.activeFile) return true;
  return entry.findings.length > 0 || entry.functions.some((fn) => fn.focus);
}

export function isExpanded(entry: SidebarEntry, overrides: Readonly<Record<string, boolean>>, context: { total: number; filter: string; activeFile: string | null }): boolean {
  const override = overrides[entry.file.path];
  return override ?? isExpandedByDefault(entry, context);
}

const STATUS_LETTERS: Record<FileStatus, string> = { added: 'A', modified: 'M', deleted: 'D', renamed: 'R' };

export function statusLetter(status: FileStatus | string): string {
  return STATUS_LETTERS[status as FileStatus] ?? '?';
}

/** Second line for files Kaizen cannot parse into functions (workflows, action.yml, scripts) */
export function fileReason(entry: SidebarEntry): string {
  if (entry.functions.length > 0) return '';
  return entry.file.risk_reasons?.[0] ?? '';
}

/** Characters kept visible at the end of a middle-truncated name */
export const NAME_TAIL = 10;

/**
 * Split a name so CSS can ellipsize the head first and keep the tail: a receiver-qualified
 * name keeps its method ("Conf…ValidateConfiguration"), others keep their last characters
 * ("validateSe…Thresholds").
 */
export function splitForMiddleTruncation(text: string, tailLength = NAME_TAIL): { head: string; tail: string } {
  const dot = text.lastIndexOf('.');
  if (dot > 0 && dot < text.length - 1) return { head: text.slice(0, dot + 1), tail: text.slice(dot + 1) };
  if (text.length <= tailLength * 2) return { head: text, tail: '' };
  return { head: text.slice(0, text.length - tailLength), tail: text.slice(text.length - tailLength) };
}

/** Tint for a group label: the most severe finding in the group (attention group only) */
export function groupSeverity(group: SidebarGroup): Severity | null {
  if (group.key !== 'attention') return null;
  return topSeverity(group.files.flatMap((entry) => entry.findings));
}

export type FilterTarget = { kind: 'file'; path: string } | { kind: 'function'; id: string };

/** What Enter in the filter opens: the first file whose path matches, else the first matching function */
export function firstFilterMatch(groups: readonly SidebarGroup[], filter: string): FilterTarget | null {
  const needle = filter.trim().toLowerCase();
  for (const group of groups) {
    for (const entry of group.files) {
      if (!needle || entry.file.path.toLowerCase().includes(needle)) return { kind: 'file', path: entry.file.path };
      const fn = entry.functions.find((candidate) => candidate.name.toLowerCase().includes(needle));
      if (fn) return { kind: 'function', id: fn.id };
      if (entry.functions[0]) return { kind: 'function', id: entry.functions[0].id };
    }
  }
  return null;
}
