import { describe, expect, it } from 'vitest';
import { confidenceFraction, confidenceLevel, confidenceView } from './confidence';

describe('confidence as a word', () => {
  it('maps fractions to low / medium / high at 0.6 and 0.85', () => {
    expect(confidenceLevel(0.3)).toBe('low');
    expect(confidenceLevel(0.599)).toBe('low');
    expect(confidenceLevel(0.6)).toBe('medium');
    expect(confidenceLevel(0.849)).toBe('medium');
    expect(confidenceLevel(0.85)).toBe('high');
    expect(confidenceLevel(1)).toBe('high');
  });

  it('accepts percents and ignores missing values', () => {
    expect(confidenceFraction(85)).toBe(0.85);
    expect(confidenceLevel(72)).toBe('medium');
    expect(confidenceLevel(0)).toBeNull();
    expect(confidenceLevel(Number.NaN)).toBeNull();
    expect(confidenceLevel(undefined)).toBeNull();
    expect(confidenceView(0)).toBeNull();
  });

  it('keeps the number in the tooltip only', () => {
    const view = confidenceView(0.92);
    expect(view?.word).toBe('high');
    expect(view?.word).not.toContain('%');
    expect(view?.title).toContain('0.92');
    expect(view?.title).toContain('92%');
  });
});
