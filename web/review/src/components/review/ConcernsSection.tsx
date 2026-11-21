// Kaizen's static-analysis concerns (blast radius, complexity, ...), collapsed by default.

import { useReview } from '../../state/StoreProvider';
import type { AffectedItem, Concern } from '../../types';
import { splitPath } from '../../lib/format';
import { Markdown } from '../shared/Markdown';
import { Disclosure } from './Disclosure';

/** Map Kaizen concern severities onto the UI's severity colours */
function concernTone(severity: string): string {
  if (severity === 'critical' || severity === 'high') return 'critical';
  if (severity === 'warning' || severity === 'medium') return 'warning';
  return 'suggestion';
}

export function ConcernsSection() {
  const concerns = useReview((state) => state.context.concerns ?? []);
  if (concerns.length === 0) return null;
  return (
    <Disclosure title="Kaizen concerns" count={concerns.length} hint="static analysis">
      <div className="rv-concerns">
        {concerns.map((concern, index) => <ConcernItem key={`${concern.type}-${index}`} concern={concern} />)}
      </div>
    </Disclosure>
  );
}

function ConcernItem({ concern }: { concern: Concern }) {
  const items = concern.affected_items ?? [];
  return (
    <div className={`rv-concern sev-${concernTone(concern.severity)}`}>
      <div className="rv-concern-title"><span className="sev-dot" aria-hidden="true" />{concern.title}</div>
      {concern.description && <Markdown source={concern.description.split(/\n\s*\n/)[0]} className="rv-concern-body" />}
      {items.length > 0 && (
        <ul className="rv-affected">
          {items.map((item, index) => <AffectedRow key={`${item.file_path}-${item.line ?? 0}-${index}`} item={item} />)}
        </ul>
      )}
    </div>
  );
}

function AffectedRow({ item }: { item: AffectedItem }) {
  const openLocation = useReview((state) => state.openLocation);
  const openFile = useReview((state) => state.openFile);
  const open = () => (item.line ? openLocation(item.file_path, item.line) : openFile(item.file_path));
  const location = `${splitPath(item.file_path).base}${item.line ? `:${item.line}` : ''}`;
  return (
    <li onClick={open} title={item.file_path}>
      <span className="mono rv-affected-name">{item.function_name || location}</span>
      {item.function_name && <span className="mono rv-affected-loc">{location}</span>}
    </li>
  );
}
