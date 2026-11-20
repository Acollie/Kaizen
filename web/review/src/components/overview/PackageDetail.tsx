// A package expanded from the map: its changed functions (riskiest first) and files.
// Clicking a function or file opens it in the code view.

import { useState } from 'react';
import { useReview } from '../../state/StoreProvider';
import type { PackageInfo, PackageCoverage } from '../../lib/packageMap';
import { lookReasons } from '../../lib/packageMap';
import { qualifiedName, splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { SeverityCounts } from '../shared/bits';

/** Rows shown per list before "Show all" */
const LIST_LIMIT = 12;

interface PackageDetailProps {
  pkg: PackageInfo;
  aiKnown: boolean;
  onClose(): void;
}

function coverageText(coverage: PackageCoverage, aiKnown: boolean, who: 'You' | 'AI'): string {
  if (who === 'AI' && !aiKnown) return 'AI: unknown';
  const basis = coverage.functions.total > 0 ? { ...coverage.functions, noun: 'functions' } : { ...coverage.files, noun: 'files' };
  return `${who}: ${basis.seen}/${basis.total} ${basis.noun}`;
}

export function PackageDetail({ pkg, aiKnown, onClose }: PackageDetailProps) {
  const viewed = useReview((state) => state.viewed);
  const selectFunction = useReview((state) => state.selectFunction);
  const openFile = useReview((state) => state.openFile);
  const [showAll, setShowAll] = useState({ functions: false, files: false });
  const functions = showAll.functions ? pkg.functions : pkg.functions.slice(0, LIST_LIMIT);
  const files = showAll.files ? pkg.files : [...pkg.files].sort((left, right) => right.risk_score - left.risk_score).slice(0, LIST_LIMIT);
  return (
    <section className="ov-detail">
      <div className="ov-aside-head">
        <button type="button" className="btn ov-back" onClick={onClose} title="Back to Where to look (Esc)">← Where to look</button>
      </div>
      <div className="ov-detail-title mono">{pkg.label}</div>
      <div className="ov-detail-meta">
        <span>{pkg.files.length} files · {pkg.functions.length} functions</span>
        <span className="mono"><span className="ov-add">+{pkg.added}</span> <span className="ov-del">−{pkg.removed}</span></span>
        <SeverityCounts findings={pkg.findings} />
      </div>
      <div className="ov-detail-meta">
        <span className="ov-cov-you">{coverageText(pkg.you, true, 'You')}</span>
        <span className={aiKnown ? '' : 'ov-unknown'}>{coverageText(pkg.ai, aiKnown, 'AI')}</span>
      </div>
      <div className="ov-detail-reason">{lookReasons(pkg).slice(0, 2).join(' · ')}</div>

      {pkg.functions.length > 0 && (
        <>
          <div className="label ov-detail-label">Changed functions</div>
          <ul className="ov-list">
            {functions.map((fn) => (
              <li key={fn.id}>
                <button type="button" className="ov-row" onClick={() => selectFunction(fn.id)} title={`${fn.file}:${fn.start_line}`}>
                  <span className="ov-row-risk mono" style={{ color: riskColor(fn.risk_score) }}>{Math.round(fn.risk_score)}</span>
                  <span className="ov-row-name mono">{fn.focus && <span style={{ color: riskColor(fn.risk_score) }}>★ </span>}{qualifiedName(fn)}</span>
                  <span className="ov-row-file mono">{splitPath(fn.file).base}</span>
                  <span className={`ov-row-seen${viewed.functions[fn.id] ? ' on' : ''}`} title={viewed.functions[fn.id] ? 'You viewed it' : 'Not viewed yet'}>
                    {viewed.functions[fn.id] ? '✓' : ''}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          <ShowAll shown={functions.length} total={pkg.functions.length} noun="functions" onShow={() => setShowAll({ ...showAll, functions: true })} />
        </>
      )}

      <div className="label ov-detail-label">Files{pkg.files.length > LIST_LIMIT ? ' · riskiest first' : ''}</div>
      <ul className="ov-list">
        {files.map((file) => {
          const { dir, base } = splitPath(file.path.slice(pkg.key ? pkg.key.length + 1 : 0));
          return (
            <li key={file.path}>
              <button type="button" className="ov-row" onClick={() => openFile(file.path)} title={file.path}>
                <span className="ov-row-risk mono" style={{ color: riskColor(file.risk_score) }}>{Math.round(file.risk_score)}</span>
                <span className="ov-row-name mono"><span className="ov-row-dir">{dir}</span>{base}</span>
                <span className="ov-row-file mono"><span className="ov-add">+{file.additions}</span> <span className="ov-del">−{file.deletions}</span></span>
                <span className={`ov-row-seen${viewed.files[file.path] ? ' on' : ''}`} title={viewed.files[file.path] ? 'You opened it' : 'Not opened yet'}>
                  {viewed.files[file.path] ? '✓' : ''}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
      <ShowAll shown={files.length} total={pkg.files.length} noun="files" onShow={() => setShowAll({ ...showAll, files: true })} />
    </section>
  );
}

function ShowAll({ shown, total, noun, onShow }: { shown: number; total: number; noun: string; onShow(): void }) {
  if (shown >= total) return null;
  return <button type="button" className="btn ov-more" onClick={onShow}>Show all {total} {noun}</button>;
}
