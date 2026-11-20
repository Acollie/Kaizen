// What the reviewer has looked at, against what Kaizen and the AI say needs a look.

import { useMemo } from 'react';
import { useFindings, useReview } from '../state/StoreProvider';
import { attentionFiles, focusFunctions, lookProgress, type LookProgress } from '../state/viewing';

export function useLookProgress(): LookProgress {
  const functions = useReview((state) => state.context.functions);
  const files = useReview((state) => state.context.files);
  const functionsByFile = useReview((state) => state.index.functionsByFile);
  const viewed = useReview((state) => state.viewed);
  const triage = useReview((state) => state.triage);
  const findings = useFindings();
  return useMemo(() => {
    const focus = focusFunctions(functions ?? []);
    const attention = attentionFiles({ files: files ?? [], functionsByFile, findings, triage });
    return lookProgress(focus, attention, viewed);
  }, [functions, files, functionsByFile, findings, triage, viewed]);
}
