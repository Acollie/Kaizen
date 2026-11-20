import type { FunctionRef, FunctionRisk, ReviewContext } from '../../types';
import { connectedFunctions, metricRows, refAction } from './functionData';

const fn = (overrides: Partial<FunctionRisk> = {}): FunctionRisk => ({
  id: 'pkg.f', name: 'f', file: 'a.go', language: 'Go', start_line: 1, end_line: 10, lines_added: 3, lines_removed: 1, is_new: false,
  fan_in: 2, fan_in_approximate: false, callers: [], callees: [], risk_score: 10, risk_reasons: [], focus: false, ...overrides,
});
const metrics = {
  name: 'f', start_line: 1, end_line: 10, length: 120, logical_lines: 5, parameter_count: 2, cyclomatic_complexity: 14,
  cognitive_complexity: 30, nesting_depth: 2, halstead_volume: 100, maintainability_index: 64.53, fan_in: 0, fan_out: 3,
  churn: { total_commits: 12 },
};
const ref = (overrides: Partial<FunctionRef>): FunctionRef => ({ id: 'x', name: 'x', file: 'b.go', line: 7, external: false, ...overrides });

describe('metricRows', () => {
  it('shows each metric with a level from its thresholds', () => {
    const rows = Object.fromEntries(metricRows(fn({ metrics })).map((row) => [row.label, row]));
    expect(rows.Cyclomatic).toMatchObject({ value: '14', level: 'notable' });
    expect(rows.Cognitive).toMatchObject({ value: '30', level: 'high' });
    expect(rows.Length).toMatchObject({ level: 'high' });
    expect(rows.Maintainability).toMatchObject({ value: '64.5', level: 'ok' });
    expect(rows.Churn).toMatchObject({ value: '12', level: 'notable' });
    expect(rows['Fan-in']).toMatchObject({ value: '2', level: 'ok' });
  });

  it('marks approximate fan-in and survives missing metrics', () => {
    const rows = metricRows(fn({ metrics: null, fan_in: 4, fan_in_approximate: true }));
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({ label: 'Fan-in', value: '≈4' });
    expect(rows[0]!.title).toMatch(/Approximate/);
  });

  it('shows a dash for absent numbers', () => {
    const rows = metricRows(fn({ metrics: { ...metrics, maintainability_index: undefined, churn: null } }));
    expect(rows.find((row) => row.label === 'Maintainability')?.value).toBe('—');
    expect(rows.find((row) => row.label === 'Churn')?.value).toBe('—');
  });
});

describe('refAction', () => {
  const changed = new Set(['pkg.changed']);
  it('selects changed functions', () => {
    expect(refAction(ref({ id: 'pkg.changed' }), 'callers', changed)).toEqual({ type: 'selectFunction', id: 'pkg.changed' });
  });
  it('opens callers at the call site and callees at the definition', () => {
    expect(refAction(ref({}), 'callers', changed)).toEqual({ type: 'openLocation', file: 'b.go', line: 7, callSite: true });
    expect(refAction(ref({}), 'callees', changed)).toEqual({ type: 'openLocation', file: 'b.go', line: 7, callSite: false });
  });
  it('does nothing for external refs', () => {
    expect(refAction(ref({ external: true, file: '' }), 'callees', changed)).toEqual({ type: 'none' });
  });
});

describe('connectedFunctions', () => {
  it('finds changed functions that call or are called by a node', () => {
    const context = {
      functions: [fn({ id: 'a', callers: [ref({ id: 'n' })] }), fn({ id: 'b', callees: [ref({ id: 'n' })] }), fn({ id: 'c' })],
    } as unknown as ReviewContext;
    expect(connectedFunctions(context, 'n').map((item) => item.id)).toEqual(['a', 'b']);
  });
});
