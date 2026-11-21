// Risk colour scale for the dark theme. Kaizen risk scores are 0-100 on absolute
// thresholds, so most real functions sit well below 50: the scale saturates early
// (amber by ~25, red by ~55) so meaningful differences are visible.

const STOPS: readonly [number, readonly [number, number, number]][] = [
  [0, [92, 99, 112]], //   slate: nothing notable
  [12, [94, 161, 255]], // blue: some signal
  [28, [245, 165, 36]], // amber: notable
  [55, [255, 93, 93]], //  red: high
  [100, [255, 93, 93]],
];

export function riskColor(score: number): string {
  const value = Math.max(0, Math.min(100, Number(score) || 0));
  for (let index = 1; index < STOPS.length; index++) {
    const [upper, upperColor] = STOPS[index]!;
    if (value <= upper) {
      const [lower, lowerColor] = STOPS[index - 1]!;
      const ratio = upper === lower ? 0 : (value - lower) / (upper - lower);
      const channel = (offset: 0 | 1 | 2) => Math.round(lowerColor[offset] + (upperColor[offset] - lowerColor[offset]) * ratio);
      return `rgb(${channel(0)}, ${channel(1)}, ${channel(2)})`;
    }
  }
  return 'rgb(255, 93, 93)';
}

export type RiskLevel = 'none' | 'low' | 'notable' | 'high';

export function riskLevel(score: number): RiskLevel {
  if (score >= 45) return 'high';
  if (score >= 20) return 'notable';
  if (score >= 5) return 'low';
  return 'none';
}
