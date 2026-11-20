// One group of findings in the post dialog ("Inline comments" / "Review body"), each
// with an include checkbox bound to the store's `included` map.

import { useReview } from '../../state/StoreProvider';
import type { Finding } from '../../types';
import { formatLineRef, SEVERITY_LABEL } from '../../lib/findings';
import { splitPath } from '../../lib/format';
import { SeverityDot } from '../shared/bits';

interface PostFindingGroupProps {
  title: string;
  hint: string;
  findings: Finding[];
}

export function PostFindingGroup({ title, hint, findings }: PostFindingGroupProps) {
  const included = useReview((state) => state.included);
  const setIncluded = useReview((state) => state.setIncluded);
  if (findings.length === 0) return null;
  const allIncluded = findings.every((finding) => included[finding.id]);
  const toggleAll = () => findings.forEach((finding) => setIncluded(finding.id, !allIncluded));

  return (
    <section className="pm-group">
      <div className="pm-group-head">
        <span className="label">{title}</span>
        <span className="pm-group-count mono">{findings.length}</span>
        <span className="pm-group-hint">{hint}</span>
        <button type="button" className="btn pm-toggle-all" onClick={toggleAll}>{allIncluded ? 'None' : 'All'}</button>
      </div>
      <ul className="pm-list">
        {findings.map((finding) => (
          <PostFindingRow key={finding.id} finding={finding} checked={Boolean(included[finding.id])}
            onChange={(checked) => setIncluded(finding.id, checked)} />
        ))}
      </ul>
    </section>
  );
}

function PostFindingRow({ finding, checked, onChange }: { finding: Finding; checked: boolean; onChange(checked: boolean): void }) {
  const { base } = splitPath(finding.file);
  return (
    <li>
      <label className={`pm-item${checked ? '' : ' excluded'}`}>
        <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)}
          aria-label={`Include ${finding.title}`} />
        <SeverityDot severity={finding.severity} />
        <span className="pm-item-title" title={finding.title}>{finding.title}</span>
        <span className="pm-item-sev">{SEVERITY_LABEL[finding.severity]}</span>
        <span className="pm-item-loc mono" title={finding.file}>{base}{finding.line ? ` ${formatLineRef(finding)}` : ''}</span>
      </label>
    </li>
  );
}
