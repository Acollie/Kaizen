import { describe, expect, test } from 'vitest';
import type { FileDiff, Finding, FunctionRisk } from '../../types';
import type { SidebarGroup } from '../../lib/grouping';
import {
  COLLAPSE_FUNCTION_LIMIT, fileReason, firstFilterMatch, groupSeverity, isExpanded, isExpandedByDefault,
  splitForMiddleTruncation, statusLetter, totalFunctions, type SidebarEntry,
} from './sidebarModel';

const file = (path: string, reasons: string[] = []) => ({ path, status: 'modified', risk_score: 0, risk_reasons: reasons }) as unknown as FileDiff;
const fn = (id: string, name = id, focus = false) => ({ id, name, focus, file: 'a.go' }) as unknown as FunctionRisk;
const finding = (severity: Finding['severity']) => ({ id: severity, severity, file: 'a.go' }) as unknown as Finding;
const entry = (overrides: Partial<SidebarEntry> = {}): SidebarEntry => ({ file: file('a.go'), functions: [fn('f')], findings: [], ...overrides });

const LARGE = COLLAPSE_FUNCTION_LIMIT + 1;

describe('expansion', () => {
  test('small PRs expand every file with functions', () => {
    expect(isExpandedByDefault(entry(), { total: 3, filter: '', activeFile: null })).toBe(true);
    expect(isExpandedByDefault(entry({ functions: [] }), { total: 3, filter: '', activeFile: null })).toBe(false);
  });
  test('large PRs expand only files that need a look', () => {
    const context = { total: LARGE, filter: '', activeFile: null };
    expect(isExpandedByDefault(entry(), context)).toBe(false);
    expect(isExpandedByDefault(entry({ functions: [fn('f', 'f', true)] }), context)).toBe(true);
    expect(isExpandedByDefault(entry({ findings: [finding('suggestion')] }), context)).toBe(true);
    expect(isExpandedByDefault(entry(), { ...context, activeFile: 'a.go' })).toBe(true);
    expect(isExpandedByDefault(entry(), { ...context, filter: 'x' })).toBe(true);
  });
  test('user overrides win', () => {
    const context = { total: 3, filter: '', activeFile: null };
    expect(isExpanded(entry(), { 'a.go': false }, context)).toBe(false);
    expect(isExpanded(entry(), {}, context)).toBe(true);
  });
});

describe('labels', () => {
  test('statusLetter', () => {
    expect(['added', 'modified', 'deleted', 'renamed', 'odd'].map(statusLetter)).toEqual(['A', 'M', 'D', 'R', '?']);
  });
  test('fileReason only for files without functions', () => {
    expect(fileReason(entry({ file: file('action.yml', ['CI config changed']), functions: [] }))).toBe('CI config changed');
    expect(fileReason(entry({ file: file('a.go', ['ignored']) }))).toBe('');
  });
});

test('splitForMiddleTruncation keeps the tail visible', () => {
  expect(splitForMiddleTruncation('short')).toEqual({ head: 'short', tail: '' });
  expect(splitForMiddleTruncation('Config.ValidateConfiguration')).toEqual({ head: 'Config.', tail: 'ValidateConfiguration' });
  expect(splitForMiddleTruncation('validateSeverityThresholds')).toEqual({ head: 'validateSeverity', tail: 'Thresholds' });
});

describe('groups', () => {
  const groups: SidebarGroup[] = [
    { key: 'attention', title: 'Needs attention', hint: '', files: [entry({ findings: [finding('suggestion'), finding('warning')] })] },
    { key: 'other', title: 'Other', hint: '', files: [entry({ file: file('pkg/b.go'), functions: [fn('b1', 'Render'), fn('b2', 'Parse')] })] },
  ];
  test('groupSeverity tints the attention group only', () => {
    expect(groupSeverity(groups[0]!)).toBe('warning');
    expect(groupSeverity(groups[1]!)).toBeNull();
  });
  test('totalFunctions counts all rows', () => {
    expect(totalFunctions(groups)).toBe(3);
  });
  test('firstFilterMatch prefers path matches, then function names', () => {
    expect(firstFilterMatch(groups, '')).toEqual({ kind: 'file', path: 'a.go' });
    expect(firstFilterMatch([groups[1]!], 'pars')).toEqual({ kind: 'function', id: 'b2' });
    expect(firstFilterMatch([groups[1]!], 'pkg/')).toEqual({ kind: 'file', path: 'pkg/b.go' });
    expect(firstFilterMatch([], 'x')).toBeNull();
  });
});
