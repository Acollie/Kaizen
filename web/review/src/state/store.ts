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
import { cleanNote, type TriageEntry, type TriageMap } from './triage';
import { EMPTY_VIEWED, withViewed, type ViewedState, type ViewKind } from './viewing';
import {
  commentToFinding, newCommentId, normalizeRange,
  type CommentInput, type CommentMap, type CommentTarget, type ReviewComment,
} from './comments';
import {
  hasWork, loadReview, loadThinkFirst, parseReview, reviewStorageKey, saveReview, saveThinkFirst, serializeReview,
  type KeyValueStorage,
} from './persistence';
import type { StateSync } from './stateSync';
import { captureReveal, type RevealMoment } from './compare';
import {
  advanceSession, mergeOrder, moveSession, startSession, verifyOrder, visibleIds, type VerifySession,
} from './verifyQueue';

export type TabName = 'review' | 'graph' | 'function' | 'activity';
export const TABS: readonly TabName[] = ['review', 'graph', 'function', 'activity'];
export type PaneName = 'sidebar' | 'code' | 'context';
export const PANES: readonly PaneName[] = ['sidebar', 'code', 'context'];

/** The main pane: the package map (landing view) or the code */
export type MainView = 'overview' | 'code';

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
  /** Main pane: the Overview (default on load) or the code view. Opening code switches to 'code'. */
  mainView: MainView;
  /** Package expanded in the Overview, if any */
  overviewPackage: string | null;
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
  /** "Think first": hide the AI's findings until the reviewer reveals them (T) */
  thinkFirst: boolean;
  /** Finding whose dismiss-reason picker is open, and where (code card or Review panel) */
  dismissing: { id: string; surface: DismissSurface } | null;
  /** Open comment composer (new comment at a line/file, or editing one) */
  composer: CommentTarget | null;
  /** Line where `c` was pressed on a removed line: shows why it can't take a comment */
  blockedCommentKey: string | null;
  /** The verify queue (V), when open */
  verify: VerifySession | null;
}

export type DismissSurface = 'code' | 'panel';

export interface ReviewState {
  context: ReviewContext;
  index: ReviewIndex;

  findings: Record<string, Finding>;
  /** Arrival order of finding IDs */
  findingOrder: string[];
  /**
   * AI findings the lead reviewer's final result left out. They are kept out of every
   * active list (counts, sidebar, Review sections, inline cards, n/N, posting) and shown
   * only in the "Withdrawn by the lead reviewer" section, so nothing silently disappears.
   */
  withdrawn: Record<string, Finding>;
  withdrawnOrder: string[];
  /** Finding ID -> Date.now() when it first arrived (for arrival animations) */
  arrivedAt: Record<string, number>;
  /** The reviewer's decision per finding; absent = unreviewed. Accepted findings are posted. */
  triage: TriageMap;
  /** Changed functions and files the reviewer has actually looked at */
  viewed: ViewedState;
  /** The reviewer's own comments (always posted; they wrote them) */
  comments: CommentMap;
  /** Findings revealed in "Think first" mode */
  revealed: boolean;
  /** When the findings were first revealed and what had been viewed by then (persisted) */
  reveal: RevealMoment | null;
  result: ReviewResult | null;
  status: EngineStatus;
  activity: ReviewEvent[];

  selection: Selection;
  history: Selection[];
  ui: UIState;

  // ---- engine stream ----
  applyEvent(event: ReviewEvent): void;
  setStatus(status: Partial<EngineStatus>): void;

  // ---- the human's review ----
  acceptFinding(id: string, note?: string): void;
  dismissFinding(id: string, reason: string, note?: string): void;
  /** Back to unreviewed */
  undoTriage(id: string): void;
  setTriageNote(id: string, note: string): void;
  toggleAccept(id: string): void;
  startDismiss(id: string, surface: DismissSurface): void;
  cancelDismiss(): void;
  markViewed(kind: ViewKind, id: string): void;
  setThinkFirst(enabled: boolean): void;
  toggleThinkFirst(): void;
  revealFindings(): void;

  // ---- verify queue (V) ----
  openVerify(): void;
  closeVerify(): void;
  verifyMove(delta: number): void;
  verifyAccept(): void;
  verifyStartDismiss(): void;
  verifyCancelDismiss(): void;
  verifyDismiss(reason: string): void;
  verifyToggleNits(): void;
  verifyToggleRelations(): void;
  openComposer(target: CommentTarget): void;
  closeComposer(): void;
  /** Saves a new comment (or the one being edited); returns its id */
  saveComment(input: CommentInput, editingId?: string): string;
  deleteComment(id: string): void;
  setBlockedComment(rowKey: string | null): void;

  // ---- navigation ----
  // Every navigation that opens code switches the main pane to 'code'. showCode: false
  // (used only for the initial selection on load) selects without leaving the Overview.
  selectFunction(id: string, options?: NavigateOptions): void;
  /** Select a flow node; changed functions select fully, callers open at their call site */
  selectNode(id: string): void;
  openFile(path: string, options?: NavigateOptions & { line?: number }): void;
  openLocation(path: string, line: number, options?: NavigateOptions & { callSite?: boolean }): void;
  openFinding(id: string, options?: NavigateOptions): void;
  goBack(): void;

  // ---- UI ----
  setMainView(view: MainView): void;
  toggleMainView(): void;
  setOverviewPackage(key: string | null): void;
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

export interface NavigateOptions {
  /** Push the current selection onto the history (default true) */
  record?: boolean;
  /** Switch the main pane to the code view (default true) */
  showCode?: boolean;
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

    const navigate = (selection: Omit<Selection, 'reveal'>, record: boolean, showCode = true) => {
      if (record) remember();
      set((state) => ({
        selection: { ...selection, reveal: state.selection.reveal + 1 },
        ui: showCode && state.ui.mainView !== 'code' ? { ...state.ui, mainView: 'code' } : state.ui,
      }));
    };

    const setTriage = (id: string, entry: TriageEntry) => {
      const clean: TriageEntry = { state: entry.state };
      if (entry.reason) clean.reason = entry.reason;
      if (entry.note) clean.note = entry.note;
      set((state) => ({ triage: { ...state.triage, [id]: clean }, ui: closeDismissFor(state.ui, id) }));
    };

    const addFinding = (finding: Finding) => {
      if (!finding || !finding.id) return;
      set((state) => {
        const isNew = !(finding.id in state.findings);
        return {
          findings: { ...state.findings, [finding.id]: finding },
          findingOrder: isNew ? [...state.findingOrder, finding.id] : state.findingOrder,
          arrivedAt: isNew ? { ...state.arrivedAt, [finding.id]: Date.now() } : state.arrivedAt,
        };
      });
    };

    /** Apply a transition to the open verify queue, given the IDs it currently walks */
    const updateVerify = (transition: (session: VerifySession, visible: string[], state: ReviewState) => VerifySession) => {
      const state = get();
      const session = state.ui.verify;
      if (!session) return;
      const next = transition(session, visibleIds(session, state.findings, state.triage), state);
      if (next !== session) set((current) => ({ ui: { ...current.ui, verify: next } }));
    };

    /** New or withdrawn findings while the queue is open: keep its order in step */
    const syncVerify = () => {
      const state = get();
      if (!state.ui.verify) return;
      const ordered = verifyOrder(findingsList(state), state.triage, (finding) => findingRisk(index, finding));
      updateVerify((session) => mergeOrder(session, ordered));
    };

    return {
      context,
      index,
      findings: {},
      findingOrder: [],
      withdrawn: {},
      withdrawnOrder: [],
      arrivedAt: {},
      triage: {},
      viewed: EMPTY_VIEWED,
      comments: {},
      revealed: false,
      reveal: null,
      result: null,
      status: { state: 'connecting', message: 'Connecting', agent: '' },
      activity: [],
      selection: EMPTY_SELECTION,
      history: [],
      ui: {
        mainView: 'overview',
        overviewPackage: null,
        tab: 'review',
        focusedPane: 'code',
        collapsed: { sidebar: false, context: false },
        flowExpanded: false,
        flowScope: 'selection',
        wrap: false,
        filter: '',
        helpOpen: false,
        postOpen: false,
        thinkFirst: false,
        dismissing: null,
        composer: null,
        blockedCommentKey: null,
        verify: null,
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
            syncVerify();
            break;
          case 'result': {
            const resultFindings = event.data.findings;
            if (Array.isArray(resultFindings)) set((state) => applyResultFindings(state, resultFindings));
            set({ result: event.data });
            syncVerify();
            setStatus({ state: 'done', message: 'Review complete', agent: '' });
            break;
          }
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

      acceptFinding(id, note) {
        setTriage(id, { state: 'accepted', note: cleanNote(note ?? get().triage[id]?.note) });
      },
      dismissFinding(id, reason, note) {
        setTriage(id, { state: 'dismissed', reason, note: cleanNote(note) });
      },
      undoTriage(id) {
        set((state) => {
          const { [id]: _removed, ...rest } = state.triage;
          void _removed;
          return { triage: rest, ui: closeDismissFor(state.ui, id) };
        });
      },
      setTriageNote(id, note) {
        const entry = get().triage[id];
        if (entry) setTriage(id, { ...entry, note: cleanNote(note) });
      },
      toggleAccept(id) {
        if (get().triage[id]?.state === 'accepted') get().undoTriage(id);
        else get().acceptFinding(id);
      },
      startDismiss(id, surface) {
        set((state) => ({ ui: { ...state.ui, dismissing: { id, surface } } }));
      },
      cancelDismiss() {
        set((state) => ({ ui: { ...state.ui, dismissing: null } }));
      },
      markViewed(kind, id) {
        set((state) => {
          const viewed = withViewed(state.viewed, kind, id);
          return viewed === state.viewed ? {} : { viewed };
        });
      },
      setThinkFirst(enabled) {
        // Turning it on hides the findings again until the next reveal
        set((state) => ({ ui: { ...state.ui, thinkFirst: enabled }, revealed: enabled ? false : state.revealed }));
      },
      toggleThinkFirst() {
        get().setThinkFirst(!get().ui.thinkFirst);
      },
      revealFindings() {
        // The first reveal is the one that counts: re-hiding does not make you unsee them
        set((state) => ({ revealed: true, reveal: state.reveal ?? captureReveal(Date.now(), state.viewed) }));
      },

      openVerify() {
        const state = get();
        if (findingsHidden(state)) return;
        const ordered = verifyOrder(findingsList(state), state.triage, (finding) => findingRisk(index, finding));
        const session = startSession(ordered, state.triage, state.selection.findingId);
        set((current) => ({ ui: { ...current.ui, verify: session, dismissing: null } }));
      },
      closeVerify() {
        set((state) => (state.ui.verify ? { ui: { ...state.ui, verify: null } } : {}));
      },
      verifyMove(delta) {
        updateVerify((session, visible) => moveSession(session, visible, delta));
      },
      verifyAccept() {
        const id = get().ui.verify?.currentId;
        if (!id || !get().findings[id]) return;
        get().acceptFinding(id);
        updateVerify((session, visible, state) => advanceSession(session, visible, state.triage));
      },
      verifyStartDismiss() {
        updateVerify((session) => (session.currentId ? { ...session, picking: true } : session));
      },
      verifyCancelDismiss() {
        updateVerify((session) => (session.picking ? { ...session, picking: false } : session));
      },
      verifyDismiss(reason) {
        const id = get().ui.verify?.currentId;
        if (!id || !get().findings[id]) return;
        get().dismissFinding(id, reason);
        updateVerify((session, visible, state) => advanceSession(session, visible, state.triage));
      },
      verifyToggleNits() {
        updateVerify((session) => ({ ...session, showNits: !session.showNits }));
      },
      verifyToggleRelations() {
        updateVerify((session) => ({ ...session, showRelations: !session.showRelations }));
      },
      openComposer(target) {
        set((state) => ({ ui: { ...state.ui, composer: target, blockedCommentKey: null } }));
      },
      closeComposer() {
        set((state) => ({ ui: { ...state.ui, composer: null } }));
      },
      saveComment(input, editingId) {
        const existing = editingId ? get().comments[editingId] : undefined;
        const id = existing?.id ?? newCommentId(Date.now(), get().comments);
        const comment: ReviewComment = {
          id, file: input.file, ...normalizeRange(input.line, input.end_line), body: input.body.trim(),
          severity: input.severity, category: input.category, createdAt: existing?.createdAt ?? Date.now(),
        };
        set((state) => ({ comments: { ...state.comments, [id]: comment }, ui: { ...state.ui, composer: null } }));
        return id;
      },
      deleteComment(id) {
        set((state) => {
          const { [id]: _removed, ...rest } = state.comments;
          void _removed;
          const composer = state.ui.composer?.editingId === id ? null : state.ui.composer;
          const selection = state.selection.findingId === id ? { ...state.selection, findingId: null } : state.selection;
          return { comments: rest, selection, ui: { ...state.ui, composer } };
        });
      },
      setBlockedComment(rowKey) {
        set((state) => ({ ui: { ...state.ui, blockedCommentKey: rowKey } }));
      },

      selectFunction(id, { record = true, showCode = true } = {}) {
        const fn = index.functions.get(id);
        if (!fn) return;
        navigate({ file: fn.file, functionId: id, nodeId: id, findingId: null, line: fn.start_line, callSite: false }, record, showCode);
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
          // An external node has no code to open: select it in the flow only
          set((state) => ({ selection: { ...state.selection, nodeId: id } }));
          return;
        }
        navigate({
          file, functionId: get().selection.functionId, nodeId: id, findingId: null,
          line: site ? site.line : node.line, callSite: Boolean(site),
        }, true);
      },

      openFile(path, { line, record = true, showCode = true } = {}) {
        const first = (index.functionsByFile.get(path) ?? [])[0];
        navigate({ file: path, functionId: null, nodeId: null, findingId: null, line: line ?? first?.start_line ?? null, callSite: false }, record, showCode);
      },

      openLocation(path, line, { callSite = false, record = true, showCode = true } = {}) {
        const fn = callSite ? null : functionAtLine(index, path, line);
        navigate({ file: path, functionId: fn?.id ?? null, nodeId: fn?.id ?? get().selection.nodeId, findingId: null, line, callSite }, record, showCode);
      },

      openFinding(id, { record = true, showCode = true } = {}) {
        const comment = get().comments[id];
        const finding = get().findings[id] ?? (comment ? commentToFinding(comment) : undefined);
        if (!finding) return;
        const fn = (finding.function_id && index.functions.get(finding.function_id))
          || (finding.line ? functionAtLine(index, finding.file, finding.line) : null);
        navigate({
          file: finding.file, functionId: fn ? fn.id : null, nodeId: fn ? fn.id : null,
          findingId: id, line: finding.line || null, callSite: false,
        }, record, showCode);
      },

      goBack() {
        const { history } = get();
        const previous = history[history.length - 1];
        if (!previous) return;
        set((state) => ({
          history: history.slice(0, -1),
          selection: { ...previous, reveal: state.selection.reveal + 1 },
          ui: { ...state.ui, mainView: 'code' },
        }));
      },

      setMainView(view) {
        set((state) => (state.ui.mainView === view ? {} : { ui: { ...state.ui, mainView: view } }));
      },
      toggleMainView() {
        get().setMainView(get().ui.mainView === 'overview' ? 'code' : 'overview');
      },
      setOverviewPackage(key) {
        set((state) => ({ ui: { ...state.ui, overviewPackage: key } }));
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

type FindingSlice = Pick<ReviewState, 'findings' | 'findingOrder' | 'withdrawn' | 'withdrawnOrder' | 'arrivedAt'>;

/** The reviewer's own comments are never withdrawn (they are not the AI's to withdraw) */
function isOwnComment(finding: Finding): boolean {
  return finding.id.startsWith('c-') || finding.source === 'you';
}

/**
 * The lead reviewer's final result is authoritative: its findings become (or stay)
 * active, and any AI finding currently shown that the result leaves out is withdrawn.
 * A withdrawn finding the result lists again is restored. Triage entries are untouched.
 * Live streams and cache replays both end with this, so they behave the same.
 */
export function applyResultFindings(state: FindingSlice, resultFindings: readonly Finding[]): FindingSlice {
  const kept = resultFindings.filter((finding) => finding && finding.id);
  const keptIds = new Set(kept.map((finding) => finding.id));
  const findings: Record<string, Finding> = {};
  const findingOrder: string[] = [];
  const withdrawn: Record<string, Finding> = { ...state.withdrawn };
  const withdrawnOrder = state.withdrawnOrder.filter((id) => !keptIds.has(id));
  const arrivedAt = { ...state.arrivedAt };
  for (const id of state.findingOrder) {
    const finding = state.findings[id];
    if (!finding) continue;
    if (keptIds.has(id) || isOwnComment(finding)) {
      findings[id] = finding;
      findingOrder.push(id);
    } else if (!(id in withdrawn)) {
      withdrawn[id] = finding;
      withdrawnOrder.push(id);
    }
  }
  for (const finding of kept) {
    delete withdrawn[finding.id];
    if (!(finding.id in findings)) {
      findingOrder.push(finding.id);
      if (!(finding.id in arrivedAt)) arrivedAt[finding.id] = Date.now();
    }
    findings[finding.id] = finding;
  }
  return { findings, findingOrder, withdrawn, withdrawnOrder, arrivedAt };
}

function closeDismissFor(ui: UIState, id: string): UIState {
  return ui.dismissing?.id === id ? { ...ui, dismissing: null } : ui;
}

export interface ServerPersistence {
  /** The document GET /api/state returned, or null when the server had none */
  saved: unknown;
  /** Sends changes to PUT /api/state */
  sync: StateSync;
}

/**
 * Restore the reviewer's saved work for this PR + head SHA, then save every change.
 * The server copy wins over localStorage (which is per-origin, and the port changes
 * each run); localStorage is the fallback for older runs and is always written as a
 * secondary copy. Returns the unsubscribe function.
 */
export function bindPersistence(store: ReviewStore, storage: KeyValueStorage | null, server?: ServerPersistence): () => void {
  const key = reviewStorageKey(store.getState().context.pr);
  const fromServer = server ? parseReview(server.saved, key) : null;
  const saved = fromServer ?? loadReview(storage, key);
  const thinkFirst = loadThinkFirst(storage);
  store.setState((state) => ({
    ...(saved ?? {}),
    ui: { ...state.ui, thinkFirst },
  }));
  // Work found only in localStorage (an older run): copy it to the server now
  if (server && !fromServer && saved && hasWork(saved)) server.sync.schedule(serializeReview(key, saved));
  return store.subscribe((state, previous) => {
    if (state.ui.thinkFirst !== previous.ui.thinkFirst) saveThinkFirst(storage, state.ui.thinkFirst);
    const changed = state.triage !== previous.triage || state.viewed !== previous.viewed
      || state.comments !== previous.comments || state.revealed !== previous.revealed || state.reveal !== previous.reveal;
    if (!changed) return;
    const review = {
      triage: state.triage, viewed: state.viewed, comments: state.comments, revealed: state.revealed, reveal: state.reveal,
    };
    saveReview(storage, key, review);
    server?.sync.schedule(serializeReview(key, review));
  });
}

/** True while "Think first" keeps the AI's findings out of sight */
export function findingsHidden(state: Pick<ReviewState, 'ui' | 'revealed'>): boolean {
  return state.ui.thinkFirst && !state.revealed;
}

// ---------- Derived data (pure; call with state from useReview) ----------

export function findingsList(state: Pick<ReviewState, 'findings' | 'findingOrder'>): Finding[] {
  return state.findingOrder.map((id) => state.findings[id]).filter((finding): finding is Finding => Boolean(finding));
}

export function findingsForFile(state: Pick<ReviewState, 'findings' | 'findingOrder'>, path: string): Finding[] {
  return findingsList(state).filter((finding) => finding.file === path);
}

/** Kaizen risk behind a finding: its function's, else its file's (0 when unknown) */
export function findingRisk(index: ReviewIndex, finding: Finding): number {
  const fn = (finding.line ? functionAtLine(index, finding.file, finding.line) : null)
    || (finding.function_id ? index.functions.get(finding.function_id) : null);
  if (fn) return fn.risk_score;
  return index.files.get(finding.file)?.risk_score ?? 0;
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
