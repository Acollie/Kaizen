import { describe, expect, it } from 'vitest';
import { makeFinding } from '../components/post/testFixtures';
import { captureReveal, compareSentence, compareView, overlaps, parseRevealMoment } from './compare';
import type { ReviewComment } from './comments';

const comment = (overrides: Partial<ReviewComment> & { id: string }): ReviewComment =>
  ({ file: 'pkg/a.go', line: 10, body: 'mine', createdAt: 100, ...overrides });

describe('overlap by file:line (±3)', () => {
  it('matches within three lines of the comment range in the same file', () => {
    expect(overlaps(comment({ id: 'c', file: 'pkg/a.go' }), makeFinding({ id: 'f', file: 'pkg/a.go', line: 13 }))).toBe(true);
    expect(overlaps(comment({ id: 'c' }), makeFinding({ id: 'f', file: 'pkg/a.go', line: 14 }))).toBe(false);
    expect(overlaps(comment({ id: 'c', end_line: 20 }), makeFinding({ id: 'f', file: 'pkg/a.go', line: 23 }))).toBe(true);
    expect(overlaps(comment({ id: 'c' }), makeFinding({ id: 'f', file: 'pkg/a.go', line: 2, end_line: 8 }))).toBe(true);
    expect(overlaps(comment({ id: 'c' }), makeFinding({ id: 'f', file: 'pkg/b.go', line: 10 }))).toBe(false);
  });

  it('matches file-level with file-level only', () => {
    expect(overlaps(comment({ id: 'c', line: 0 }), makeFinding({ id: 'f', file: 'pkg/a.go', line: 0 }))).toBe(true);
    expect(overlaps(comment({ id: 'c', line: 0 }), makeFinding({ id: 'f', file: 'pkg/a.go', line: 4 }))).toBe(false);
  });
});

describe('compare view', () => {
  const reveal = { at: 500, viewedFunctions: ['fn-b'], viewedFiles: ['pkg/c.go'] };
  const fileOfFunction = (id: string) => ({ 'fn-b': 'pkg/b.go' } as Record<string, string>)[id];

  it('counts your pre-reveal comments, overlaps and findings outside your pass', () => {
    const view = compareView({
      reveal,
      comments: [comment({ id: 'mine', file: 'pkg/a.go', line: 10 }), comment({ id: 'later', file: 'pkg/d.go', line: 5, createdAt: 900 })],
      findings: [
        makeFinding({ id: 'same-spot', file: 'pkg/a.go', line: 12 }),
        makeFinding({ id: 'viewed-fn-file', file: 'pkg/b.go', line: 40 }),
        makeFinding({ id: 'viewed-file', file: 'pkg/c.go', line: 1 }),
        makeFinding({ id: 'unseen', file: 'pkg/d.go', line: 5 }),
      ],
      fileOfFunction,
    });
    expect(view.yours).toBe(1);
    expect(view.ai).toBe(4);
    expect([...view.overlapIds]).toEqual(['same-spot']);
    // pkg/d.go: only commented after the reveal, so it was outside the pass
    expect([...view.outsideIds]).toEqual(['unseen']);
    expect(compareSentence(view)).toBe('You flagged 1 thing yourself · AI raised 4 · 1 overlap by file:line (±3)');
  });

  it('captures and parses the reveal moment', () => {
    const moment = captureReveal(42, { functions: { f: true }, files: { 'a.go': true } });
    expect(moment).toEqual({ at: 42, viewedFunctions: ['f'], viewedFiles: ['a.go'] });
    expect(parseRevealMoment(JSON.parse(JSON.stringify(moment)))).toEqual(moment);
    expect(parseRevealMoment({ at: 'x' })).toBeNull();
    expect(parseRevealMoment(null)).toBeNull();
    expect(parseRevealMoment({ at: 1, viewedFiles: [3] })).toEqual({ at: 1, viewedFunctions: [], viewedFiles: [] });
  });
});
