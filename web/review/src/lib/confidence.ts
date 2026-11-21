// How a finding's confidence is shown. Model confidence proved poorly calibrated in
// blind evaluations (severity was well calibrated), so the UI leads with severity and
// shows confidence only as a small word, with the number in a tooltip. Every surface
// that shows confidence goes through confidenceView.

export type ConfidenceLevel = 'low' | 'medium' | 'high';

export const LOW_BELOW = 0.6;
export const HIGH_FROM = 0.85;

export interface ConfidenceView {
  level: ConfidenceLevel;
  /** "low" / "medium" / "high" */
  word: string;
  /** Tooltip with the number and a calibration caveat */
  title: string;
}

/** Confidence as a 0-1 fraction (percent values above 1 are scaled down); null when missing */
export function confidenceFraction(confidence: number | undefined): number | null {
  if (confidence === undefined || !Number.isFinite(confidence) || confidence <= 0) return null;
  return confidence > 1 ? Math.min(1, confidence / 100) : confidence;
}

/** < 0.6 low, 0.6-0.85 medium, >= 0.85 high; null when there is no confidence */
export function confidenceLevel(confidence: number | undefined): ConfidenceLevel | null {
  const fraction = confidenceFraction(confidence);
  if (fraction === null) return null;
  if (fraction < LOW_BELOW) return 'low';
  if (fraction < HIGH_FROM) return 'medium';
  return 'high';
}

export function confidenceView(confidence: number | undefined): ConfidenceView | null {
  const fraction = confidenceFraction(confidence);
  const level = confidenceLevel(confidence);
  if (level === null || fraction === null) return null;
  return {
    level,
    word: level,
    title: `Model confidence ${fraction.toFixed(2)} (${Math.round(fraction * 100)}%). Poorly calibrated: judge by severity and your own check.`,
  };
}
