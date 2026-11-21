// The reviewer's own progress at the top of the Review panel: how much they have
// triaged, what they have looked at (click to jump to the next unseen thing), the
// "Think first" switch and the triage filter.

import { useReview, useReviewStore } from '../../state/StoreProvider';
import { useLookProgress } from '../../hooks/useLookProgress';
import { filterCount, progressSentence, TRIAGE_FILTERS, type TriageFilter, type TriageProgress } from '../../state/triage';
import { lookSentence, nextUnseen } from '../../state/viewing';

const FILTER_LABELS: Record<TriageFilter, string> = {
  unreviewed: 'Unreviewed', accepted: 'Accepted', dismissed: 'Dismissed', all: 'All',
};

interface YourReviewProps {
  progress: TriageProgress;
  filter: TriageFilter;
  onFilter(filter: TriageFilter): void;
  hidden: boolean;
}

export function YourReview({ progress, filter, onFilter, hidden }: YourReviewProps) {
  return (
    <section className="rv-yours" aria-label="Your review">
      <div className="rv-yours-head">
        <span className="label">Your review</span>
        <ThinkFirstSwitch />
      </div>
      {!hidden && <div className="rv-yours-line">{progressSentence(progress)}</div>}
      {!hidden && <LookLine />}
      {!hidden && progress.total > 0 && <VerifyQueueButton />}
      {!hidden && progress.total > 0 && <TriageFilterBar progress={progress} filter={filter} onFilter={onFilter} />}
    </section>
  );
}

function LookLine() {
  const store = useReviewStore();
  const look = useLookProgress();
  const sentence = lookSentence(look);
  if (!sentence) return null;
  const target = nextUnseen(look);
  const jump = () => {
    const state = store.getState();
    if (target?.kind === 'function') state.selectFunction(target.id);
    if (target?.kind === 'file') state.openFile(target.path);
  };
  if (!target) {
    return <div className="rv-yours-line rv-look-done"><span className="rv-look-check" aria-hidden="true">✓</span>{sentence}</div>;
  }
  return (
    <button type="button" className="rv-yours-line rv-look" onClick={jump} title="Open the next one you haven't looked at">
      {sentence}<span className="rv-look-next">next unseen →</span>
    </button>
  );
}

function ThinkFirstSwitch() {
  const enabled = useReview((state) => state.ui.thinkFirst);
  const toggle = useReview((state) => state.toggleThinkFirst);
  return (
    <button type="button" role="switch" aria-checked={enabled} className={`rv-switch${enabled ? ' rv-switch-on' : ''}`}
      onClick={toggle} title="Think first (on by default): do your own pass before the AI's findings are shown (T)">
      <span className="rv-switch-track" aria-hidden="true"><span className="rv-switch-thumb" /></span>
      Think first <kbd>T</kbd>
    </button>
  );
}

function TriageFilterBar({ progress, filter, onFilter }: { progress: TriageProgress; filter: TriageFilter; onFilter(filter: TriageFilter): void }) {
  return (
    <div className="rv-filter" role="tablist" aria-label="Show findings">
      {TRIAGE_FILTERS.map((option) => (
        <button key={option} type="button" role="tab" aria-selected={filter === option}
          className={`rv-filter-btn${filter === option ? ' rv-filter-on' : ''}`} onClick={() => onFilter(option)}>
          {FILTER_LABELS[option]}<span className="mono rv-filter-count">{filterCount(progress, option)}</span>
        </button>
      ))}
    </div>
  );
}

/** Opens the verify queue (V): one finding at a time, single-key decisions */
function VerifyQueueButton() {
  const openVerify = useReview((state) => state.openVerify);
  return (
    <button type="button" className="btn btn-outline rv-verify-btn" onClick={openVerify}
      title="Verify queue: one finding at a time, with its code and how to verify it (V)">
      Verify queue <kbd>V</kbd>
    </button>
  );
}
