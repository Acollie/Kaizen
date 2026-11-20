// What the human has actually looked at. A changed function counts as viewed once it
// was selected with its code on screen for VIEW_DWELL_MS; a file once it was open that
// long. These helpers turn that into "You've looked at 2/3 focus functions" and into
// the warnings the post dialog shows. All pure.

import type { FileDiff, Finding, FunctionRisk } from '../types';
import { isUrgent } from '../lib/findings';
import type { AiCoverage } from './aiCoverage';
import type { TriageMap } from './triage';

export const VIEW_DWELL_MS = 2000;

export type ViewKind = 'function' | 'file';

export interface ViewedState {
  functions: Readonly<Record<string, true>>;
  files: Readonly<Record<string, true>>;
}

export const EMPTY_VIEWED: ViewedState = { functions: {}, files: {} };

/** Focus functions, riskiest first */
export function focusFunctions(functions: readonly FunctionRisk[]): FunctionRisk[] {
  return functions.filter((fn) => fn.focus).sort((left, right) => right.risk_score - left.risk_score);
}

/** Files Kaizen flags with reasons but no parsed functions (workflows, action.yml, scripts) */
export function riskyFiles(files: readonly FileDiff[], functionsByFile: ReadonlyMap<string, readonly FunctionRisk[]>): FileDiff[] {
  return files
    .filter((file) => (file.risk_reasons ?? []).some(Boolean) && !(functionsByFile.get(file.path)?.length))
    .sort((left, right) => right.risk_score - left.risk_score);
}

/**
 * Files needing attention: files with an urgent AI finding the reviewer has not
 * dismissed, plus files Kaizen flags that have no functions to look at instead.
 * `findings` should be the visible ones (none while "Think first" hides them).
 */
export function attentionFiles(input: {
  files: readonly FileDiff[];
  functionsByFile: ReadonlyMap<string, readonly FunctionRisk[]>;
  findings: readonly Finding[];
  triage: TriageMap;
}): FileDiff[] {
  const urgentPaths = new Set(input.findings
    .filter((finding) => isUrgent(finding) && input.triage[finding.id]?.state !== 'dismissed')
    .map((finding) => finding.file));
  const flagged = new Set(riskyFiles(input.files, input.functionsByFile).map((file) => file.path));
  return input.files.filter((file) => urgentPaths.has(file.path) || flagged.has(file.path));
}

export interface SeenCount<T> {
  total: number;
  seen: number;
  unseen: T[];
}

export interface LookProgress {
  focus: SeenCount<FunctionRisk>;
  files: SeenCount<FileDiff>;
}

function countSeen<T>(items: readonly T[], isSeen: (item: T) => boolean): SeenCount<T> {
  const unseen = items.filter((item) => !isSeen(item));
  return { total: items.length, seen: items.length - unseen.length, unseen };
}

export function lookProgress(focus: readonly FunctionRisk[], attention: readonly FileDiff[], viewed: ViewedState): LookProgress {
  return {
    focus: countSeen(focus, (fn) => Boolean(viewed.functions[fn.id])),
    files: countSeen(attention, (file) => Boolean(viewed.files[file.path])),
  };
}

/** "You've looked at 2/3 focus functions · 1/3 files needing attention"; "" when there is nothing to look at */
export function lookSentence(progress: LookProgress): string {
  const parts: string[] = [];
  if (progress.focus.total > 0) parts.push(`${progress.focus.seen}/${progress.focus.total} focus ${progress.focus.total === 1 ? 'function' : 'functions'}`);
  if (progress.files.total > 0) parts.push(`${progress.files.seen}/${progress.files.total} ${progress.files.total === 1 ? 'file' : 'files'} needing attention`);
  return parts.length ? `You've looked at ${parts.join(' · ')}` : '';
}

export function allFocusSeen(progress: LookProgress): boolean {
  return progress.focus.total > 0 && progress.focus.unseen.length === 0;
}

export function everythingSeen(progress: LookProgress): boolean {
  return progress.focus.unseen.length === 0 && progress.files.unseen.length === 0;
}

export type LookTarget = { kind: 'function'; id: string } | { kind: 'file'; path: string };

/** The next thing to look at: unseen focus functions first, then files */
export function nextUnseen(progress: LookProgress): LookTarget | null {
  const fn = progress.focus.unseen[0];
  if (fn) return { kind: 'function', id: fn.id };
  const file = progress.files.unseen[0];
  if (file) return { kind: 'file', path: file.path };
  return null;
}

/** True when the finished AI review never opened this changed function */
export function aiSkippedFunction(coverage: AiCoverage, functionId: string): boolean {
  return coverage.known && coverage.complete && !coverage.examinedFunctions.has(functionId);
}

export function withViewed(viewed: ViewedState, kind: ViewKind, id: string): ViewedState {
  const bucket = kind === 'function' ? viewed.functions : viewed.files;
  if (bucket[id]) return viewed;
  return kind === 'function'
    ? { ...viewed, functions: { ...viewed.functions, [id]: true } }
    : { ...viewed, files: { ...viewed.files, [id]: true } };
}
