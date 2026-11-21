import { describe, expect, test } from 'vitest';
import type { ReviewEvent } from '../../types';
import { activityAgents, clockTime, eventAgent, eventText, groupRuns, isNearBottom } from './activityModel';

const time = '2026-11-20T00:17:00Z';
const status = (seq: number, agent: string, message = 'working'): ReviewEvent => ({ seq, type: 'status', time, data: { message, agent } });
const tool = (seq: number, agent: string): ReviewEvent => ({ seq, type: 'tool_call', time, data: { tool: 'Read', summary: 'Read(a.go)', agent } });
const finding = (seq: number, source?: string): ReviewEvent => ({
  seq, type: 'finding', time,
  data: { id: `f${seq}`, severity: 'warning', category: 'design', file: 'a.go', title: 'Problem', body: '', confidence: 1, source },
});

describe('eventAgent and eventText', () => {
  test('agents default to lead', () => {
    expect(eventAgent(status(1, ''))).toBe('lead');
    expect(eventAgent(finding(2, 'security-reviewer'))).toBe('security-reviewer');
    expect(eventAgent(finding(3))).toBe('lead');
    expect(eventAgent({ seq: 4, type: 'done', time, data: null })).toBe('lead');
  });
  test('text per type', () => {
    expect(eventText(tool(1, 'lead'))).toBe('Read(a.go)');
    expect(eventText({ seq: 2, type: 'error', time, data: { text: '' } })).toBe('Engine error');
    expect(eventText({ seq: 3, type: 'result', time, data: { summary: '', verdict: 'comment', findings: null, engine: 'x' } })).toBe('Review complete · 0 findings');
  });
});

describe('groupRuns', () => {
  const events = [status(1, 'lead'), tool(2, 'lead'), status(3, 'hotspot-reviewer'), tool(4, 'hotspot-reviewer'), finding(5, 'hotspot-reviewer'), tool(6, 'lead')];
  test('folds consecutive events by agent', () => {
    expect(groupRuns(events, null).map((run) => [run.agent, run.events.length, run.key])).toEqual([
      ['lead', 2, 1], ['hotspot-reviewer', 3, 3], ['lead', 1, 6],
    ]);
  });
  test('filters by agent and merges the remaining runs', () => {
    expect(groupRuns(events, 'lead').map((run) => run.events.length)).toEqual([3]);
  });
  test('activityAgents lists agents in order of appearance', () => {
    expect(activityAgents(events)).toEqual(['lead', 'hotspot-reviewer']);
  });
});

test('clockTime is 24-hour and zero-padded', () => {
  const local = new Date(2026, 10, 20, 9, 5, 7).toISOString();
  expect(clockTime(local)).toBe('09:05:07');
  expect(clockTime('nonsense')).toBe('');
});

test('isNearBottom', () => {
  expect(isNearBottom({ scrollTop: 480, scrollHeight: 1000, clientHeight: 500 })).toBe(true);
  expect(isNearBottom({ scrollTop: 100, scrollHeight: 1000, clientHeight: 500 })).toBe(false);
});
