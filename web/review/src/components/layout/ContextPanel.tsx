// Right-hand context panel with tabs: Review (AI), Flow (call graph), Function, Activity.
// OWNER: shell agent for the tab bar; each tab's content is owned by its panel's agent.

import { useEffect, useState } from 'react';
import { useFindings, useReview } from '../../state/StoreProvider';
import { TABS, type TabName } from '../../state/store';
import { ReviewPanel } from '../review/ReviewPanel';
import { FlowPanel } from '../flow/FlowPanel';
import { FunctionPanel } from '../function/FunctionPanel';
import { ActivityPanel } from '../activity/ActivityPanel';
import { FunctionCommands } from '../function/FunctionCommands';
import { Chevron } from './Divider';
import { activityBadge, reviewBadge, type TabBadge } from './tabBadges';
import './ContextPanel.css';

const TAB_LABELS: Record<TabName, string> = { review: 'Review', graph: 'Flow', function: 'Function', activity: 'Activity' };
const TAB_KEYS: Record<TabName, string> = { review: 'g1', graph: 'g2', function: 'g3', activity: 'g4' };

export function ContextPanel() {
  const tab = useReview((state) => state.ui.tab);
  const badges = useTabBadges(tab);
  return (
    <div className="context-panel">
      {/* g r works from any tab: it switches to the Function tab and shows the callers */}
      <FunctionCommands />
      <TabBar tab={tab} badges={badges} />
      <div className="tab-body" role="tabpanel" id={`cp-panel-${tab}`} aria-labelledby={`cp-tab-${tab}`}>
        {tab === 'review' && <ReviewPanel />}
        {tab === 'graph' && <FlowPanel />}
        {tab === 'function' && <FunctionPanel />}
        {tab === 'activity' && <ActivityPanel />}
      </div>
    </div>
  );
}

function TabBar({ tab, badges }: { tab: TabName; badges: Partial<Record<TabName, TabBadge | null>> }) {
  const setTab = useReview((state) => state.setTab);
  const setCollapsed = useReview((state) => state.setCollapsed);
  const flowExpanded = useReview((state) => state.ui.flowExpanded);
  const toggleFlowExpanded = useReview((state) => state.toggleFlowExpanded);
  return (
    <div className="tabs">
      <div className="cp-tablist" role="tablist" aria-label="Context">
        {TABS.map((name) => (
          <button key={name} id={`cp-tab-${name}`} type="button" role="tab" aria-selected={tab === name}
            aria-controls={`cp-panel-${name}`} className="tab" title={`${TAB_LABELS[name]} (${TAB_KEYS[name]})`}
            onClick={() => setTab(name)}>
            {TAB_LABELS[name]}
            <Badge badge={badges[name] ?? null} />
          </button>
        ))}
      </div>
      <div className="cp-actions">
        {flowExpanded
          ? <button type="button" className="btn cp-exit" onClick={toggleFlowExpanded} title="Exit full width (F or Esc)">Exit full width <kbd>Esc</kbd></button>
          : <button type="button" className="btn cp-collapse" onClick={() => setCollapsed('context', true)}
              title="Collapse panel" aria-label="Collapse context panel"><Chevron direction="right" /></button>}
      </div>
    </div>
  );
}

function Badge({ badge }: { badge: TabBadge | null }) {
  if (!badge) return null;
  return <span className={`cp-badge cp-badge-${badge.tone}`} title={badge.title}>{badge.text}</span>;
}

/** Badge per tab; Activity counts events since the tab was last open */
function useTabBadges(tab: TabName): Partial<Record<TabName, TabBadge | null>> {
  const findings = useFindings();
  const activityCount = useReview((state) => state.activity.length);
  const [seen, setSeen] = useState(0);
  useEffect(() => {
    if (tab === 'activity') setSeen(activityCount);
  }, [tab, activityCount]);
  return {
    review: reviewBadge(findings),
    activity: tab === 'activity' ? null : activityBadge(activityCount, seen),
  };
}
