// Derives the Overview's data from the store: the package map (with what you viewed and
// what the AI looked at), the one-line summary and the "Where to look" list.

import { useMemo } from 'react';
import { useFindings, useReview } from '../../state/StoreProvider';
import { useAiCoverage } from '../../state/aiCoverage';
import {
  buildPackageMap, overviewSummary, whereToLook,
  type LookTarget, type OverviewSummary, type PackageMap,
} from '../../lib/packageMap';

export interface OverviewData {
  map: PackageMap;
  summary: OverviewSummary;
  targets: LookTarget[];
  aiKnown: boolean;
}

export function useOverviewData(): OverviewData {
  const files = useReview((state) => state.context.files);
  const functions = useReview((state) => state.context.functions);
  const graph = useReview((state) => state.context.graph);
  const viewed = useReview((state) => state.viewed);
  const findings = useFindings();
  const coverage = useAiCoverage();

  return useMemo(() => {
    const source = {
      viewedFunctions: viewed.functions,
      viewedFiles: viewed.files,
      aiFunctions: coverage.examinedFunctions,
      aiFiles: coverage.examinedFiles,
      aiKnown: coverage.known,
    };
    const map = buildPackageMap({ files: files ?? [], functions: functions ?? [], graph, findings, ...source });
    return {
      map,
      summary: overviewSummary(map, { ...source, functions: functions ?? [], aiComplete: coverage.complete }),
      targets: whereToLook(map),
      aiKnown: coverage.known,
    };
  }, [files, functions, graph, viewed, findings, coverage]);
}
