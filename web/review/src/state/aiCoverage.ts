// What the AI examined, derived from the engine's tool_call events. Lets the UI show
// blind spots (changed code the AI never opened) so a review is never mistaken for
// complete coverage.
//
// CONTRACT (implementation owned by the "evidence & blind spots" work):
//   useAiCoverage() -> AiCoverage, recomputed as activity streams in.

export interface AiCoverage {
  /** Changed-function IDs the AI opened (get_function / get_callers / get_callees, or read their lines) */
  examinedFunctions: ReadonlySet<string>;
  /** Repo-relative paths of PR files the AI opened (diff or contents) */
  examinedFiles: ReadonlySet<string>;
  /** False while the review is still running (blind spots may still be filled in) */
  complete: boolean;
  /** False when the engine reported no tool targets at all (e.g. an old cached review) */
  known: boolean;
}

const EMPTY: AiCoverage = { examinedFunctions: new Set(), examinedFiles: new Set(), complete: false, known: false };

/** STUB: replaced with the real derivation */
export function useAiCoverage(): AiCoverage {
  return EMPTY;
}
