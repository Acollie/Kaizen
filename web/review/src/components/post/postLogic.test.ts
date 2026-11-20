import { describe, expect, it } from 'vitest';
import { generateSummary, planPost, selectedFindings, selectionNote, sortForPost } from './postLogic';
import { makeFile, makeFinding } from './testFixtures';

const files = new Map([['pkg/a.go', makeFile()]]);

describe('planPost', () => {
  it('puts findings on a diff line inline and the rest in the body', () => {
    const onDiff = makeFinding({ id: 'on-diff', line: 2 });
    const offDiff = makeFinding({ id: 'off-diff', line: 30 });
    const fileLevel = makeFinding({ id: 'file-level', line: 0 });
    const otherFile = makeFinding({ id: 'other', file: 'pkg/b.go', line: 2 });
    const plan = planPost([onDiff, offDiff, fileLevel, otherFile], files);
    expect(plan.inline.map((finding) => finding.id)).toEqual(['on-diff']);
    expect(plan.body.map((finding) => finding.id).sort()).toEqual(['file-level', 'off-diff', 'other']);
  });
});

describe('sortForPost', () => {
  it('orders by severity, then file, then line', () => {
    const sorted = sortForPost([
      makeFinding({ id: 's', severity: 'suggestion', line: 1 }),
      makeFinding({ id: 'w2', severity: 'warning', line: 9 }),
      makeFinding({ id: 'c', severity: 'critical', file: 'z.go' }),
      makeFinding({ id: 'w1', severity: 'warning', line: 3 }),
    ]);
    expect(sorted.map((finding) => finding.id)).toEqual(['c', 'w1', 'w2', 's']);
  });
});

describe('selection', () => {
  const inline = makeFinding({ id: 'inline', line: 2 });
  const body = makeFinding({ id: 'body', line: 0 });
  const plan = planPost([inline, body], files);

  it('keeps only included findings', () => {
    expect(selectedFindings([inline, body], { body: true }).map((finding) => finding.id)).toEqual(['body']);
  });

  it('describes the split', () => {
    expect(selectionNote(plan, { inline: true, body: true })).toBe('2 selected · 1 inline · 1 in body');
    expect(selectionNote(plan, { inline: false })).toBe('0 selected · 0 inline · 0 in body');
  });
});

describe('generateSummary', () => {
  it('counts actionable findings and lists the urgent ones', () => {
    const summary = generateSummary([
      makeFinding({ id: '1', severity: 'critical', title: 'Nil deref', line: 4 }),
      makeFinding({ id: '2', severity: 'suggestion', title: 'Rename' }),
      makeFinding({ id: '3', severity: 'praise', title: 'Nice' }),
    ]);
    expect(summary.split('\n')[0]).toBe('Kaizen review: 1 must fix, 1 to consider.');
    expect(summary).toContain('- **Nil deref** (`pkg/a.go` L4)');
    expect(summary).not.toContain('Rename');
  });

  it('says so when there is nothing to fix', () => {
    expect(generateSummary([makeFinding({ id: 'p', severity: 'praise' })])).toBe('Kaizen review: no issues found.');
  });
});
