import { describe, expect, it } from 'vitest';
import type { FileDiff } from '../../types';
import { needsFetch, snippetRange, snippetRows } from './snippet';

const file: FileDiff = {
  path: 'a.go', old_path: 'a.go', status: 'modified', language: 'Go', additions: 1, deletions: 1, binary: false,
  line_count: 30, risk_score: 0,
  hunks: [{
    old_start: 10, old_count: 3, new_start: 10, new_count: 3, header: '',
    lines: [
      { kind: 'context', old_line: 10, new_line: 10, text: 'a' },
      { kind: 'removed', old_line: 11, new_line: 0, text: 'old' },
      { kind: 'added', old_line: 0, new_line: 11, text: 'new' },
      { kind: 'context', old_line: 12, new_line: 12, text: 'c' },
    ],
  }],
};

describe('verify snippet', () => {
  it('takes ±8 lines around the finding, clamped to the file', () => {
    expect(snippetRange({ line: 11 }, file)).toEqual({ from: 3, to: 19, markFrom: 11, markTo: 11 });
    expect(snippetRange({ line: 2, end_line: 4 }, file)).toEqual({ from: 1, to: 12, markFrom: 2, markTo: 4 });
    expect(snippetRange({ line: 28 }, file)?.to).toBe(30);
    expect(snippetRange({ line: 0 }, file)?.from).toBe(10);
  });

  it('keeps diff colouring and removed lines in place, filling the rest from the file', () => {
    const range = { from: 9, to: 13, markFrom: 11, markTo: 11 };
    const pending = snippetRows(file, range, null);
    expect(pending.map((row) => `${row.kind}:${row.newLine || row.oldLine}${row.pending ? '?' : ''}`))
      .toEqual(['context:9?', 'context:10', 'removed:11', 'added:11', 'context:12', 'context:13?']);
    expect(pending.find((row) => row.kind === 'added')?.marked).toBe(true);
    const loaded = snippetRows(file, range, new Map([[9, 'nine'], [13, 'thirteen']]));
    expect(loaded.map((row) => row.text)).toEqual(['nine', 'a', 'old', 'new', 'c', 'thirteen']);
    expect(needsFetch(file, range)).toBe(true);
    expect(needsFetch(file, { from: 10, to: 12, markFrom: 0, markTo: -1 })).toBe(false);
  });
});
