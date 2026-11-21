// The comparison between the reviewer's own pass and the AI's findings, available once
// a Think-first review has been revealed (null otherwise).

import { useMemo } from 'react';
import { useAllFindings, useComments, useReview } from '../state/StoreProvider';
import { compareView, type CompareView } from '../state/compare';
import { isUserComment } from '../state/comments';

export function useCompare(): CompareView | null {
  const reveal = useReview((state) => state.reveal);
  const shown = useReview((state) => state.ui.thinkFirst && state.revealed);
  const functions = useReview((state) => state.index.functions);
  const comments = useComments();
  const findings = useAllFindings();
  return useMemo(() => {
    if (!shown || !reveal) return null;
    const aiFindings = findings.filter((finding) => !isUserComment(finding));
    return compareView({ reveal, comments, findings: aiFindings, fileOfFunction: (id) => functions.get(id)?.file });
  }, [shown, reveal, comments, findings, functions]);
}

/** "overlap" / "outside" for one AI finding, or null */
export function useCompareTag(id: string): 'overlap' | 'outside' | null {
  const view = useCompare();
  if (!view) return null;
  if (view.overlapIds.has(id)) return 'overlap';
  if (view.outsideIds.has(id)) return 'outside';
  return null;
}
