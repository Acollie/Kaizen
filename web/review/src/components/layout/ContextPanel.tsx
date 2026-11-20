// Right-hand context panel with tabs: Review (AI), Flow (call graph), Function, Activity.
// OWNER: shell agent for the tab bar; each tab's content is owned by its panel's agent.

import { useReview } from '../../state/StoreProvider';
import type { TabName } from '../../state/store';
import { ReviewPanel } from '../review/ReviewPanel';
import { FlowPanel } from '../flow/FlowPanel';
import { FunctionPanel } from '../function/FunctionPanel';
import { ActivityPanel } from '../activity/ActivityPanel';
import './ContextPanel.css';

const TAB_LABELS: Record<TabName, string> = { review: 'Review', graph: 'Flow', function: 'Function', activity: 'Activity' };
const TAB_KEYS: Record<TabName, string> = { review: 'g1', graph: 'g2', function: 'g3', activity: 'g4' };

export function ContextPanel() {
  const tab = useReview((state) => state.ui.tab);
  const setTab = useReview((state) => state.setTab);
  return (
    <div className="context-panel">
      <div className="tabs" role="tablist">
        {(Object.keys(TAB_LABELS) as TabName[]).map((name) => (
          <button key={name} type="button" role="tab" aria-selected={tab === name} className="tab"
            title={`${TAB_LABELS[name]} (${TAB_KEYS[name]})`} onClick={() => setTab(name)}>
            {TAB_LABELS[name]}
          </button>
        ))}
      </div>
      <div className="tab-body">
        {tab === 'review' && <ReviewPanel />}
        {tab === 'graph' && <FlowPanel />}
        {tab === 'function' && <FunctionPanel />}
        {tab === 'activity' && <ActivityPanel />}
      </div>
    </div>
  );
}
