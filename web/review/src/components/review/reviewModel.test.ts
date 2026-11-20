import { describe, expect, test } from 'vitest';
import type { FileDiff, Finding, FunctionRisk } from '../../types';
import {
  clusterAgents, countsSentence, firstSentence, focusFunctions, formatConfidence, locationLabel,
  riskyFiles, severitySections, verdictView,
} from './reviewModel';

const finding = (overrides: Partial<Finding>): Finding => ({
  id: 'f1', severity: 'warning', category: 'correctness', file: 'internal/config/config.go', line: 10,
  title: 'Title', body: '', confidence: 0.8, ...overrides,
});

describe('verdictView', () => {
  test('maps verdicts to labels and tones', () => {
    expect(verdictView('request_changes')).toEqual({ label: 'Changes requested', tone: 'critical' });
    expect(verdictView('approve')).toEqual({ label: 'Approved', tone: 'praise' });
    expect(verdictView('comment')).toEqual({ label: 'Comments', tone: 'warning' });
    expect(verdictView('unknown')).toEqual({ label: 'Comments', tone: 'warning' });
  });
});

describe('countsSentence', () => {
  test('joins counts with commas and "and"', () => {
    const findings = [
      finding({ id: 'a', severity: 'critical' }), finding({ id: 'b', severity: 'critical' }),
      finding({ id: 'c', severity: 'warning' }), finding({ id: 'd', severity: 'suggestion' }),
    ];
    expect(countsSentence(findings)).toBe('2 must fix, 1 should fix and 1 suggestion');
  });
  test('single and empty cases', () => {
    expect(countsSentence([finding({ severity: 'suggestion' }), finding({ id: 'x', severity: 'suggestion' })])).toBe('2 suggestions');
    expect(countsSentence([])).toBe('No findings');
  });
});

describe('severitySections', () => {
  test('orders sections by severity and folds near-duplicates', () => {
    const sections = severitySections([
      finding({ id: 'a', severity: 'suggestion', line: 50 }),
      finding({ id: 'b', severity: 'warning', line: 10, source: 'hotspot-reviewer' }),
      finding({ id: 'c', severity: 'warning', line: 12, source: 'security-reviewer', confidence: 0.5 }),
    ]);
    expect(sections.map((section) => section.title)).toEqual(['Should fix', 'Consider']);
    expect(sections[0]!.clusters).toHaveLength(1);
    expect(sections[0]!.clusters[0]!.similar.map((item) => item.id)).toEqual(['c']);
    expect(clusterAgents(sections[0]!.clusters[0]!)).toEqual(['hotspot-reviewer', 'security-reviewer']);
  });
});

describe('firstSentence', () => {
  test('takes the first sentence and strips markup', () => {
    expect(firstSentence('For `num == math.MinInt`, **this** overflows. Use strconv.Itoa instead.')).toBe('For num == math.MinInt, this overflows.');
  });
  test('ignores later paragraphs and code blocks', () => {
    expect(firstSentence('No period here\n\nSecond paragraph.')).toBe('No period here');
    expect(firstSentence('```go\nx := 1\n```')).toBe('go x := 1');
  });
  test('does not split on dotted identifiers', () => {
    expect(firstSentence('Calls pkg.Func twice. Then more.')).toBe('Calls pkg.Func twice.');
  });
  test('clamps very long text', () => {
    expect(firstSentence('a'.repeat(400)).length).toBe(220);
  });
});

describe('small formatters', () => {
  test('locationLabel uses the base name', () => {
    expect(locationLabel({ file: 'internal/config/config.go', line: 118 })).toBe('config.go:118');
    expect(locationLabel({ file: 'action.yml' })).toBe('action.yml');
  });
  test('formatConfidence accepts fractions and percents', () => {
    expect(formatConfidence(0.92)).toBe('92%');
    expect(formatConfidence(85)).toBe('85%');
    expect(formatConfidence(0)).toBe('');
  });
});

describe('focus lists', () => {
  const fn = (id: string, risk: number, focus: boolean, file = 'a.go') =>
    ({ id, name: id, file, risk_score: risk, focus }) as unknown as FunctionRisk;
  test('focusFunctions keeps focus only, riskiest first', () => {
    expect(focusFunctions([fn('a', 5, true), fn('b', 30, true), fn('c', 90, false)]).map((item) => item.id)).toEqual(['b', 'a']);
  });
  test('riskyFiles keeps files with reasons and no functions', () => {
    const file = (path: string, reasons: string[], risk = 0) => ({ path, risk_reasons: reasons, risk_score: risk }) as unknown as FileDiff;
    const byFile = new Map([['a.go', [fn('a', 1, false)]]]);
    const result = riskyFiles([file('a.go', ['x']), file('action.yml', ['CI config changed'], 10), file('README.md', [])], byFile);
    expect(result.map((item) => item.path)).toEqual(['action.yml']);
  });
});
