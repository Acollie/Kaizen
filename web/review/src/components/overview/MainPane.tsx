// The main pane: a quiet "Overview · Code" toggle (o) over either the package map or
// the code view. The Overview is the landing view; opening any code switches to Code.

import { useReview } from '../../state/StoreProvider';
import type { MainView } from '../../state/store';
import { CodeView } from '../code/CodeView';
import { Overview } from './Overview';
import './MainPane.css';

const VIEWS: readonly { view: MainView; label: string }[] = [
  { view: 'overview', label: 'Overview' },
  { view: 'code', label: 'Code' },
];

export function MainPane() {
  const mainView = useReview((state) => state.ui.mainView);
  return (
    <div className="mp">
      <MainViewToggle current={mainView} />
      <div className="mp-body">{mainView === 'overview' ? <Overview /> : <CodeView />}</div>
    </div>
  );
}

function MainViewToggle({ current }: { current: MainView }) {
  const setMainView = useReview((state) => state.setMainView);
  return (
    <div className="mp-toggle" role="tablist" aria-label="Main view">
      {VIEWS.map(({ view, label }, order) => (
        <span key={view} className="mp-toggle-item">
          {order > 0 && <span className="mp-sep" aria-hidden="true">·</span>}
          <button
            type="button" role="tab" aria-selected={current === view}
            className={`mp-view${current === view ? ' mp-view-on' : ''}`}
            title="Toggle overview / code (o)" onClick={() => setMainView(view)}
          >
            {label}
          </button>
        </span>
      ))}
      <kbd className="mp-key" title="Toggle overview / code">o</kbd>
    </div>
  );
}
