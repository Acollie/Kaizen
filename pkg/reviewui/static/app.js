// Kaizen review UI entry point: loads the context, wires the views together,
// streams engine events and installs the keymap.

import { fetchContext, hasToken, openEventStream } from './api.js';
import { h, clear, externalLink, severityCounts, SEVERITIES } from './dom.js';
import { createStore } from './store.js';
import { createSidebar } from './sidebar.js';
import { createCodeView } from './codeview.js';
import { createGraph } from './graph.js';
import { createActivityFeed, renderFunctionPanel } from './panels.js';
import { renderReviewPanel } from './review.js';
import { createPostModal } from './post.js';
import { createLayout } from './layout.js';
import { helpGroups, installKeymap } from './keys.js';

const byId = (id) => document.getElementById(id);

async function main() {
  if (!hasToken()) {
    showFatal('This page needs its session token. Open the URL printed by `kaizen review` (it ends in ?t=…).');
    return;
  }
  let context;
  try {
    context = await fetchContext();
  } catch (error) {
    showFatal(`Could not load the review context: ${error.message}`);
    return;
  }
  const app = createApp(createStore(context));
  app.start();
}

function showFatal(message) {
  const body = byId('code-body');
  clear(body).append(h('div', { class: 'empty-state', text: message }));
  byId('pr-title').textContent = 'Kaizen review';
  setStatus({ state: 'error', message: 'Not connected' });
}

function setStatus(status) {
  const pill = byId('status-pill');
  pill.dataset.state = status.state;
  const label = {
    connecting: 'Connecting…',
    running: status.agent && status.agent !== 'lead' ? `${status.agent}: ${status.message}` : status.message || 'Running',
    thinking: `${status.agent || 'lead'} is thinking…`,
    done: 'Review complete',
    error: status.message ? `Error: ${status.message}` : 'Error',
  }[status.state] || status.message;
  byId('status-text').textContent = label;
  pill.title = status.message || label;
}

function createApp(store) {
  const history = [];
  let currentFindingId = null;

  const layout = createLayout({ workspaceEl: byId('workspace'), onTabChange: (tab) => renderTab(tab) });
  const post = createPostModal({ store });
  const sidebar = createSidebar({
    store,
    listEl: byId('changes-list'),
    filterEl: byId('filter'),
    summaryEl: byId('changes-summary'),
    onSelectFunction: (id) => selectFunction(id),
    onSelectFile: (path) => openFile(path),
  });
  const code = createCodeView({
    store,
    scrollEl: byId('code-scroll'),
    bodyEl: byId('code-body'),
    headerEl: byId('code-header'),
    onSelectFunction: (id) => selectFunction(id),
    onCursorFinding: (id) => { currentFindingId = id; },
    onBack: () => goBack(),
    canGoBack: () => history.length > 0,
  });
  const graph = createGraph({
    store,
    canvasEl: byId('graph-canvas'),
    legendEl: byId('graph-legend'),
    tooltipEl: byId('graph-tooltip'),
    hintEl: byId('graph-hint'),
    onNodeClick: (id) => selectNode(id, { openFile: true }),
  });
  const activity = createActivityFeed({
    feedEl: byId('activity-feed'),
    panelEl: byId('panel-activity'),
    latestButton: byId('activity-latest'),
    onFindingClick: (id) => openFinding(id),
  });

  byId('graph-fit').addEventListener('click', () => graph.fit());
  byId('graph-layered').addEventListener('click', () => graph.toggleLayered());
  byId('graph-expand').addEventListener('click', () => toggleFlowExpanded());
  byId('post-button').addEventListener('click', () => post.open());
  byId('help-button').addEventListener('click', () => openHelp());
  byId('help-close').addEventListener('click', () => byId('help-modal').close());

  // ---------- Navigation ----------

  function rememberLocation() {
    const path = code.currentPath();
    if (!path) return;
    history.push({ path, line: code.cursorLine(), selection: { ...store.selection } });
    if (history.length > 100) history.shift();
  }

  function setSelection(selection) {
    store.selection = { functionId: null, nodeId: null, file: null, ...selection };
    sidebar.setActive({ file: store.selection.file, functionId: store.selection.functionId });
    if (layout.activeTab() === 'function') renderTab('function');
    if (layout.activeTab() === 'review') renderTab('review');
  }

  async function selectFunction(id, { record = true } = {}) {
    const fn = store.functions.get(id);
    if (!fn) return;
    if (record) rememberLocation();
    setSelection({ functionId: id, nodeId: id, file: fn.file });
    graph.focus(id);
    await code.showFunction(fn);
  }

  async function selectNode(id, { openFile: shouldOpen = false } = {}) {
    if (store.functions.has(id)) return selectFunction(id);
    const node = store.nodes.get(id);
    if (!node) return;
    setSelection({ nodeId: id, file: store.selection.file });
    graph.focus(id);
    if (shouldOpen && node.file) {
      const site = callSiteFor(id);
      await openLocation(site ? site.file : node.file, site ? site.line : node.line, { callSite: Boolean(site) });
    }
  }

  /** For a caller node, the call site of its call into a changed function */
  function callSiteFor(nodeId) {
    const selected = store.selection.functionId;
    const ordered = [...store.context.functions].sort((left, right) => (right.id === selected) - (left.id === selected));
    for (const fn of ordered) {
      const ref = fn.callers.find((caller) => caller.id === nodeId && caller.file);
      if (ref) return ref;
    }
    return null;
  }

  async function openFile(path, { record = true } = {}) {
    if (record) rememberLocation();
    setSelection({ file: path });
    await code.show(path);
  }

  async function openLocation(path, line, { callSite = false, record = true } = {}) {
    if (!path) return;
    if (record) rememberLocation();
    if (!store.selection.functionId || store.functions.get(store.selection.functionId)?.file !== path) {
      sidebar.setActive({ file: store.files.has(path) ? path : null, functionId: null });
    }
    try {
      await code.show(path, { line: line || 1, callSite });
    } catch (error) {
      clear(byId('code-body')).append(h('div', { class: 'empty-state', text: `Could not open ${path}: ${error.message}` }));
    }
  }

  async function openFinding(id, { record = true } = {}) {
    const finding = store.findings.get(id);
    if (!finding) return;
    if (record) rememberLocation();
    currentFindingId = id;
    queueReviewRender();
    const fn = finding.function_id && store.functions.get(finding.function_id) || store.functionAtLine(finding.file, finding.line);
    setSelection({ file: finding.file, functionId: fn ? fn.id : null, nodeId: fn ? fn.id : null });
    if (fn) graph.focus(fn.id);
    await code.showFinding(finding);
  }

  async function goBack() {
    const location = history.pop();
    if (!location) return;
    setSelection(location.selection);
    if (location.selection.nodeId) graph.focus(location.selection.nodeId);
    await code.show(location.path, { line: location.line || undefined, flash: false });
    code.refreshHeader();
  }

  function openRef(ref) {
    if (!ref.file) return;
    if (store.functions.has(ref.id)) {
      selectFunction(ref.id);
      return;
    }
    if (store.nodes.has(ref.id)) graph.focus(ref.id);
    openLocation(ref.file, ref.line, { callSite: true });
  }

  // ---------- Views fed by the store ----------

  function renderTab(tab) {
    if (tab === 'function') {
      renderFunctionPanel(byId('panel-function'), {
        store, selection: store.selection, onOpenRef: openRef, onSelectFunction: (id) => selectFunction(id),
      });
    } else if (tab === 'review') {
      renderReviewPanel(byId('panel-review'), {
        store,
        currentFindingId,
        onOpenFinding: (id) => openFinding(id),
        onSelectFunction: (id) => selectFunction(id),
      });
    } else if (tab === 'graph') {
      graph.fit();
    } else if (tab === 'activity') {
      byId('activity-badge').hidden = true;
      unseenActivity = 0;
      activity.scrollToEnd();
    }
  }

  let unseenActivity = 0;
  function noteActivity() {
    if (layout.activeTab() === 'activity') return;
    unseenActivity++;
    const badge = byId('activity-badge');
    badge.textContent = unseenActivity > 99 ? '99+' : String(unseenActivity);
    badge.hidden = false;
  }

  function renderHeader() {
    const { pr } = store.context;
    byId('pr-number').textContent = `#${pr.number}`;
    byId('pr-title').textContent = pr.title;
    byId('pr-title').title = pr.title;
    document.title = `#${pr.number} ${pr.title} · Kaizen Review`;
    clear(byId('pr-meta')).append(
      h('span', { text: `by ${pr.author}` }),
      h('span', { class: 'branch', text: `${pr.head_ref} → ${pr.base_ref}` }),
      h('span', {}, h('span', { class: 'stat-add', text: `+${pr.additions}` }), ' ', h('span', { class: 'stat-del', text: `−${pr.deletions}` }),
        ` · ${pr.file_count} files`),
      pr.url ? externalLink(pr.url, 'View on GitHub ↗') : null,
    );
  }

  const SEVERITY_TITLES = { critical: 'must fix', warning: 'should fix', suggestion: 'consider', praise: 'good' };

  function renderSeverityCounts() {
    const findings = store.findingsList();
    const counts = new Map(severityCounts(findings));
    clear(byId('severity-counts')).append(...SEVERITIES.filter((severity) => severity !== 'praise').map((severity) => {
      const count = counts.get(severity) || 0;
      return h('button', {
        type: 'button',
        class: `sev-chip sev-${severity}${count ? '' : ' zero'}`,
        title: count ? `Jump to the first ${SEVERITY_TITLES[severity]} finding` : `No ${SEVERITY_TITLES[severity]} findings`,
        disabled: count ? null : true,
        onClick: () => openFirstFinding(severity),
      }, h('span', { class: 'chip-count', text: String(count) }), ` ${SEVERITY_TITLES[severity]}`);
    }));
    const badge = byId('review-badge');
    const urgent = findings.filter((finding) => finding.severity === 'critical' || finding.severity === 'warning').length;
    badge.textContent = String(urgent || findings.length);
    badge.hidden = findings.length === 0;
    badge.dataset.urgent = urgent ? 'true' : 'false';
  }

  function openFirstFinding(severity) {
    const first = orderedFindings().find((finding) => finding.severity === severity);
    if (first) openFinding(first.id);
  }

  function toggleFlowExpanded() {
    const next = !layout.isFlowExpanded();
    layout.setFlowExpanded(next);
    graph.setExpanded(next);
    byId('graph-expand').textContent = next ? 'Collapse' : 'Expand';
  }

  // Re-render the Review tab at most once per animation frame while findings stream in
  let reviewRenderQueued = false;
  function queueReviewRender() {
    if (reviewRenderQueued || layout.activeTab() !== 'review') return;
    reviewRenderQueued = true;
    window.requestAnimationFrame(() => {
      reviewRenderQueued = false;
      renderTab('review');
    });
  }

  store.on('finding', () => {
    renderSeverityCounts();
    queueReviewRender();
    if (layout.activeTab() === 'function') renderTab('function');
  });
  store.on('result', () => queueReviewRender());
  store.on('status', (status) => {
    setStatus(status);
    queueReviewRender();
  });

  // ---------- Event stream ----------

  function handleEvent(event) {
    const data = event.data || {};
    activity.append(event);
    if (event.type !== 'done') noteActivity();
    switch (event.type) {
      case 'status':
        store.setStatus({ state: 'running', message: data.message, agent: data.agent });
        break;
      case 'thinking':
        store.setStatus({ state: 'thinking', agent: data.agent, message: 'thinking' });
        break;
      case 'tool_call':
        store.setStatus({ state: 'running', agent: data.agent, message: data.summary || data.tool });
        break;
      case 'finding':
        store.addFinding(data);
        break;
      case 'result':
        store.setResult(data);
        store.setStatus({ state: 'done', message: 'Review complete' });
        break;
      case 'error':
        store.setStatus({ state: 'error', message: data.text });
        break;
      case 'done':
        if (store.status.state !== 'error') store.setStatus({ state: 'done', message: 'Review complete' });
        break;
      default:
        break;
    }
  }

  function connectEvents() {
    openEventStream({
      onEvent: handleEvent,
      onOpen: () => { if (store.status.state === 'connecting') store.setStatus({ state: 'running', message: 'Waiting for the engine' }); },
      onError: (closed) => {
        if (store.status.state === 'done') return;
        if (closed && !store.result) store.setStatus({ state: 'done', message: 'Stream closed' });
        else if (!closed) store.setStatus({ state: 'connecting', message: 'Reconnecting' });
      },
    });
  }

  // ---------- Keyboard ----------

  function orderedFindings() {
    const fileOrder = new Map(sidebar.visibleFiles().map((file, index) => [file.path, index]));
    return store.findingsList().slice().sort((left, right) =>
      (fileOrder.get(left.file) ?? 999) - (fileOrder.get(right.file) ?? 999) || (left.line || 0) - (right.line || 0));
  }

  function stepFinding(delta) {
    const list = orderedFindings();
    if (!list.length) return;
    const index = list.findIndex((finding) => finding.id === currentFindingId);
    const next = index === -1 ? (delta > 0 ? 0 : list.length - 1) : (index + delta + list.length * 10) % list.length;
    openFinding(list[next].id);
  }

  function stepFunction(delta) {
    const list = sidebar.visibleFunctions();
    if (!list.length) return;
    const index = list.findIndex((fn) => fn.id === store.selection.functionId);
    const next = index === -1 ? (delta > 0 ? 0 : list.length - 1) : Math.max(0, Math.min(list.length - 1, index + delta));
    selectFunction(list[next].id);
  }

  function stepFile(delta) {
    const list = sidebar.visibleFiles();
    if (!list.length) return;
    const index = list.findIndex((file) => file.path === code.currentPath());
    const next = index === -1 ? 0 : Math.max(0, Math.min(list.length - 1, index + delta));
    openFile(list[next].path);
  }

  function gotoDefinition() {
    const node = store.selection.nodeId && store.nodes.get(store.selection.nodeId);
    if (node && !store.functions.has(node.id) && node.file) {
      openLocation(node.file, node.line);
      return;
    }
    const fn = store.selection.functionId && store.functions.get(store.selection.functionId);
    if (!fn) return;
    const text = code.cursorText();
    const callees = fn.callees.filter((callee) => callee.file && !callee.external);
    const target = callees.find((callee) => text.includes(`${callee.name}(`)) || callees[0];
    if (target) openRef(target);
  }

  function gotoCallers() {
    const id = store.selection.functionId;
    if (!id) return;
    layout.showTab('function');
    graph.highlightCallers(id);
    const list = byId('panel-function').querySelector('[data-refs="callers"]');
    if (list) {
      list.scrollIntoView({ block: 'center' });
      list.classList.add('flash');
      const first = list.querySelector('button:not([disabled])');
      if (first) first.focus();
    }
  }

  function openHelp() {
    const body = clear(byId('help-body'));
    for (const { group, rows } of helpGroups()) {
      body.append(h('div', { class: 'help-group' }, h('h3', { text: group }),
        rows.map((row) => h('div', { class: 'help-row' },
          h('span', { class: 'help-keys' }, row.labels.map((label) => h('kbd', { text: label }))),
          h('span', { text: row.description })))));
    }
    byId('help-modal').showModal();
  }

  const repeat = (action) => (count) => action(count);
  const handlers = {
    nextFunction: repeat((count) => stepFunction(count)),
    prevFunction: repeat((count) => stepFunction(-count)),
    nextFile: repeat((count) => stepFile(count)),
    prevFile: repeat((count) => stepFile(-count)),
    nextHunk: (count) => code.nextHunk(count),
    prevHunk: (count) => code.prevHunk(count),
    nextFinding: (count) => stepFinding(count),
    prevFinding: (count) => stepFinding(-count),
    toggleInclude: () => {
      const id = code.cursorFindingId() || currentFindingId;
      if (id) store.setIncluded(id, !store.included.has(id));
    },
    top: () => code.top(),
    bottom: () => code.bottom(),
    halfPageDown: (count) => code.scrollPage(0.5 * count),
    halfPageUp: (count) => code.scrollPage(-0.5 * count),
    pageDown: (count) => code.scrollPage(0.9 * count),
    pageUp: (count) => code.scrollPage(-0.9 * count),
    expandAbove: async (count) => { for (let step = 0; step < count; step++) await code.expandAbove(); },
    expandBelow: async (count) => { for (let step = 0; step < count; step++) await code.expandBelow(); },
    expandAll: () => code.expandAll(),
    gotoDefinition,
    gotoCallers,
    back: () => goBack(),
    focusLeft: () => layout.moveFocus(-1),
    focusRight: () => layout.moveFocus(1),
    focusFilter: () => { layout.focusPane('sidebar', { moveFocus: false }); sidebar.focusFilter(); },
    tab: (count, binding) => layout.showTab(binding.argument),
    nextTab: (count) => layout.cycleTab(count),
    prevTab: (count) => layout.cycleTab(-count),
    toggleWrap: () => code.toggleWrap(),
    fitGraph: () => { layout.showTab('graph'); graph.fit(); },
    expandFlow: () => toggleFlowExpanded(),
    toggleLayered: () => { layout.showTab('graph'); graph.toggleLayered(); },
    post: () => post.open(),
    help: () => openHelp(),
    escape: () => {
      for (const dialog of document.querySelectorAll('dialog[open]')) dialog.close();
      if (layout.isFlowExpanded()) toggleFlowExpanded();
      else graph.clear();
    },
  };

  function updateMode(pending, mode) {
    const indicator = byId('mode-indicator');
    indicator.textContent = pending ? `${mode} · ${pending}` : mode;
    indicator.classList.toggle('pending', Boolean(pending));
    indicator.classList.toggle('input', mode === 'INPUT');
  }

  function start() {
    renderHeader();
    renderSeverityCounts();
    setStatus(store.status);
    installKeymap({ handlers, onPending: updateMode, isModalOpen: () => Boolean(document.querySelector('dialog[open]')) });
    const first = store.context.functions.find((fn) => fn.focus) || store.context.functions[0];
    if (first) selectFunction(first.id, { record: false });
    else if (store.context.files[0]) openFile(store.context.files[0].path, { record: false });
    renderTab('review');
    connectEvents();
  }

  return { start };
}

main();
