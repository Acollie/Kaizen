// The Function tab: everything Kaizen knows about the selected changed function (risk,
// reasons, metrics, AI findings, callers and callees). A selected caller/callee node
// from the flow shows its basic info instead.

import { useCallback, useEffect, useMemo, useRef } from 'react';
import { useFindings, useReview, useReviewStore } from '../../state/StoreProvider';
import { useCommands } from '../../state/commands';
import { findingBelongsTo } from '../../lib/findings';
import type { FunctionRef, FunctionRisk } from '../../types';
import { callersCapNote, callersHeading, refAction, type RefAction, type RefDirection } from './functionData';
import { FindingList, FunctionHeader, MetricsTable, RefList, RiskReasons } from './FunctionSections';
import { NodeInfo } from './NodeInfo';
import { onCallersRequest, requestCallersFocus, takeCallersRequest } from './callersRequest';
import './FunctionPanel.css';

export function FunctionPanel() {
  const store = useReviewStore();
  const functionId = useReview((state) => state.selection.functionId);
  const nodeId = useReview((state) => state.selection.nodeId);
  const fn = useReview((state) => (functionId ? state.index.functions.get(functionId) : undefined));
  const otherNode = useReview((state) => (nodeId && !state.index.functions.has(nodeId) ? state.index.nodes.get(nodeId) : undefined));

  useCommands({ gotoCallers: () => requestCallersFocus(store) });

  if (otherNode) return <NodeInfo node={otherNode} />;
  if (fn) return <ChangedFunction key={fn.id} fn={fn} />;
  return (
    <div className="empty fn-empty">
      Select a function in the Changes list, the code or the call flow.
      <div className="fn-empty-keys"><kbd>j</kbd> <kbd>k</kbd> step through changed functions</div>
    </div>
  );
}

function ChangedFunction({ fn }: { fn: FunctionRisk }) {
  const store = useReviewStore();
  const changedIds = useReview((state) => state.index.functions);
  const allFindings = useFindings();
  const findings = useMemo(() => allFindings.filter((finding) => findingBelongsTo(finding, fn)), [allFindings, fn]);
  const callersRef = useCallersFocus();

  const run = useCallback((action: RefAction) => {
    const state = store.getState();
    if (action.type === 'selectFunction') state.selectFunction(action.id);
    else if (action.type === 'openLocation') state.openLocation(action.file, action.line, { callSite: action.callSite });
  }, [store]);
  const actionFor = (direction: RefDirection) => (ref: FunctionRef) => refAction(ref, direction, new Set(changedIds.keys()));

  return (
    <div className="fn-panel">
      <FunctionHeader fn={fn} />
      <RiskReasons reasons={fn.risk_reasons ?? []} />
      <MetricsTable fn={fn} />
      <FindingList findings={findings} onOpen={(id) => store.getState().openFinding(id)} />
      <RefList title="Callers" heading={callersHeading(fn)} note={callersCapNote(fn)} refs={fn.callers ?? []}
        actionFor={actionFor('callers')} onAction={run} sectionRef={callersRef} />
      <RefList title="Callees" refs={fn.callees ?? []} actionFor={actionFor('callees')} onAction={run} />
    </div>
  );
}

/** Scroll the callers list into view and focus its first row when g r asks for it */
function useCallersFocus() {
  const sectionRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const focus = () => {
      if (!takeCallersRequest()) return;
      const section = sectionRef.current;
      if (!section) return;
      section.scrollIntoView({ block: 'start', behavior: 'smooth' });
      section.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus({ preventScroll: true });
    };
    focus();
    return onCallersRequest(() => window.requestAnimationFrame(focus));
  }, []);
  return sectionRef;
}
