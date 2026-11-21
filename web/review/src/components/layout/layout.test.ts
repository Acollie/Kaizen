import { describe, expect, it } from 'vitest';
import {
  clampWidth, DEFAULT_WIDTHS, effectiveWidths, MIN_WIDTH, parseStoredWidths, RAIL_WIDTH, widthFromPointer,
} from './layoutMath';
import { activityBadge, badgeText, reviewBadge } from './tabBadges';
import { makeFinding } from '../post/testFixtures';

describe('clampWidth', () => {
  it('keeps a reasonable width', () => {
    expect(clampWidth('sidebar', 320, 1440, 440)).toBe(320);
  });
  it('enforces the minimum', () => {
    expect(clampWidth('sidebar', 50, 1440, 440)).toBe(MIN_WIDTH.sidebar);
  });
  it('caps at the share of the workspace', () => {
    expect(clampWidth('sidebar', 900, 1440, 300)).toBe(Math.round(1440 * 0.4));
  });
  it('leaves room for the code view', () => {
    // 1280 - 440 - 360 = 480, below the 60% share (768)
    expect(clampWidth('context', 700, 1280, 440)).toBe(480);
  });
  it('lets the minimum win on tiny screens', () => {
    expect(clampWidth('context', 500, 600, 300)).toBe(MIN_WIDTH.context);
  });
});

describe('effectiveWidths', () => {
  it('uses rails for collapsed panes', () => {
    const widths = effectiveWidths(DEFAULT_WIDTHS, 1440, { sidebar: true, context: false });
    expect(widths.sidebar).toBe(RAIL_WIDTH);
    expect(widths.context).toBe(DEFAULT_WIDTHS.context);
  });
  it('returns the preferred widths before the workspace is measured', () => {
    expect(effectiveWidths({ sidebar: 999, context: 999 }, 0, { sidebar: false, context: false })).toEqual({ sidebar: 999, context: 999 });
  });
  it('shrinks both panes at 1000px', () => {
    const widths = effectiveWidths(DEFAULT_WIDTHS, 1000, { sidebar: false, context: false });
    expect(1000 - widths.sidebar - widths.context).toBeGreaterThanOrEqual(360 - 1);
  });
});

describe('widthFromPointer', () => {
  it('measures from the left for the sidebar and from the right for the context', () => {
    expect(widthFromPointer('sidebar', 350, { left: 0, right: 1440 })).toBe(350);
    expect(widthFromPointer('context', 1000, { left: 0, right: 1440 })).toBe(440);
  });
});

describe('parseStoredWidths', () => {
  it('falls back to defaults for missing or malformed data', () => {
    expect(parseStoredWidths(null)).toEqual(DEFAULT_WIDTHS);
    expect(parseStoredWidths('{nope')).toEqual(DEFAULT_WIDTHS);
    expect(parseStoredWidths('{"sidebar":"wide","context":-3}')).toEqual(DEFAULT_WIDTHS);
  });
  it('reads valid widths', () => {
    expect(parseStoredWidths('{"sidebar":260,"context":500}')).toEqual({ sidebar: 260, context: 500 });
  });
});

describe('tab badges', () => {
  it('shows urgent findings in the worst colour', () => {
    const badge = reviewBadge([
      makeFinding({ id: '1', severity: 'warning' }),
      makeFinding({ id: '2', severity: 'critical' }),
      makeFinding({ id: '3', severity: 'suggestion' }),
    ]);
    expect(badge).toMatchObject({ text: '2', tone: 'critical' });
  });
  it('uses warning colour when nothing is critical', () => {
    expect(reviewBadge([makeFinding({ id: '1', severity: 'warning' })])).toMatchObject({ text: '1', tone: 'warning' });
  });
  it('shows the neutral total when nothing is urgent', () => {
    expect(reviewBadge([makeFinding({ id: '1', severity: 'suggestion' }), makeFinding({ id: '2', severity: 'praise' })]))
      .toMatchObject({ text: '2', tone: 'neutral' });
  });
  it('shows nothing without findings', () => {
    expect(reviewBadge([])).toBeNull();
  });
  it('counts unseen activity', () => {
    expect(activityBadge(10, 4)).toMatchObject({ text: '6' });
    expect(activityBadge(4, 4)).toBeNull();
    expect(badgeText(250)).toBe('99+');
  });
});
