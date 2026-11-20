import type { Finding, FunctionRisk } from '../../types';
import { findCalledFunction, findingAnchorLine, functionRails, placeFindings } from './placement';
import { buildPlainRows } from './rows';
import { calleeOnLine, cursorRowIndex } from './useCodeCommands';

const finding = (id: string, line: number | undefined, extra: Partial<Finding> = {}): Finding => ({
  id, severity: 'warning', category: 'correctness', file: 'a.go', line, title: id, body: '', confidence: 0.5, ...extra,
});

const fn = (id: string, start: number, end: number, extra: Partial<FunctionRisk> = {}): FunctionRisk => ({
  id, name: id, file: 'a.go', language: 'Go', start_line: start, end_line: end, lines_added: 0, lines_removed: 0,
  is_new: false, fan_in: 0, fan_in_approximate: false, callers: [], callees: [], risk_score: 10, risk_reasons: [], focus: false,
  ...extra,
});

const lines = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, index) => ({ n: from + index, text: `x${from + index}` }));

describe('placeFindings', () => {
  const rows = buildPlainRows(lines(10, 30), 100);

  it('puts findings under their end line, file-level ones apart, and the rest outside', () => {
    const placement = placeFindings(rows, [
      finding('range', 12, { end_line: 14 }),
      finding('file', undefined),
      finding('far', 80),
      finding('critical', 14, { severity: 'critical' }),
    ]);
    expect(placement.byLine.get(14)?.map((item) => item.id)).toEqual(['critical', 'range']);
    expect(placement.fileLevel.map((item) => item.id)).toEqual(['file']);
    expect(placement.outside.map((item) => item.id)).toEqual(['far']);
  });

  it('falls back to the first line when the end line is hidden', () => {
    const placement = placeFindings(rows, [finding('edge', 29, { end_line: 33 })]);
    expect(placement.byLine.get(29)?.[0]?.id).toBe('edge');
    expect(findingAnchorLine(finding('edge', 29, { end_line: 33 }))).toBe(33);
  });
});

describe('functionRails', () => {
  it('maps rows to the innermost function and labels each function once', () => {
    const rows = buildPlainRows(lines(1, 20), 20);
    const outer = fn('outer', 2, 15);
    const inner = fn('inner', 5, 8);
    const rails = functionRails(rows, [outer, inner]);
    expect(rails.byRow.get('n3')?.id).toBe('outer');
    expect(rails.byRow.get('n6')?.id).toBe('inner');
    expect(rails.byRow.get('n16')).toBeUndefined();
    expect([...rails.labels.entries()].map(([key, value]) => [key, value.id])).toEqual([['n2', 'outer'], ['n5', 'inner']]);
  });
});

describe('go to definition helpers', () => {
  it('finds the earliest called name, ignoring partial identifiers', () => {
    const refs = [{ name: 'Sprintf' }, { name: 'validate' }, { name: 'fmt' }];
    expect(findCalledFunction('errors = append(errors, validate(x)); fmt.Sprintf()', refs)?.name).toBe('validate');
    expect(findCalledFunction('revalidate(x)', [{ name: 'validate' }])).toBeNull();
    expect(findCalledFunction('no calls here', refs)).toBeNull();
  });

  it('prefers the enclosing function callees, then changed functions, skipping external ones', () => {
    const helper = fn('pkg.helper', 40, 50, { name: 'helper' });
    const caller = fn('pkg.caller', 1, 20, {
      name: 'caller',
      callees: [
        { id: 'ext.Println', name: 'Println', file: '', line: 0, external: true },
        { id: 'pkg.load', name: 'load', file: 'b.go', line: 7, external: false },
      ],
    });
    expect(calleeOnLine('fmt.Println(helper(1))', caller, [caller, helper])?.id).toBe('pkg.helper');
    expect(calleeOnLine('helper(load(1))', caller, [caller, helper])?.id).toBe('pkg.load');
    expect(calleeOnLine('x := helper(1)', caller, [caller, helper])).toMatchObject({ id: 'pkg.helper', line: 40 });
    expect(calleeOnLine('caller()', caller, [caller])).toBeNull();
  });

  it('maps a finding card cursor to the row it hangs under', () => {
    const rows = buildPlainRows(lines(1, 10), 10);
    const placement = placeFindings(rows, [finding('f1', 4)]);
    expect(cursorRowIndex(rows, 'f:f1', placement)).toBe(rows.findIndex((row) => row.key === 'n4'));
    expect(cursorRowIndex(rows, 'n7', placement)).toBe(6);
    expect(cursorRowIndex(rows, null, placement)).toBe(-1);
  });
});
