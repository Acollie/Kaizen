// Derives what the flow panel shows from the store: the model, which node is
// highlighted, the layout for the current scope and direction, and findings per node.

import { useCallback, useMemo, useState } from 'react';
import { useFindings, useReview, useReviewShallow } from '../../state/StoreProvider';
import { buildFlowModel, layoutFlow, visibleNodeIds, type FlowLayout, type FlowModel } from '../../lib/flowLayout';
import type { Finding } from '../../types';
import { chooseDirection, findingsByNode, rankExtentFor } from './flowData';

export interface FlowView {
  model: FlowModel;
  layout: FlowLayout;
  scope: 'selection' | 'all';
  /** Selected node, unless the user cleared the highlight (Esc / background click) */
  highlightId: string | null;
  /** Node the layout is centred on (selection scope only) */
  layoutFocusId: string | null;
  findings: Map<string, Finding[]>;
  clearHighlight(): void;
}

export function useFlowView(size: { width: number; height: number }): FlowView {
  const graph = useReview((state) => state.context.graph);
  const functions = useReview((state) => state.context.functions);
  const scope = useReview((state) => state.ui.flowScope);
  const expanded = useReview((state) => state.ui.flowExpanded);
  const selection = useReviewShallow((state) => ({
    nodeId: state.selection.nodeId, functionId: state.selection.functionId, reveal: state.selection.reveal,
  }));
  const allFindings = useFindings();
  const model = useMemo(() => buildFlowModel(graph), [graph]);

  // Clearing is remembered per selection, so the next navigation highlights again
  const selectionKey = `${selection.reveal}:${selection.nodeId}:${selection.functionId}`;
  const [clearedKey, setClearedKey] = useState<string | null>(null);
  const selectedId = [selection.nodeId, selection.functionId].find((id): id is string => Boolean(id && model.nodes.has(id))) ?? null;
  const highlightId = clearedKey === selectionKey ? null : selectedId;
  const layoutFocusId = scope === 'selection' ? highlightId : null;

  const direction = chooseDirection(size.width, size.height, expanded);
  const aspect = size.height ? Math.round((size.width / size.height) * 4) / 4 : 1;
  const maxRankExtent = rankExtentFor(size.width);
  const layout = useMemo(
    () => layoutFlow(model, visibleNodeIds(model, layoutFocusId, { hideExternal: true }), { focusId: layoutFocusId, direction, aspect, maxRankExtent }),
    [model, layoutFocusId, direction, aspect, maxRankExtent],
  );
  const findings = useMemo(() => findingsByNode(functions ?? [], allFindings), [functions, allFindings]);
  const clearHighlight = useCallback(() => setClearedKey(selectionKey), [selectionKey]);

  return { model, layout, scope, highlightId, layoutFocusId, findings, clearHighlight };
}
