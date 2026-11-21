// Fixtures shared by the AI coverage and blind-spot tests.

import type { FunctionRisk, ReviewEvent, ToolCallData } from '../types';

export function makeFunction(id: string, file: string, start: number, end: number, overrides: Partial<FunctionRisk> = {}): FunctionRisk {
  return {
    id, name: id.split('.').pop() ?? id, file, language: 'Go', start_line: start, end_line: end,
    lines_added: 1, lines_removed: 0, is_new: false, fan_in: 0, fan_in_approximate: false,
    callers: [], callees: [], risk_score: 10, risk_reasons: [], focus: false, ...overrides,
  };
}

let seq = 0;
export function toolCall(data: ToolCallData): ReviewEvent {
  seq += 1;
  return { seq, type: 'tool_call', time: '2026-01-01T00:00:00Z', data };
}
