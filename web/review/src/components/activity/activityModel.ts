// Pure helpers for the activity feed: which agent an event belongs to, its one-line
// text, and grouping consecutive events from the same agent into runs.

import type { ReviewEvent } from '../../types';

export function eventAgent(event: ReviewEvent): string {
  switch (event.type) {
    case 'status':
    case 'thinking':
    case 'tool_call':
    case 'error':
      return event.data?.agent || 'lead';
    case 'finding':
      return event.data?.source || 'lead';
    default:
      return 'lead';
  }
}

/** Plain one-line text for an event (findings and results get richer rendering) */
export function eventText(event: ReviewEvent): string {
  switch (event.type) {
    case 'status': return event.data?.message ?? '';
    case 'thinking': return event.data?.text ?? '';
    case 'tool_call': return event.data?.summary || event.data?.tool || '';
    case 'finding': return event.data?.title ?? '';
    case 'error': return event.data?.text || 'Engine error';
    case 'result': return `Review complete · ${(event.data?.findings ?? []).length} findings`;
    case 'done': return 'Stream finished';
  }
}

/** Agents in order of first appearance */
export function activityAgents(events: readonly ReviewEvent[]): string[] {
  return [...new Set(events.map(eventAgent))];
}

export interface AgentRun {
  agent: string;
  /** seq of the first event; stable React key */
  key: number;
  events: ReviewEvent[];
}

/** Consecutive events from one agent fold into a run with a single agent header */
export function groupRuns(events: readonly ReviewEvent[], agentFilter: string | null): AgentRun[] {
  const runs: AgentRun[] = [];
  for (const event of events) {
    const agent = eventAgent(event);
    if (agentFilter && agent !== agentFilter) continue;
    const last = runs[runs.length - 1];
    if (last && last.agent === agent) last.events.push(event);
    else runs.push({ agent, key: event.seq, events: [event] });
  }
  return runs;
}

/** 24-hour "14:03:07" so the time column has a fixed width in every locale */
export function clockTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

/** Pixels from the bottom within which the feed counts as "following" new events */
export const FOLLOW_THRESHOLD = 24;

export function isNearBottom(element: { scrollTop: number; scrollHeight: number; clientHeight: number }): boolean {
  return element.scrollHeight - element.scrollTop - element.clientHeight <= FOLLOW_THRESHOLD;
}
