import type { FileDiff, Hunk } from '../../types';
import {
  buildDiffRows, buildPlainRows, expandGap, findGapContaining, gapForExpansion, gapRange, hiddenLineCount,
  lineRowKey, stepHunk, type GapRow, type Row,
} from './rows';

function hunk(newStart: number, newCount: number, oldStart: number, oldCount: number, added = 1): Hunk {
  const lines: Hunk['lines'] = [];
  for (let offset = 0; offset < newCount; offset++) {
    const isAdded = offset < added;
    lines.push({
      kind: isAdded ? 'added' : 'context',
      old_line: isAdded ? 0 : oldStart + offset - added,
      new_line: newStart + offset,
      text: `line ${newStart + offset}`,
    });
  }
  return { old_start: oldStart, old_count: oldCount, new_start: newStart, new_count: newCount, header: 'func x()', lines };
}

function diff(hunks: Hunk[], lineCount: number, status: FileDiff['status'] = 'modified'): FileDiff {
  return {
    path: 'a.go', old_path: 'a.go', status, language: 'Go', additions: 0, deletions: 0, binary: false,
    hunks, line_count: lineCount, risk_score: 0,
  };
}

const gaps = (rows: readonly Row[]) => rows.filter((row): row is GapRow => row.type === 'gap');
const source = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, index) => ({ n: from + index, text: `src ${from + index}` }));

describe('buildDiffRows', () => {
  const file = diff([hunk(10, 3, 9, 2), hunk(40, 3, 38, 2)], 100);
  const rows = buildDiffRows(file);

  it('adds top, middle and bottom gaps with old-line offsets', () => {
    expect(gaps(rows).map((gap) => [gap.from, gap.to, gap.offset, gap.edge])).toEqual([
      [1, 9, -1, 'top'], [13, 39, -2, 'middle'], [43, 100, -3, 'bottom'],
    ]);
    expect(hiddenLineCount(rows)).toBe(9 + 27 + 58);
  });

  it('formats hunk ranges', () => {
    const header = rows.find((row) => row.type === 'hunk');
    expect(header).toMatchObject({ range: '@@ -9,2 +10,3 @@', header: 'func x()', begin: 10, end: 12 });
  });

  it('does not offer expansion for deleted files', () => {
    expect(gaps(buildDiffRows(diff([hunk(1, 2, 1, 1)], 50, 'deleted')))).toEqual([]);
  });
});

describe('expandGap', () => {
  const rows = buildDiffRows(diff([hunk(10, 3, 9, 2), hunk(40, 3, 38, 2)], 100));
  const middle = gaps(rows)[1]!;

  it('expands above a hunk, leaving the remainder as a gap', () => {
    const [from, to] = gapRange(middle, 'above');
    expect([from, to]).toEqual([20, 39]);
    const next = expandGap(rows, middle.key, from, to, source(from, to), { plain: false });
    expect(gaps(next).map((gap) => [gap.from, gap.to])).toEqual([[1, 9], [13, 19], [43, 100]]);
    const row = next.find((candidate) => candidate.key === 'n20');
    expect(row).toMatchObject({ kind: 'context', newLine: 20, oldLine: 18, text: 'src 20' });
  });

  it('keeps the identity of untouched rows', () => {
    const next = expandGap(rows, middle.key, 13, 32, source(13, 32), { plain: false });
    expect(next[0]).toBe(rows[0]);
    expect(next[next.length - 1]).toBe(rows[rows.length - 1]);
  });

  it('can split a gap in two (reveal a line in the middle)', () => {
    const next = expandGap(rows, middle.key, 20, 25, source(20, 25), { plain: true });
    expect(gaps(next).map((gap) => [gap.from, gap.to, gap.edge])).toEqual([[1, 9, 'top'], [13, 19, 'middle'], [26, 39, 'middle'], [43, 100, 'bottom']]);
    expect(next.find((row) => row.key === 'n22')).toMatchObject({ oldLine: 0 });
  });

  it('finds the gap holding a line and the row for a line', () => {
    expect(findGapContaining(rows, 50)?.key).toBe('g43-100');
    expect(findGapContaining(rows, 11)).toBeNull();
    expect(lineRowKey(rows, 11)).toBe('n11');
  });
});

describe('cursor helpers', () => {
  const rows = buildDiffRows(diff([hunk(10, 3, 9, 2), hunk(40, 3, 38, 2), hunk(60, 2, 58, 1)], 100));
  const indexOf = (key: string) => rows.findIndex((row) => row.key === key);

  it('steps between hunks from the cursor', () => {
    expect(rows[stepHunk(rows, indexOf('n11'), 1)]?.key).toBe('h1');
    expect(rows[stepHunk(rows, indexOf('n11'), 5)]?.key).toBe('h2');
    expect(rows[stepHunk(rows, indexOf('n41'), -1)]?.key).toBe('h0');
    expect(rows[stepHunk(rows, 0, 1)]?.key).toBe('h0');
    expect(stepHunk(rows, 0, -1)).toBe(-1);
  });

  it('picks the gap above or below the hunk at the cursor', () => {
    expect(gapForExpansion(rows, indexOf('n41'), 'above')?.key).toBe('g13-39');
    expect(gapForExpansion(rows, indexOf('n41'), 'below')?.key).toBe('g43-59');
    expect(gapForExpansion(rows, 0, 'above')?.key).toBe('g1-9');
  });

  it('uses the nearest gap in a read-only file', () => {
    const plain = buildPlainRows(source(40, 60), 200);
    expect(plain[0]).toMatchObject({ type: 'gap', from: 1, to: 39, edge: 'top' });
    expect(plain[plain.length - 1]).toMatchObject({ type: 'gap', from: 61, to: 200, edge: 'bottom' });
    expect(gapForExpansion(plain, 5, 'above')?.key).toBe('g1-39');
    expect(gapForExpansion(plain, 5, 'below')?.key).toBe('g61-200');
  });
});
