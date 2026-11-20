import { describe, expect, it } from 'vitest';
import { deriveAiCoverage, rangeCoversFunction, readRange, type CoverageInput } from './aiCoverage';
import type { FileDiff, ReviewEvent } from '../types';
import { makeFile } from '../components/post/testFixtures';
import { makeFunction, toolCall } from './aiCoverage.fixtures';

const functions = [
  makeFunction('pkg.A', 'pkg/a.go', 10, 20, { focus: true }),
  makeFunction('pkg.B', 'pkg/a.go', 30, 60),
  makeFunction('pkg.C', 'pkg/c.go', 5, 15),
  makeFunction('pkg.D', 'pkg/d.go', 1, 9),
];
const files: FileDiff[] = [makeFile('pkg/a.go'), makeFile('pkg/c.go'), makeFile('pkg/d.go'), makeFile('.github/workflows/ci.yml')];

function derive(activity: ReviewEvent[], overrides: Partial<CoverageInput> = {}) {
  return deriveAiCoverage({ activity, functions, files, statusState: 'running', hasResult: false, ...overrides });
}

describe('readRange', () => {
  it('parses read_lines and Read ranges, and treats a rangeless Read as the whole file', () => {
    expect(readRange({ tool: 'read_lines', summary: 'read_lines(pkg/a.go:1-40)' })).toEqual({ start: 1, end: 40 });
    expect(readRange({ tool: 'Read', summary: 'Read(pkg/a.go:4-13)' })).toEqual({ start: 4, end: 13 });
    expect(readRange({ tool: 'Read', summary: 'Read(pkg/a.go:30-)' })).toEqual({ start: 30, end: Infinity });
    expect(readRange({ tool: 'Read', summary: 'Read(pkg/a.go)' })).toEqual({ start: 1, end: Infinity });
    expect(readRange({ tool: 'get_file_diff', summary: 'get_file_diff(pkg/a.go)' })).toBeNull();
    expect(readRange({ tool: 'read_lines', summary: 'read_lines(pkg/a.go:9-3)' })).toBeNull();
  });

  it('needs at least half of a function in range', () => {
    const fn = { start_line: 10, end_line: 20 };
    expect(rangeCoversFunction({ start: 1, end: 15 }, fn)).toBe(true);
    expect(rangeCoversFunction({ start: 1, end: 12 }, fn)).toBe(false);
    expect(rangeCoversFunction({ start: 21, end: 40 }, fn)).toBe(false);
  });
});

describe('deriveAiCoverage', () => {
  it('is unknown and incomplete with no targeted tool calls', () => {
    const coverage = derive([toolCall({ tool: 'get_function', summary: 'get_function(pkg.A)' })]);
    expect(coverage.known).toBe(false);
    expect(coverage.complete).toBe(false);
    expect(coverage.examinedFunctions.size).toBe(0);
  });

  it('collects function targets, read ranges and the files they live in', () => {
    const coverage = derive([
      toolCall({ tool: 'get_function', summary: 'get_function(pkg.A)', target: 'pkg.A', target_kind: 'function' }),
      toolCall({ tool: 'get_callers', summary: 'get_callers(other.X)', target: 'other.X', target_kind: 'function' }),
      toolCall({ tool: 'read_lines', summary: 'read_lines(pkg/a.go:28-50)', target: 'pkg/a.go', target_kind: 'file' }),
      toolCall({ tool: 'Read', summary: 'Read(pkg/c.go)', target: 'pkg/c.go', target_kind: 'file', agent: 'security-reviewer' }),
      toolCall({ tool: 'get_file_diff', summary: 'get_file_diff(pkg/d.go)', target: 'pkg/d.go', target_kind: 'file' }),
      toolCall({ tool: 'Read', summary: 'Read(docs/x.md)', target: 'docs/x.md', target_kind: 'file' }),
      toolCall({ tool: 'Grep', summary: 'Grep("x")' }),
    ]);
    expect(coverage.known).toBe(true);
    expect([...coverage.examinedFunctions].sort()).toEqual(['pkg.A', 'pkg.B', 'pkg.C']);
    expect([...coverage.examinedFiles].sort()).toEqual(['pkg/a.go', 'pkg/c.go', 'pkg/d.go']);
  });

  it('adds the file of an examined function even when the file itself was not opened', () => {
    const coverage = derive([toolCall({ tool: 'get_function', summary: '', target: 'pkg.D', target_kind: 'function' })]);
    expect([...coverage.examinedFiles]).toEqual(['pkg/d.go']);
  });

  it('is complete once done, errored or a result exists', () => {
    expect(derive([], { statusState: 'done' }).complete).toBe(true);
    expect(derive([], { statusState: 'error' }).complete).toBe(true);
    expect(derive([], { hasResult: true }).complete).toBe(true);
    expect(derive([], { statusState: 'thinking' }).complete).toBe(false);
  });
});
