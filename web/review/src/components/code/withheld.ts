// Why a generated file's diff was withheld, from the reason the server recorded in
// risk_reasons (e.g. "lockfile: diff not shown or sent to the AI; check dependency
// changes in the manifest").

import type { FileDiff } from '../../types';

const WITHHELD_MARKER = /:\s*diff not shown or sent to the AI;?\s*/i;

export interface WithheldReason {
  /** Short cause, e.g. "lockfile" or "minified file" */
  reason: string;
  /** Optional advice that followed the cause */
  hint: string;
}

export function withheldReason(diff: Pick<FileDiff, 'risk_reasons'>): WithheldReason {
  const reasons = (diff.risk_reasons ?? []).filter(Boolean);
  const recorded = reasons.find((text) => WITHHELD_MARKER.test(text));
  if (!recorded) return { reason: 'generated file', hint: '' };
  const [reason = '', hint = ''] = recorded.split(WITHHELD_MARKER);
  return { reason: reason.trim() || 'generated file', hint: hint.trim() };
}
