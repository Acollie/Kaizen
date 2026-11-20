// Changes sidebar: filter, groups (useSidebarGroups), files with functions, risk and finding counts.

import { useState } from 'react';
import { useReview, useSidebarGroups } from '../../state/StoreProvider';
import type { SidebarGroup } from '../../lib/grouping';
import { plural } from '../../lib/format';
import { groupSeverity, isExpanded, totalFunctions } from './sidebarModel';
import { SidebarFilter } from './SidebarFilter';
import { FileRow } from './FileRow';
import { AI_SKIPPED_HINT } from './FunctionRow';
import { useAiCoverage } from '../../state/aiCoverage';
import { aiSkippedFunction } from '../../state/viewing';
import './Sidebar.css';

export function Sidebar() {
  const groups = useSidebarGroups();
  const filter = useReview((state) => state.ui.filter);
  const activeFile = useReview((state) => state.selection.file);
  const coverage = useAiCoverage();
  const anySkipped = groups.some((group) => group.files.some((entry) => entry.functions.some((fn) => aiSkippedFunction(coverage, fn.id))));
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});
  const context = { total: totalFunctions(groups), filter, activeFile };

  const toggle = (path: string, expanded: boolean) => setOverrides((current) => ({ ...current, [path]: !expanded }));

  return (
    <div className="sb">
      <SidebarFilter groups={groups} />
      <div className="sb-scroll" role="tree" aria-label="Changed files">
        {groups.length === 0 && <div className="empty">{filter ? `No files or functions match “${filter}”` : 'No changed files'}</div>}
        {groups.map((group) => (
          <section key={group.key} className="sb-group" aria-label={group.title}>
            <GroupHeader group={group} />
            {group.files.map((entry) => {
              const expanded = isExpanded(entry, overrides, context);
              return <FileRow key={entry.file.path} entry={entry} expanded={expanded} coverage={coverage} onToggle={() => toggle(entry.file.path, expanded)} />;
            })}
          </section>
        ))}
      </div>
      <SidebarLegend anySkipped={anySkipped} />
    </div>
  );
}

function SidebarLegend({ anySkipped }: { anySkipped: boolean }) {
  return (
    <div className="sb-legend">
      <span><span className="sb-viewed">✓</span> you looked</span>
      {anySkipped && <span title={AI_SKIPPED_HINT}><span className="sb-ai-skipped">◌</span> AI didn't open this</span>}
    </div>
  );
}

function GroupHeader({ group }: { group: SidebarGroup }) {
  const severity = groupSeverity(group);
  return (
    <div className="sb-group-head">
      <span className={`label sb-group-title${severity ? ` sev-${severity}` : ''}`}>{group.title}</span>
      <span className="sb-group-count mono" title={plural(group.files.length, 'file')}>{group.files.length}</span>
      {group.hint && <span className="sb-group-hint">{group.hint}</span>}
    </div>
  );
}
