// AI review: verdict headline, findings grouped by severity, reviewer summary, Kaizen focus
// and Kaizen concerns. Before the AI result arrives the headline shows live progress and
// Kaizen focus is what the reviewer reads first.

import { useFindings, useReview } from '../../state/StoreProvider';
import { Headline } from './Headline';
import { FindingSections } from './FindingSections';
import { SummarySection } from './SummarySection';
import { FocusSection } from './FocusSection';
import { ConcernsSection } from './ConcernsSection';
import { BlindSpots } from './BlindSpots';
import './ReviewPanel.css';

export function ReviewPanel() {
  const findings = useFindings();
  const hasResult = useReview((state) => state.result !== null);
  return (
    <div className="rv">
      <Headline findings={findings} />
      <FindingSections findings={findings} />
      <BlindSpots />
      {hasResult && <SummarySection />}
      <FocusSection />
      <ConcernsSection />
    </div>
  );
}
