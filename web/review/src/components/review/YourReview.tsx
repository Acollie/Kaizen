// The reviewer's own progress at the top of the Review panel: how much they have
// triaged, what they have looked at (click to jump to the next unseen thing), the
// "Think first" switch and the triage filter.

import { useReview, useReviewStore } from '../../state/StoreProvider';
import { useLookProgress } from '../../hooks/useLookProgress';
import { filterCount, progressSentence, TRIAGE_FILTERS, type TriageFilter, type TriageProgress } from '../../state/triage';
import { allFocusSeen, everythingSeen, lookSentence, nextUnseen } from '../../state/viewing';

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
      <LookLine />
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
      onClick={toggle} title="Think first: hide the AI's findings until you've looked at the code yourself (T)">
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

/** Shown instead of the findings while "Think first" hides them */
export function ThinkFirstNotice({ total, running }: { total: number; running: boolean }) {
  const reveal = useReview((state) => state.revealFindings);
  const look = useLookProgress();
  const ready = allFocusSeen(look) || (look.focus.total === 0 && everythingSeen(look));
  const message = noticeText(total, running, ready);
  return (
    <div className={`rv-think${ready ? ' rv-think-ready' : ''}`} role="status">
      <p className="rv-think-text">{message}</p>
      <button type="button" className={`btn ${ready ? 'btn-primary' : 'btn-outline'} rv-think-btn`} onClick={reveal} disabled={total === 0}>
        Reveal the AI findings
      </button>
    </div>
  );
}

function noticeText(total: number, running: boolean, ready: boolean): string {
  if (total === 0) return running ? 'The AI is still reviewing. Look at the focus functions yourself in the meantime.' : 'The AI found nothing to report.';
  if (ready) return "You've seen the focus functions. Reveal the AI's findings?";
  const count = `${total} AI ${total === 1 ? 'finding' : 'findings'} ready.`;
  return `${count} Look at the focus functions first, then reveal.`;
}
