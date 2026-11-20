import { describe, expect, it } from 'vitest';
import type { DiffLine, FileDiff, Finding, Hunk } from '../types';
import { isInlineFinding } from './findings';

function hunk(lines: DiffLine[]): Hunk {
  return { old_start: 1, old_count: 0, new_start: lines[0]?.new_line ?? 1, new_count: lines.length, header: '', lines };
}

const added = (line: number): DiffLine => ({ kind: 'added', old_line: 0, new_line: line, text: '' });
const context = (line: number): DiffLine => ({ kind: 'context', old_line: line, new_line: line, text: '' });
const removed = (oldLine: number): DiffLine => ({ kind: 'removed', old_line: oldLine, new_line: 0, text: '' });

/** Two hunks: head lines 10-14 and 40-42; old line 13 removed inside the first */
const FILE: FileDiff = {
  path: 'pkg/a.go', old_path: 'pkg/a.go', status: 'modified', language: 'Go', additions: 5, deletions: 1,
  binary: false, line_count: 100, risk_score: 0, risk_reasons: [],
  hunks: [
    hunk([context(10), added(11), added(12), removed(13), context(13), context(14)]),
    hunk([context(40), added(41), context(42)]),
  ],
};

function finding(line: number, endLine?: number, filePath = 'pkg/a.go'): Finding {
  return {
    id: 'f', severity: 'warning', category: 'correctness', file: filePath, line, title: 't', body: '', confidence: 0.8,
    ...(endLine === undefined ? {} : { end_line: endLine }),
  };
}

// Mirrors inlineComment in pkg/github/post.go: line commentable; when end_line > line, both
// ends commentable and in the same hunk
describe('isInlineFinding', () => {
  const cases: [string, Finding, boolean][] = [
    ['single added line', finding(11), true],
    ['single context line', finding(42), true],
    ['single line outside the diff', finding(20), false],
    ['range inside one hunk', finding(10, 14), true],
    ['range across two hunks', finding(12, 41), false],
    ['start outside the diff, end inside', finding(5, 11), false],
    ['start inside, end outside', finding(41, 50), false],
    ['end equal to line', finding(41, 41), true],
    ['end before line is ignored, as in Go', finding(41, 30), true],
    ['no line (file-level)', finding(0), false],
    ['no file', finding(11, undefined, ''), false],
  ];
  for (const [name, input, want] of cases) {
    it(name, () => expect(isInlineFinding(input, FILE)).toBe(want));
  }

  it('is false when the file is not part of the diff', () => {
    expect(isInlineFinding(finding(11), undefined)).toBe(false);
  });

  it('does not treat a removed line as commentable', () => {
    const onlyRemoved: FileDiff = { ...FILE, hunks: [hunk([removed(7)])] };
    expect(isInlineFinding(finding(7), onlyRemoved)).toBe(false);
  });
});
