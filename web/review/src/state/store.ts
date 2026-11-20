// The review store (zustand). One source of truth for the static Kaizen context, the
// live engine stream (findings, result, status, activity), the user's selection and
// navigation history, and UI state. Components select slices with useReview(selector);
// keyboard commands and components call the actions.
//
// Selection model: `selection` says what is selected (file / function / graph node /
// finding / line). `selection.reveal` increments every time the code view should
// scroll to the selection, so re-selecting the same thing still scrolls.

import { create } from 'zustand';
import type {
  FileDiff, Finding, FunctionRisk, GraphEdge, GraphNode, ReviewContext, ReviewEvent, ReviewResult,
} from '../types';
import { isUrgent, severityRank } from '../lib/findings';
import { buildIndex, type ReviewIndex } from './indexing';

export type TabName = 'review' | 'graph' | 'function' | 'activity';
export const TABS: readonly TabName[] = ['review', 'graph', 'function', 'activity'];
export type PaneName = 'sidebar' | 'code' | 'context';
export const PANES: readonly PaneName[] = ['sidebar', 'code', 'context'];

export type EngineState = 'connecting' | 'running' | 'thinking' | 'done' | 'error';
export interface EngineStatus {
  state: EngineState;
  message: string;
  agent: string;
}

export interface Selection {
  /** File shown in the code view (a PR file, or a caller's file outside the diff) */
  file: string | null;
  functionId: string | null;
  /** Selected node in the flow diagram (a changed function or a caller/callee) */
  nodeId: string | null;
  findingId: string | null;
  /** Head-side line to reveal in the code view */
  line: number | null;
  /** True when line is a call site in a file outside the diff */
  callSite: boolean;
  /** Increments whenever the code view should scroll to the selection */
  reveal: number;
}

export interface UIState {
  tab: TabName;
  focusedPane: PaneName;
  collapsed: Record<'sidebar' | 'context', boolean>;
  /** Full-width call flow (F) */
  flowExpanded: boolean;
  /** Flow diagram shows the selected function's neighbourhood, or the whole PR (L) */
  flowScope: 'selection' | 'all';
  wrap: boolean;
  filter: string;
  helpOpen: boolean;
  postOpen: boolean;
}

export interface ReviewState {
  context: ReviewContext;
  index: ReviewIndex;

  findings: Record<string, Finding>;
  /** Arrival order of finding IDs */
  findingOrder: string[];
  /** Finding ID -> Date.now() when it first arrived (for arrival animations) */
  arrivedAt: Record<string, number>;
  /** Findings selected for posting to GitHub */
  included: Record<string, boolean>;
  result: ReviewResult | null;
  status: EngineStatus;
  activity: ReviewEvent[];

  selection: Selection;
  history: Selection[];
  ui: UIState;

  // ---- engine stream ----
  applyEvent(event: ReviewEvent): void;
  setStatus(status: Partial<EngineStatus>): void;
  setIncluded(id: string, included: boolean): void;

  // ---- navigation ----
  selectFunction(id: string, options?: { record?: boolean }): void;
  /** Select a flow node; changed functions select fully, callers open at their call site */
  selectNode(id: string): void;
  openFile(path: string, options?: { line?: number; record?: boolean }): void;
  openLocation(path: string, line: number, options?: { callSite?: boolean; record?: boolean }): void;
  openFinding(id: string, options?: { record?: boolean }): void;
  goBack(): void;

  // ---- UI ----
  setTab(tab: TabName): void;
  cycleTab(delta: number): void;
  setFocusedPane(pane: PaneName): void;
  moveFocus(delta: number): void;
  setCollapsed(pane: 'sidebar' | 'context', collapsed: boolean): void;
  toggleFlowExpanded(): void;
  toggleFlowScope(): void;
  toggleWrap(): void;
  setFilter(filter: string): void;
  setHelpOpen(open: boolean): void;
  setPostOpen(open: boolean): void;
}

const EMPTY_SELECTION: Selection = {
  file: null, functionId: null, nodeId: null, findingId: null, line: null, callSite: false, reveal: 0,
};
const HISTORY_LIMIT = 100;

/** Fill null slices from Go (nil slices marshal as null) so components never null-check */
export function normalizeContext(raw: ReviewContext): ReviewContext {
  const functions = (raw.functions ?? []).map((fn) => ({
    ...fn,
    callers: fn.callers ?? [],
    callees: fn.callees ?? [],
    risk_reasons: fn.risk_reasons ?? [],
  }));
  const files = (raw.files ?? []).map((file) => ({ ...file, hunks: file.hunks ?? [], risk_reasons: file.risk_reasons ?? [] }));
  const nodes: GraphNode[] = raw.graph?.nodes ?? [];
  const edges: GraphEdge[] = raw.graph?.edges ?? [];
  return { ...raw, files, functions, graph: { nodes, edges }, concerns: raw.concerns ?? [] };
}

export function createReviewStore(rawContext: ReviewContext) {
  const context = normalizeContext(rawContext);
  const index = buildIndex(context);

  return create<ReviewState>()((set, get) => {
    /** Push the current selection onto the history before navigating away */
    const remember = () => {
      const { selection, history } = get();
      if (!selection.file) return;
      const next = [...history, selection];
      set({ history: next.length > HISTORY_LIMIT ? next.slice(-HISTORY_LIMIT) : next });
    };

    const navigate = (selection: Omit<Selection, 'reveal'>, record: boolean) => {
      if (record) remember();
      set((state) => ({ selection: { ...selection, reveal: state.selection.reveal + 1 } }));
    };

    const addFinding = (finding: Finding) => {
      if (!finding || !finding.id) return;
      set((state) => {
        const isNew = !(finding.id in state.findings);
        return {
          findings: { ...state.findings, [finding.id]: finding },
          findingOrder: isNew ? [...state.findingOrder, finding.id] : state.findingOrder,
          arrivedAt: isNew ? { ...state.arrivedAt, [finding.id]: Date.now() } : state.arrivedAt,
          included: isNew && finding.severity !== 'praise' ? { ...state.included, [finding.id]: true } : state.included,
        };
      });
    };

    return {
      context,
      index,
      findings: {},
      findingOrder: [],
      arrivedAt: {},
      included: {},
      result: null,
      status: { state: 'connecting', message: 'Connecting', agent: '' },
      activity: [],
      selection: EMPTY_SELECTION,
      history: [],
      ui: {
        tab: 'review',
        focusedPane: 'code',
        collapsed: { sidebar: false, context: false },
        flowExpanded: false,
        flowScope: 'selection',
        wrap: false,
        filter: '',
        helpOpen: false,
        postOpen: false,
      },

      applyEvent(event) {
        set((state) => ({ activity: [...state.activity, event] }));
        const { setStatus } = get();
        switch (event.type) {
          case 'status':
            setStatus({ state: 'running', message: event.data.message, agent: event.data.agent ?? '' });
            break;
          case 'thinking':
            setStatus({ state: 'thinking', message: 'thinking', agent: event.data.agent ?? '' });
            break;
          case 'tool_call':
            setStatus({ state: 'running', message: event.data.summary || event.data.tool, agent: event.data.agent ?? '' });
            break;
          case 'finding':
            addFinding(event.data);
            break;
          case 'result':
            for (const finding of event.data.findings ?? []) addFinding(finding);
            set({ result: event.data });
            setStatus({ state: 'done', message: 'Review complete', agent: '' });
            break;
          case 'error':
            setStatus({ state: 'error', message: event.data.text, agent: '' });
            break;
          case 'done':
            if (get().status.state !== 'error') setStatus({ state: 'done', message: get().result ? 'Review complete' : 'Finished', agent: '' });
            break;
        }
      },

      setStatus(status) {
        set((state) => ({ status: { ...state.status, ...status } }));
      },

      setIncluded(id, included) {
        set((state) => ({ included: { ...state.included, [id]: included } }));
      },

      selectFunction(id, { record = true } = {}) {
        const fn = index.functions.get(id);
        if (!fn) return;
        navigate({ file: fn.file, functionId: id, nodeId: id, findingId: null, line: fn.start_line, callSite: false }, record);
      },

      selectNode(id) {
        if (index.functions.has(id)) {
          get().selectFunction(id);
          return;
        }
        const node = index.nodes.get(id);
        if (!node) return;
        const site = callSiteFor(context, index, id, get().selection.functionId);
        const file = site ? site.file : node.file;
        if (!file) {
          set((state) => ({ selection: { ...state.selection, nodeId: id } }));
          return;
        }
        navigate({
          file, functionId: get().selection.functionId, nodeId: id, findingId: null,
          line: site ? site.line : node.line, callSite: Boolean(site),
        }, true);
      },

      openFile(path, { line, record = true } = {}) {
        const first = (index.functionsByFile.get(path) ?? [])[0];
        navigate({ file: path, functionId: null, nodeId: null, findingId: null, line: line ?? first?.start_line ?? null, callSite: false }, record);
      },

      openLocation(path, line, { callSite = false, record = true } = {}) {
        const fn = callSite ? null : functionAtLine(index, path, line);
        navigate({ file: path, functionId: fn?.id ?? null, nodeId: fn?.id ?? get().selection.nodeId, findingId: null, line, callSite }, record);
      },

      openFinding(id, { record = true } = {}) {
        const finding = get().findings[id];
        if (!finding) return;
        const fn = (finding.function_id && index.functions.get(finding.function_id))
          || (finding.line ? functionAtLine(index, finding.file, finding.line) : null);
        navigate({
          file: finding.file, functionId: fn ? fn.id : null, nodeId: fn ? fn.id : null,
          findingId: id, line: finding.line || null, callSite: false,
        }, record);
      },

      goBack() {
        const { history } = get();
        const previous = history[history.length - 1];
        if (!previous) return;
        set((state) => ({
          history: history.slice(0, -1),
          selection: { ...previous, reveal: state.selection.reveal + 1 },
        }));
      },

      setTab(tab) {
        set((state) => ({ ui: { ...state.ui, tab, collapsed: { ...state.ui.collapsed, context: false } } }));
      },
      cycleTab(delta) {
        const current = TABS.indexOf(get().ui.tab);
        const next = TABS[(((current + delta) % TABS.length) + TABS.length) % TABS.length] ?? 'review';
        get().setTab(next);
      },
      setFocusedPane(pane) {
        set((state) => ({
          ui: {
            ...state.ui,
            focusedPane: pane,
            collapsed: pane === 'code' ? state.ui.collapsed : { ...state.ui.collapsed, [pane]: false },
          },
        }));
      },
      moveFocus(delta) {
        const current = PANES.indexOf(get().ui.focusedPane);
        const next = PANES[Math.max(0, Math.min(PANES.length - 1, current + delta))] ?? 'code';
        get().setFocusedPane(next);
      },
      setCollapsed(pane, collapsed) {
        set((state) => ({
          ui: {
            ...state.ui,
            collapsed: { ...state.ui.collapsed, [pane]: collapsed },
            focusedPane: collapsed && state.ui.focusedPane === pane ? 'code' : state.ui.focusedPane,
          },
        }));
      },
      toggleFlowExpanded() {
        set((state) => ({ ui: { ...state.ui, flowExpanded: !state.ui.flowExpanded, tab: 'graph' } }));
      },
      toggleFlowScope() {
        set((state) => ({ ui: { ...state.ui, flowScope: state.ui.flowScope === 'all' ? 'selection' : 'all', tab: 'graph' } }));
      },
      toggleWrap() {
        set((state) => ({ ui: { ...state.ui, wrap: !state.ui.wrap } }));
      },
      setFilter(filter) {
        set((state) => ({ ui: { ...state.ui, filter } }));
      },
      setHelpOpen(open) {
        set((state) => ({ ui: { ...state.ui, helpOpen: open } }));
      },
      setPostOpen(open) {
        set((state) => ({ ui: { ...state.ui, postOpen: open } }));
      },
    };
  });
}

export type ReviewStore = ReturnType<typeof createReviewStore>;

// ---------- Derived data (pure; call with state from useReview) ----------

export function findingsList(state: Pick<ReviewState, 'findings' | 'findingOrder'>): Finding[] {
  return state.findingOrder.map((id) => state.findings[id]).filter((finding): finding is Finding => Boolean(finding));
}

export function findingsForFile(state: Pick<ReviewState, 'findings' | 'findingOrder'>, path: string): Finding[] {
  return findingsList(state).filter((finding) => finding.file === path);
}

export function functionAtLine(index: ReviewIndex, path: string, line: number): FunctionRisk | null {
  return (index.functionsByFile.get(path) ?? []).find((fn) => line >= fn.start_line && line <= fn.end_line) ?? null;
}

/** For a caller node, its call site into the selected (or any) changed function */
export function callSiteFor(context: ReviewContext, index: ReviewIndex, nodeId: string, selectedFunctionId: string | null) {
  const functions = context.functions ?? [];
  const ordered = [...functions].sort((left, right) => Number(right.id === selectedFunctionId) - Number(left.id === selectedFunctionId));
  for (const fn of ordered) {
    const ref = (fn.callers ?? []).find((caller) => caller.id === nodeId && caller.file);
    if (ref) return ref;
  }
  void index;
  return null;
}

/** Findings ordered the way n/N walk them: urgent first, then by sidebar file order and line */
export function orderedFindings(findings: Finding[], fileOrder: readonly FileDiff[]): Finding[] {
  const position = new Map(fileOrder.map((file, order) => [file.path, order]));
  return [...findings].sort((left, right) =>
    Number(isUrgent(right)) - Number(isUrgent(left))
    || (position.get(left.file) ?? 999) - (position.get(right.file) ?? 999)
    || (left.line ?? 0) - (right.line ?? 0)
    || severityRank(left.severity) - severityRank(right.severity));
}
