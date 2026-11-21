// Filter input. "/" focuses it (focusFilter command); Enter opens the first match and blurs.

import { useRef, type KeyboardEvent } from 'react';
import { useReview, useReviewStore } from '../../state/StoreProvider';
import { useCommands } from '../../state/commands';
import type { SidebarGroup } from '../../lib/grouping';
import { firstFilterMatch } from './sidebarModel';

export function SidebarFilter({ groups }: { groups: readonly SidebarGroup[] }) {
  const store = useReviewStore();
  const filter = useReview((state) => state.ui.filter);
  const setFilter = useReview((state) => state.setFilter);
  const inputRef = useRef<HTMLInputElement>(null);

  useCommands({
    focusFilter: () => {
      store.getState().setFocusedPane('sidebar');
      // The pane may be expanding from collapsed; focus after it renders
      window.requestAnimationFrame(() => {
        inputRef.current?.focus();
        inputRef.current?.select();
      });
    },
  });

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    const target = firstFilterMatch(groups, filter);
    const state = store.getState();
    if (target?.kind === 'file') state.openFile(target.path);
    if (target?.kind === 'function') state.selectFunction(target.id);
    inputRef.current?.blur();
  };

  return (
    <div className="sb-filter">
      <svg className="sb-filter-icon" width="13" height="13" viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="7" cy="7" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
        <path d="M10.5 10.5 14 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <input ref={inputRef} type="search" className="sb-filter-input" value={filter} spellCheck={false}
        placeholder="Filter files and functions" aria-label="Filter files and functions"
        onChange={(event) => setFilter(event.target.value)} onKeyDown={onKeyDown} />
      {filter
        ? <button type="button" className="sb-filter-clear" aria-label="Clear filter" onClick={() => setFilter('')}>×</button>
        : <kbd aria-hidden="true">/</kbd>}
    </div>
  );
}
