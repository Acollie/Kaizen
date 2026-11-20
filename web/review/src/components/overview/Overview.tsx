// The Overview: what this PR touches (packages), how it fits together (calls between
// packages) and where the reviewer's attention should go. It directs attention and
// shows coverage honestly; it does not summarise the AI's verdict.

import { useReview } from '../../state/StoreProvider';
import { useCommands } from '../../state/commands';
import { summaryParts } from '../../lib/packageMap';
import { OverviewMap } from './OverviewMap';
import { PackageDetail } from './PackageDetail';
import { WhereToLook } from './WhereToLook';
import { useOverviewData } from './useOverviewData';
import './Overview.css';

export function Overview() {
  const data = useOverviewData();
  const selectedKey = useReview((state) => state.ui.overviewPackage);
  const setOverviewPackage = useReview((state) => state.setOverviewPackage);
  const selected = data.map.packages.find((pkg) => pkg.key === selectedKey) ?? null;

  useCommands({
    escape: () => { if (selected) setOverviewPackage(null); },
  });

  if (!data.map.packages.length) {
    return <div className="ov"><div className="empty">This PR changes no files.</div></div>;
  }

  return (
    <div className="ov">
      <Summary parts={summaryParts(data.summary)} />
      <div className="ov-body">
        <OverviewMap map={data.map} selectedKey={selected?.key ?? null} onSelect={setOverviewPackage} />
        <aside className="ov-aside" aria-label={selected ? `Package ${selected.label}` : 'Where to look'}>
          {selected
            ? <PackageDetail key={selected.key} pkg={selected} aiKnown={data.aiKnown} onClose={() => setOverviewPackage(null)} />
            : <WhereToLook targets={data.targets} aiKnown={data.aiKnown} onOpen={setOverviewPackage} />}
        </aside>
      </div>
    </div>
  );
}

function Summary({ parts }: { parts: string[] }) {
  const [packages, files, functions, you, ai] = parts;
  return (
    <div className="ov-summary" aria-live="polite">
      <span className="ov-sum-counts">{[packages, files, functions].join(' · ')}</span>
      <span className="ov-sum-sep" aria-hidden="true">·</span>
      <span className="ov-sum-you">{you}</span>
      <span className="ov-sum-sep" aria-hidden="true">·</span>
      <span className={`ov-sum-ai${ai?.includes('unknown') ? ' ov-unknown' : ''}`}>{ai}</span>
    </div>
  );
}
