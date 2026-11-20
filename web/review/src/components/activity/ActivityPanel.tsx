// Live agent activity feed: status, tool calls, thinking, findings, errors and the result,
// grouped into runs per agent, with an agent filter and follow-the-tail scrolling.

import { useMemo, useState } from 'react';
import { useReview } from '../../state/StoreProvider';
import { findingsHidden } from '../../state/store';
import { AgentTag } from '../shared/bits';
import { activityAgents, groupRuns, type AgentRun } from './activityModel';
import { ActivityItem } from './ActivityItem';
import { useStickToBottom } from './useStickToBottom';
import './ActivityPanel.css';

export function ActivityPanel() {
  const allActivity = useReview((state) => state.activity);
  const hidden = useReview(findingsHidden);
  // "Think first" hides the AI's conclusions here too; its tool calls stay visible
  const activity = useMemo(
    () => (hidden ? allActivity.filter((event) => event.type !== 'finding' && event.type !== 'result' && event.type !== 'thinking') : allActivity),
    [allActivity, hidden],
  );
  const busy = useReview((state) => state.status.state !== 'done' && state.status.state !== 'error');
  const [agentFilter, setAgentFilter] = useState<string | null>(null);
  const agents = useMemo(() => activityAgents(activity), [activity]);
  const runs = useMemo(() => groupRuns(activity, agentFilter), [activity, agentFilter]);
  const visibleCount = runs.reduce((sum, run) => sum + run.events.length, 0);
  const feed = useStickToBottom<HTMLDivElement>(visibleCount);

  return (
    <div className="ac">
      {agents.length > 1 && <AgentFilter agents={agents} active={agentFilter} onChange={setAgentFilter} />}
      <div ref={feed.ref} className="ac-feed" onScroll={feed.onScroll} role="log" aria-label="Review activity">
        {runs.length === 0 && <div className="empty">{busy ? 'Waiting for the engine…' : 'No activity'}</div>}
        {runs.map((run) => <Run key={run.key} run={run} />)}
        {busy && runs.length > 0 && <div className="ac-tail"><span className="spinner" aria-hidden="true" /></div>}
      </div>
      {feed.unseen && (
        <button type="button" className="ac-jump" onClick={feed.scrollToBottom}>↓ New activity</button>
      )}
    </div>
  );
}

function Run({ run }: { run: AgentRun }) {
  return (
    <section className="ac-run">
      <div className="ac-run-head"><AgentTag agent={run.agent} /></div>
      {run.events.map((event) => <ActivityItem key={event.seq} event={event} />)}
    </section>
  );
}

function AgentFilter({ agents, active, onChange }: { agents: string[]; active: string | null; onChange(agent: string | null): void }) {
  return (
    <div className="ac-filter" role="group" aria-label="Filter by agent">
      <button type="button" className="ac-filter-chip" aria-pressed={active === null} onClick={() => onChange(null)}>All</button>
      {agents.map((agent) => (
        <button key={agent} type="button" className="ac-filter-chip" aria-pressed={active === agent}
          onClick={() => onChange(active === agent ? null : agent)}>
          <AgentTag agent={agent} />
        </button>
      ))}
    </div>
  );
}
