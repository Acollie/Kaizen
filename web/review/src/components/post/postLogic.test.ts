import { describe, expect, it } from 'vitest';
import {
  generateSummary, MAX_WARNING_ITEMS, planPost, postButtonLabel, postedFindings, postWarnings, selectionNote, sortForPost,
} from './postLogic';
import { makeFile, makeFinding } from './testFixtures';
import type { ReviewComment } from '../../state/comments';
import { lookProgress } from '../../state/viewing';
import type { FunctionRisk } from '../../types';

const files = new Map([['pkg/a.go', makeFile()]]);

const comment = (overrides: Partial<ReviewComment> & { id: string }): ReviewComment => ({
  file: 'pkg/a.go', line: 2, body: 'Mine', createdAt: 1, ...overrides,
});

describe('planPost', () => {
  const onDiff = makeFinding({ id: 'on-diff', line: 2 });
  const offDiff = makeFinding({ id: 'off-diff', line: 30 });
  const unreviewed = makeFinding({ id: 'unreviewed', line: 2 });
  const dismissed = makeFinding({ id: 'dismissed', line: 2 });
  const triage = {
    'on-diff': { state: 'accepted' as const, note: 'Checked: the nil path is real.' },
    'off-diff': { state: 'accepted' as const },
    dismissed: { state: 'dismissed' as const, reason: 'False positive' },
  };

  it('posts only accepted findings, never unreviewed or dismissed ones', () => {
    const plan = planPost({ comments: [], findings: [onDiff, offDiff, unreviewed, dismissed], triage, files });
    expect(plan.accepted.map((entry) => entry.finding.id).sort()).toEqual(['off-diff', 'on-diff']);
  });

  it('marks inline vs body with the same rule as findings', () => {
    const plan = planPost({ comments: [comment({ id: 'c-1' }), comment({ id: 'c-2', line: 0 })], findings: [onDiff, offDiff], triage, files });
    const inline = (id: string) => [...plan.comments, ...plan.accepted].find((entry) => entry.finding.id === id)?.inline;
    expect(inline('on-diff')).toBe(true);
    expect(inline('off-diff')).toBe(false);
    expect(inline('c-1')).toBe(true);
    expect(inline('c-2')).toBe(false);
  });

  it('appends the reviewer note to the accepted comment', () => {
    const plan = planPost({ comments: [], findings: [onDiff], triage, files });
    expect(plan.accepted[0]?.finding.body).toBe('Body\n\n**Reviewer note:** Checked: the nil path is real.');
  });

  it('sends your comments first, as findings from "you"', () => {
    const plan = planPost({ comments: [comment({ id: 'c-1', end_line: 3 })], findings: [onDiff], triage, files });
    const sent = postedFindings(plan);
    expect(sent.map((finding) => finding.id)).toEqual(['c-1', 'on-diff']);
    expect(sent[0]).toMatchObject({
      source: 'you', confidence: 1, severity: 'suggestion', category: 'design', title: '', body: 'Mine', line: 2, end_line: 3,
    });
  });

  it('describes and labels the selection', () => {
    const plan = planPost({ comments: [comment({ id: 'c-1' })], findings: [onDiff, offDiff], triage, files });
    expect(selectionNote(plan)).toBe('1 yours · 2 accepted · 2 inline · 1 in body');
    expect(postButtonLabel(plan)).toBe('Post 1 comment and 2 accepted findings');
    expect(postButtonLabel(planPost({ comments: [], findings: [onDiff], triage, files }))).toBe('Post 1 accepted finding');
    expect(postButtonLabel(planPost({ comments: [], findings: [unreviewed], triage, files }))).toBe('Post summary only');
  });
});

describe('postWarnings', () => {
  const focus = [{ id: 'fn', focus: true } as FunctionRisk];
  it('warns about untriaged urgent findings, unviewed focus code and AI text', () => {
    const warnings = postWarnings({
      findings: [makeFinding({ id: 'w', severity: 'warning' }), makeFinding({ id: 's', severity: 'suggestion' })],
      triage: {},
      look: lookProgress(focus, [makeFile()], { functions: {}, files: {} }),
      acceptedCount: 1,
    });
    expect(warnings.map((warning) => warning.text)).toEqual([
      '1 must-fix or should-fix finding not triaged yet.',
      "You haven't looked at 1 focus function or 1 file needing attention.",
      "You're posting AI-written text as yourself. Make sure you've checked each one.",
    ]);
  });

  it('is quiet when everything is triaged and seen', () => {
    const warnings = postWarnings({
      findings: [makeFinding({ id: 'w', severity: 'warning' })],
      triage: { w: { state: 'dismissed' } },
      look: lookProgress(focus, [], { functions: { fn: true }, files: {} }),
      acceptedCount: 0,
    });
    expect(warnings).toEqual([]);
  });
});

describe('postWarnings: what you have not viewed, whatever the AI did', () => {
  const focus = [
    { id: 'a', name: 'Post', receiver_type: '*Client', focus: true } as FunctionRisk,
    { id: 'b', name: 'encode', focus: true } as FunctionRisk,
  ];

  it('names unviewed focus functions and says when the AI only looked at them', () => {
    const warnings = postWarnings({
      findings: [], triage: {}, acceptedCount: 0,
      look: lookProgress(focus, [], { functions: {}, files: {} }),
      aiFunctions: new Set(['a']),
    });
    expect(warnings).toEqual([{
      key: 'unviewed',
      text: "You haven't looked at 2 focus functions.",
      items: [{ label: 'Client.Post', aiLooked: true }, { label: 'encode', aiLooked: false }],
    }]);
  });

  it('lists risky packages you have not opened, capped, including ones the AI looked at', () => {
    const packages = Array.from({ length: MAX_WARNING_ITEMS + 2 }, (_, index) => ({ label: `pkg/p${index}`, aiLooked: index === 0 }));
    const [warning] = postWarnings({
      findings: [], triage: {}, acceptedCount: 0,
      look: lookProgress([], [], { functions: {}, files: {} }),
      unviewedPackages: packages,
    });
    expect(warning!.key).toBe('unviewed-packages');
    expect(warning!.text).toBe("You haven't opened 8 risky packages:");
    expect(warning!.items).toHaveLength(MAX_WARNING_ITEMS);
    expect(warning!.items![0]).toEqual({ label: 'pkg/p0', aiLooked: true });
    expect(warning!.more).toBe(2);
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

  it('mentions your comments', () => {
    expect(generateSummary([], 2)).toBe('Review: 2 comments from me.');
  });

  it('says so when there is nothing to fix', () => {
    expect(generateSummary([makeFinding({ id: 'p', severity: 'praise' })])).toBe('Kaizen review: no issues found.');
  });
});
