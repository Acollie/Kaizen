// One changed file: status letter, dir + base name, finding counts, risk; expandable to its functions.

import type { MouseEvent } from 'react';
import { useReview } from '../../state/StoreProvider';
import { splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { SeverityCounts } from '../shared/bits';
import { fileReason, statusLetter, type SidebarEntry } from './sidebarModel';
import { FunctionRow } from './FunctionRow';
import { useScrollIntoView } from './useScrollIntoView';

interface FileRowProps {
  entry: SidebarEntry;
  expanded: boolean;
  onToggle(): void;
}

export function FileRow({ entry, expanded, onToggle }: FileRowProps) {
  const { file, functions, findings } = entry;
  const isActiveFile = useReview((state) => state.selection.file === file.path);
  const isActiveRow = useReview((state) => state.selection.file === file.path && !state.selection.functionId);
  const reveal = useReview((state) => state.selection.reveal);
  const openFile = useReview((state) => state.openFile);
  const rowRef = useScrollIntoView<HTMLDivElement>(isActiveRow, reveal);
  const { dir, base } = splitPath(file.path);
  const reason = fileReason(entry);
  const expandable = functions.length > 0;

  const onTwistyClick = (event: MouseEvent) => {
    event.stopPropagation();
    onToggle();
  };

  return (
    <div className="sb-file" role="treeitem" aria-expanded={expandable ? expanded : undefined} aria-selected={isActiveRow}>
      <div ref={rowRef} className={`sb-row sb-file-row${isActiveRow ? ' sb-active' : ''}${isActiveFile ? ' sb-active-file' : ''}`}
        title={file.path} onClick={() => openFile(file.path)}>
        <button type="button" className={`sb-twisty${expanded ? ' sb-open' : ''}`} tabIndex={-1}
          aria-label={expanded ? 'Collapse' : 'Expand'} disabled={!expandable} onClick={onTwistyClick}>
          {expandable && <svg width="8" height="8" viewBox="0 0 8 8" aria-hidden="true"><path d="M2 1l4 3-4 3z" fill="currentColor" /></svg>}
        </button>
        <span className={`sb-status sb-status-${file.status} mono`} title={file.status}>{statusLetter(file.status)}</span>
        <span className="sb-path">
          {dir && <span className="sb-dir"><bdi>{dir}</bdi></span>}
          <span className="sb-base">{base}</span>
        </span>
        <span className="sb-counts"><SeverityCounts findings={findings} /></span>
        <span className="sb-risk mono" style={{ color: riskColor(file.risk_score) }} title={`Kaizen risk ${Math.round(file.risk_score)}`}>{Math.round(file.risk_score)}</span>
      </div>
      {reason && <div className="sb-reason" title={reason}>{reason}</div>}
      {expanded && (
        <div role="group">
          {functions.map((fn) => <FunctionRow key={fn.id} fn={fn} fileFindings={findings} />)}
        </div>
      )}
    </div>
  );
}
