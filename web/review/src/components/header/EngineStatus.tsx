// Engine status: spinner + agent + current action while running, cost when done, red on error.

import { useReview } from '../../state/StoreProvider';
import { AgentTag } from '../shared/bits';
import { engineView } from './headerModel';

export function EngineStatusView() {
  const status = useReview((state) => state.status);
  const result = useReview((state) => state.result);
  const setTab = useReview((state) => state.setTab);
  const view = engineView(status, result);
  return (
    <button type="button" className={`hd-engine hd-engine-${view.tone}`} title={view.title}
      aria-live="polite" onClick={() => setTab('activity')}>
      {view.tone === 'busy' && <span className="spinner" aria-hidden="true" />}
      {view.tone === 'done' && <span className="hd-engine-check" aria-hidden="true">✓</span>}
      {view.tone === 'error' && <span className="hd-engine-cross" aria-hidden="true">!</span>}
      {view.agent && <AgentTag agent={view.agent} />}
      <span className="hd-engine-label">{view.label}</span>
    </button>
  );
}
