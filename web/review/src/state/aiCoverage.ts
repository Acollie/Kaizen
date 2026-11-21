// What the AI looked at (opened; not verified), derived from the engine's tool_call events. Lets the UI show
// blind spots (changed code the AI never opened) so a review is never mistaken for
// complete coverage.
//
// CONTRACT (implementation owned by the "evidence & blind spots" work):
//   useAiCoverage() -> AiCoverage, recomputed as activity streams in.

import { useMemo } from 'react';
import { useReview } from './StoreProvider';
import { findingsHidden, type EngineState } from './store';
import type { FileDiff, FunctionRisk, ReviewEvent, ToolCallData } from '../types';

export interface AiCoverage {
  /** Changed-function IDs the AI opened (get_function / get_callers / get_callees, or read their lines) */
  examinedFunctions: ReadonlySet<string>;
  /** Repo-relative paths of PR files the AI opened (diff or contents) */
  examinedFiles: ReadonlySet<string>;
  /** False while the review is still running (blind spots may still be filled in) */
  complete: boolean;
  /** False when the engine reported no tool targets at all (e.g. an old cached review) */
  known: boolean;
  /**
   * True while "Think first" hides the AI's work: nothing about what the AI looked at
   * may be shown, so the reviewer's own pass is not anchored by it
   */
  hidden: boolean;
}

export interface CoverageInput {
  activity: readonly ReviewEvent[];
  functions: readonly FunctionRisk[];
  files: readonly FileDiff[];
  statusState: EngineState;
  hasResult: boolean;
}

/** An inclusive head-side line range; end Infinity means "to the end of the file" */
export interface LineRange { start: number; end: number }

const WHOLE_FILE: LineRange = { start: 1, end: Infinity };

/**
 * The lines a file-reading call opened, parsed from its summary:
 * "read_lines(path:1-40)", "Read(path:4-13)", "Read(path:30-)" or "Read(path)" (whole file).
 * Returns null for tools that do not read file contents (get_file_diff shows hunks only).
 */
export function readRange(call: ToolCallData): LineRange | null {
  if (call.tool !== 'read_lines' && call.tool !== 'Read') return null;
  const match = /:(\d+)-(\d*)\)$/.exec(call.summary);
  if (!match) return call.tool === 'Read' ? WHOLE_FILE : null;
  const start = Number(match[1]);
  const end = match[2] ? Number(match[2]) : Infinity;
  return end >= start ? { start, end } : null;
}

/** True when the range shows at least half of the function's lines */
export function rangeCoversFunction(range: LineRange, fn: Pick<FunctionRisk, 'start_line' | 'end_line'>): boolean {
  const start = fn.start_line;
  const end = Math.max(fn.end_line, start);
  const overlap = Math.min(end, range.end) - Math.max(start, range.start) + 1;
  return overlap > 0 && overlap * 2 >= end - start + 1;
}

function isFinished(statusState: EngineState, hasResult: boolean): boolean {
  return statusState === 'done' || statusState === 'error' || hasResult;
}

/** Pure derivation behind useAiCoverage, exported for tests */
export function deriveAiCoverage({ activity, functions, files, statusState, hasResult }: CoverageInput): AiCoverage {
  const changedIds = new Set(functions.map((fn) => fn.id));
  const prFiles = new Set(files.map((file) => file.path));
  const functionsByFile = new Map<string, FunctionRisk[]>();
  for (const fn of functions) functionsByFile.set(fn.file, [...(functionsByFile.get(fn.file) ?? []), fn]);

  const examinedFunctions = new Set<string>();
  const examinedFiles = new Set<string>();
  let known = false;
  for (const event of activity) {
    if (event.type !== 'tool_call' || !event.data?.target) continue;
    known = true;
    const { target, target_kind: kind } = event.data;
    if (kind === 'function') {
      if (changedIds.has(target)) examinedFunctions.add(target);
      continue;
    }
    if (kind !== 'file') continue;
    if (prFiles.has(target)) examinedFiles.add(target);
    const range = readRange(event.data);
    if (!range) continue;
    for (const fn of functionsByFile.get(target) ?? []) {
      if (rangeCoversFunction(range, fn)) examinedFunctions.add(fn.id);
    }
  }
  for (const fn of functions) {
    if (examinedFunctions.has(fn.id) && prFiles.has(fn.file)) examinedFiles.add(fn.file);
  }
  const complete = isFinished(statusState, hasResult);
  return { examinedFunctions, examinedFiles, complete, known, hidden: false };
}

const HIDDEN_COVERAGE: AiCoverage = {
  examinedFunctions: new Set(), examinedFiles: new Set(), complete: false, known: false, hidden: true,
};

export function useAiCoverage(): AiCoverage {
  const hidden = useReview(findingsHidden);
  const activity = useReview((state) => state.activity);
  const functions = useReview((state) => state.context.functions);
  const files = useReview((state) => state.context.files);
  // Select only "finished" so running <-> thinking transitions do not recompute
  const finished = useReview((state) => isFinished(state.status.state, state.result !== null));
  return useMemo(
    () => (hidden ? HIDDEN_COVERAGE : deriveAiCoverage({
      activity, functions: functions ?? [], files: files ?? [], statusState: finished ? 'done' : 'running', hasResult: finished,
    })),
    [hidden, activity, functions, files, finished],
  );
}
