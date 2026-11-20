// One activity event, rendered by type. All text is untrusted and rendered as React text.

import { useState } from 'react';
import { useReview } from '../../state/StoreProvider';
import type { ReviewEvent } from '../../types';
import { formatClock } from '../../lib/format';
import { SEVERITY_LABEL } from '../../lib/findings';
import { verdictView } from '../review/reviewModel';
import { clockTime, eventText } from './activityModel';

export function ActivityItem({ event }: { event: ReviewEvent }) {
  return (
    <div className={`ac-item ac-kind-${event.type}`}>
      <time className="ac-time mono" dateTime={event.time} title={formatClock(event.time)}>{clockTime(event.time)}</time>
      <div className="ac-body"><ActivityBody event={event} /></div>
    </div>
  );
}

function ActivityBody({ event }: { event: ReviewEvent }) {
  switch (event.type) {
    case 'tool_call': return <ToolCall summary={eventText(event)} />;
    case 'thinking': return <Thinking text={eventText(event)} />;
    case 'finding': return <FindingLine event={event} />;
    case 'result': {
      const verdict = verdictView(event.data?.verdict ?? 'comment');
      return <span className="ac-result"><span className={`sev-dot sev-${verdict.tone}`} aria-hidden="true" />{verdict.label}<span className="ac-dim"> · {eventText(event)}</span></span>;
    }
    default: return <span className="ac-text">{eventText(event)}</span>;
  }
}

/** "get_callers(pkg.Func)": tool name brighter than its arguments */
function ToolCall({ summary }: { summary: string }) {
  const open = summary.indexOf('(');
  const name = open > 0 ? summary.slice(0, open) : summary;
  const args = open > 0 ? summary.slice(open) : '';
  return <span className="ac-tool mono" title={summary}><span className="ac-tool-name">{name}</span>{args}</span>;
}

function Thinking({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <button type="button" className={`ac-thinking${open ? ' ac-open' : ''}`} aria-expanded={open} onClick={() => setOpen(!open)}>
      <span className="ac-thinking-label">thinking</span>
      <span className="ac-thinking-text">{text}</span>
    </button>
  );
}

function FindingLine({ event }: { event: Extract<ReviewEvent, { type: 'finding' }> }) {
  const openFinding = useReview((state) => state.openFinding);
  const finding = event.data;
  return (
    <button type="button" className={`ac-finding sev-${finding.severity}`} onClick={() => openFinding(finding.id)}>
      <span className="sev-dot" aria-hidden="true" />
      <span className="ac-finding-sev">{SEVERITY_LABEL[finding.severity] ?? finding.severity}</span>
      <span className="ac-finding-title">{finding.title}</span>
    </button>
  );
}
