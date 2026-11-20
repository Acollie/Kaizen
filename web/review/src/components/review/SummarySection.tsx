// Collapsible reviewer summary (untrusted Markdown, rendered safely).

import { useReview } from '../../state/StoreProvider';
import { Markdown } from '../shared/Markdown';
import { Disclosure } from './Disclosure';

export function SummarySection() {
  const summary = useReview((state) => state.result?.summary ?? '');
  if (!summary.trim()) return null;
  return (
    <Disclosure title="Reviewer summary" defaultOpen>
      <Markdown source={summary} className="rv-summary" />
    </Disclosure>
  );
}
