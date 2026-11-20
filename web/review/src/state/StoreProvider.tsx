// React binding for the review store: <StoreProvider store={...}> plus hooks.

import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { useStore } from 'zustand';
import { useShallow } from 'zustand/react/shallow';
import { buildSidebarGroups, type SidebarGroup } from '../lib/grouping';
import { findingsHidden, findingsList, type ReviewState, type ReviewStore } from './store';
import { commentsList, commentToFinding, type ReviewComment } from './comments';
import type { Finding } from '../types';

const NO_FINDINGS: Finding[] = [];

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

/** Every AI finding in arrival order, even while "Think first" hides them (counts, posting) */
export function useAllFindings(): Finding[] {
  const findings = useReview((state) => state.findings);
  const order = useReview((state) => state.findingOrder);
  return useMemo(() => findingsList({ findings, findingOrder: order }), [findings, order]);
}

/** The AI findings the reviewer can see: none while "Think first" hides them */
export function useFindings(): Finding[] {
  const all = useAllFindings();
  const hidden = useReview(findingsHidden);
  return hidden ? NO_FINDINGS : all;
}

/** The reviewer's own comments, oldest first */
export function useComments(): ReviewComment[] {
  const comments = useReview((state) => state.comments);
  return useMemo(() => commentsList(comments), [comments]);
}

/** The reviewer's comments as findings (for placement, n/N and posting) */
export function useCommentFindings(): Finding[] {
  const comments = useComments();
  return useMemo(() => comments.map(commentToFinding), [comments]);
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
