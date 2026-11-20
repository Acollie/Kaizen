// React binding for the review store: <StoreProvider store={...}> plus hooks.

import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { useStore } from 'zustand';
import { useShallow } from 'zustand/react/shallow';
import { buildSidebarGroups, type SidebarGroup } from '../lib/grouping';
import { findingsList, type ReviewState, type ReviewStore } from './store';
import type { Finding } from '../types';

const StoreContext = createContext<ReviewStore | null>(null);

export function StoreProvider({ store, children }: { store: ReviewStore; children: ReactNode }) {
  return <StoreContext.Provider value={store}>{children}</StoreContext.Provider>;
}

/** The raw store (for imperative access in event handlers: store.getState()) */
export function useReviewStore(): ReviewStore {
  const store = useContext(StoreContext);
  if (!store) throw new Error('useReviewStore must be used inside <StoreProvider>');
  return store;
}

/** Select a slice of review state. Use useReviewShallow for objects/arrays built in the selector. */
export function useReview<T>(selector: (state: ReviewState) => T): T {
  return useStore(useReviewStore(), selector);
}

export function useReviewShallow<T>(selector: (state: ReviewState) => T): T {
  return useStore(useReviewStore(), useShallow(selector));
}

/** All findings in arrival order (stable reference until findings change) */
export function useFindings(): Finding[] {
  const findings = useReview((state) => state.findings);
  const order = useReview((state) => state.findingOrder);
  return useMemo(() => findingsList({ findings, findingOrder: order }), [findings, order]);
}

/** Sidebar groups, shared by the sidebar and the j/k/J/K commands */
export function useSidebarGroups(): SidebarGroup[] {
  const findings = useFindings();
  const filter = useReview((state) => state.ui.filter);
  const files = useReview((state) => state.context.files);
  const functionsByFile = useReview((state) => state.index.functionsByFile);
  return useMemo(
    () => buildSidebarGroups({ files: files ?? [], functionsByFile, findings, filter }),
    [files, functionsByFile, findings, filter],
  );
}
