// Pane layout: draggable dividers, collapse toggles, pane focus (h/l) and tabs.

const PANES = ['sidebar', 'code', 'context'];
const STORAGE_KEY = 'kaizen-review-layout';

function loadLayout() {
  try {
    return JSON.parse(window.localStorage.getItem(STORAGE_KEY)) || {};
  } catch {
    return {};
  }
}

function saveLayout(layout) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(layout));
  } catch {
    // storage unavailable (private mode); widths simply are not remembered
  }
}

export function createLayout({ workspaceEl, onTabChange }) {
  const layout = loadLayout();
  const rootStyle = document.documentElement.style;
  if (layout.sidebar) rootStyle.setProperty('--sidebar-w', `${layout.sidebar}px`);
  if (layout.context) rootStyle.setProperty('--context-w', `${layout.context}px`);
  let focusedPane = 'code';
  let activeTab = 'graph';

  for (const divider of workspaceEl.querySelectorAll('[data-divider]')) setupDivider(divider);
  for (const button of workspaceEl.querySelectorAll('[data-collapse]')) {
    button.addEventListener('click', () => setCollapsed(button.dataset.collapse, true));
  }
  for (const button of workspaceEl.querySelectorAll('[data-restore]')) {
    button.addEventListener('click', () => setCollapsed(button.dataset.restore, false));
  }
  for (const tab of document.querySelectorAll('[data-tab]')) {
    tab.addEventListener('click', () => showTab(tab.dataset.tab));
  }
  for (const pane of PANES) {
    paneEl(pane).addEventListener('mousedown', () => focusPane(pane, { moveFocus: false }));
  }
  focusPane('code', { moveFocus: false });

  function paneEl(name) {
    return document.getElementById(`pane-${name}`);
  }

  function setupDivider(divider) {
    const side = divider.dataset.divider;
    divider.addEventListener('pointerdown', (event) => {
      event.preventDefault();
      divider.setPointerCapture(event.pointerId);
      divider.classList.add('dragging');
      const bounds = workspaceEl.getBoundingClientRect();
      const move = (moveEvent) => {
        const width = side === 'sidebar' ? moveEvent.clientX - bounds.left : bounds.right - moveEvent.clientX;
        const clamped = Math.round(Math.max(180, Math.min(bounds.width * 0.5, width)));
        rootStyle.setProperty(side === 'sidebar' ? '--sidebar-w' : '--context-w', `${clamped}px`);
        layout[side] = clamped;
      };
      const stop = () => {
        divider.classList.remove('dragging');
        divider.removeEventListener('pointermove', move);
        divider.removeEventListener('pointerup', stop);
        saveLayout(layout);
        window.dispatchEvent(new Event('resize'));
      };
      divider.addEventListener('pointermove', move);
      divider.addEventListener('pointerup', stop);
    });
  }

  function setCollapsed(side, collapsed) {
    workspaceEl.classList.toggle(`${side}-collapsed`, collapsed);
    workspaceEl.querySelector(`[data-restore="${side}"]`).hidden = !collapsed;
    if (collapsed && focusedPane === side) focusPane('code');
  }

  function isCollapsed(side) {
    return workspaceEl.classList.contains(`${side}-collapsed`);
  }

  function focusPane(name, { moveFocus = true } = {}) {
    if (isCollapsed(name)) setCollapsed(name, false);
    focusedPane = name;
    for (const pane of PANES) paneEl(pane).classList.toggle('pane-focused', pane === name);
    if (moveFocus) {
      const target = { sidebar: 'changes-list', code: 'code-scroll', context: `panel-${activeTab}` }[name];
      const element = document.getElementById(target);
      if (element) {
        if (!element.hasAttribute('tabindex')) element.setAttribute('tabindex', '-1');
        element.focus({ preventScroll: true });
      }
    }
  }

  function moveFocus(delta) {
    const index = PANES.indexOf(focusedPane);
    focusPane(PANES[Math.max(0, Math.min(PANES.length - 1, index + delta))]);
  }

  function showTab(name) {
    activeTab = name;
    for (const tab of document.querySelectorAll('[data-tab]')) tab.setAttribute('aria-selected', String(tab.dataset.tab === name));
    for (const panel of document.querySelectorAll('[data-panel]')) panel.hidden = panel.dataset.panel !== name;
    if (isCollapsed('context')) setCollapsed('context', false);
    onTabChange(name);
  }

  function cycleTab(delta) {
    const tabs = [...document.querySelectorAll('[data-tab]')].map((tab) => tab.dataset.tab);
    const index = tabs.indexOf(activeTab);
    showTab(tabs[(((index + delta) % tabs.length) + tabs.length) % tabs.length]);
  }

  return { showTab, cycleTab, focusPane, moveFocus, activeTab: () => activeTab, focusedPane: () => focusedPane };
}
