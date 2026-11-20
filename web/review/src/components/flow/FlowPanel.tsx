// Layered, directed call-flow diagram. Calls flow from callers to callees: top to
// bottom in the side panel, left to right when expanded (F) or wide. In 'selection'
// scope the selected node's callers (two levels) flow into it and it flows into its
// callees; in 'all' scope (L) the whole PR is shown, one flow per connected group,
// riskiest first.

import { useEffect, useRef } from 'react';
import { useReview, useReviewStore } from '../../state/StoreProvider';
import { useCommands } from '../../state/commands';
import { FlowCanvas } from './FlowCanvas';
import { FlowLegend, FlowToolbar } from './FlowChrome';
import { FlowEmpty } from './FlowEmpty';
import { flowHint } from './flowData';
import { useElementSize, useFlowZoom } from './useFlowZoom';
import { useFlowView, type FlowView } from './useFlowView';
import { requestCallersFocus } from '../function/callersRequest';
import './FlowPanel.css';

export function FlowPanel() {
  const hasGraph = useReview((state) => (state.context.graph.nodes ?? []).length > 0);
  return hasGraph ? <FlowDiagram /> : <FlowEmpty />;
}

function FlowDiagram() {
  const store = useReviewStore();
  const ui = useReview((state) => state.ui);
  const canvasRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const viewportRef = useRef<SVGGElement>(null);
  const size = useElementSize(canvasRef);
  const view = useFlowView(size);
  const fit = useFlowZoom(svgRef, viewportRef);
  const animate = useAutoFit(view, size, fit);

  useCommands({
    fitGraph: () => fit(view.layout.bounds, true),
    escape: () => {
      // Esc first closes overlays and the expanded view (useKeymap); only then clear
      if (ui.helpOpen || ui.postOpen || ui.flowExpanded) return;
      if (view.highlightId) view.clearHighlight();
    },
    gotoCallers: () => requestCallersFocus(store),
  });

  const focusFunction = useReview((state) => (view.layoutFocusId ? state.index.functions.get(view.layoutFocusId) : undefined));
  const nodeName = (id: string | null) => (id ? view.model.nodes.get(id)?.name ?? null : null);
  const hint = flowHint({
    layout: view.layout, scope: view.scope, focusName: nodeName(view.layoutFocusId), highlightName: nodeName(view.highlightId), focusFunction,
  });

  return (
    <div className={`fl-panel${ui.flowExpanded ? ' fl-expanded' : ''}`}>
      <FlowToolbar
        scope={view.scope}
        expanded={ui.flowExpanded}
        onToggleScope={() => store.getState().toggleFlowScope()}
        onFit={() => fit(view.layout.bounds, true)}
        onToggleExpanded={() => store.getState().toggleFlowExpanded()}
      />
      <div className="fl-hint" aria-live="polite">{hint}</div>
      <div className="fl-canvas" ref={canvasRef}>
        <FlowCanvas
          layout={view.layout}
          highlightId={view.highlightId}
          findings={view.findings}
          svgRef={svgRef}
          viewportRef={viewportRef}
          animate={animate}
          onSelect={(id) => store.getState().selectNode(id)}
          onBackground={() => { if (view.scope === 'all') view.clearHighlight(); }}
        />
      </div>
      <FlowLegend />
    </div>
  );
}

/**
 * Fit the diagram whenever the layout or the canvas size changes: animated for a new
 * layout, instant for a resize or the first paint. Returns whether layouts should tween.
 */
function useAutoFit(view: FlowView, size: { width: number; height: number }, fit: ReturnType<typeof useFlowZoom>): boolean {
  const painted = useRef(false);
  const lastSize = useRef(size);
  const { layout } = view;
  const ready = size.width > 0 && size.height > 0;

  useEffect(() => {
    if (!ready) return;
    const resized = lastSize.current.width !== size.width || lastSize.current.height !== size.height;
    lastSize.current = size;
    fit(layout.bounds, painted.current && !resized);
    painted.current = true;
  }, [layout, size, ready, fit]);

  return painted.current;
}
