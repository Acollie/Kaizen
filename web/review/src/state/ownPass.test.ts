import { describe, expect, it } from 'vitest';
import type { FileDiff, FunctionRisk } from '../types';
import { buildPass, nextPassItem, passProgress, passPrompt, passSentence, revealLabel } from './ownPass';

const file = (path: string, risk = 0, reasons: string[] = []): FileDiff =>
  ({ path, old_path: path, status: 'modified', language: 'Go', additions: 20, deletions: 2, binary: false, hunks: [], line_count: 100, risk_score: risk, risk_reasons: reasons });
const fn = (id: string, path: string, risk: number, focus: boolean): FunctionRisk =>
  ({ id, name: id, file: path, language: 'Go', start_line: 1, end_line: 10, lines_added: 5, lines_removed: 0, is_new: false, fan_in: 0, fan_in_approximate: false, callers: [], callees: [], risk_score: risk, risk_reasons: [`complexity ${risk}`], focus });

function input() {
  const files = [file('pkg/core/load.go', 40), file('pkg/util/str.go', 3), file('.github/workflows/ci.yml', 30, ['CI workflow changed'])];
  const functions = [fn('core.Load', 'pkg/core/load.go', 40, true), fn('core.parse', 'pkg/core/load.go', 18, true), fn('util.Trim', 'pkg/util/str.go', 3, false)];
  const functionsByFile = new Map<string, FunctionRisk[]>();
  for (const item of functions) functionsByFile.set(item.file, [...(functionsByFile.get(item.file) ?? []), item]);
  return { files, functions, graph: { nodes: [], edges: [] }, functionsByFile };
}

describe('your pass', () => {
  it('lists Where to look packages, focus functions and risky non-code files', () => {
    const items = buildPass(input());
    expect(items.map((item) => item.kind)).toEqual(['package', 'package', 'function', 'function', 'file']);
    expect(items.map((item) => item.label)).toEqual(['pkg/core', '.github/workflows', 'core.Load', 'core.parse', '.github/workflows/ci.yml']);
  });

  it('never mentions the AI', () => {
    const text = buildPass(input()).map((item) => `${item.label} ${item.reason}`).join(' ');
    expect(text).not.toMatch(/\bAI\b|finding|looked at|coverage/i);
  });

  it('tracks progress from what you viewed', () => {
    const items = buildPass(input());
    const none = passProgress(items, { functions: {}, files: {} });
    expect(passSentence(none)).toBe('0 of 5 viewed');
    expect(nextPassItem(items, none)?.label).toBe('pkg/core');
    const some = passProgress(items, { functions: { 'core.parse': true }, files: {} });
    // Viewing a function in pkg/core counts for the package too
    expect(passSentence(some)).toBe('2 of 5 viewed');
    const all = passProgress(items, { functions: { 'core.Load': true, 'core.parse': true }, files: { '.github/workflows/ci.yml': true } });
    expect(all.done).toBe(true);
    expect(passPrompt(all, false)).toBe("You've done your pass. Compare with the AI.");
    expect(nextPassItem(items, all)).toBeNull();
  });

  it('labels the reveal button with the number ready', () => {
    expect(revealLabel(12)).toBe('Reveal AI findings (12 ready)');
  });
});
