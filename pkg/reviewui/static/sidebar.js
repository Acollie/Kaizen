// "Changes" sidebar: files grouped by what needs a reviewer's attention (AI findings
// that should be fixed first, then Kaizen's metrics-based focus, then the rest), each
// expanding to its changed functions with risk bars, focus stars and finding badges.

import { h, clear, makeRiskBar, riskChip, severityBadges, severityCounts, splitPath } from './dom.js';

const GROUPS = [
  { key: 'attention', title: 'Needs attention', hint: 'AI found issues to fix' },
  { key: 'focus', title: 'Kaizen focus', hint: 'riskiest by metrics' },
  { key: 'other', title: 'Other changes', hint: '' },
];
const FOCUS_FILE_SCORE = 20;
const AUTO_EXPAND_LIMIT = 24;

export function truncateMiddle(text, limit) {
  if (text.length <= limit) return text;
  const head = Math.ceil((limit - 1) / 2);
  const tail = Math.floor((limit - 1) / 2);
  return `${text.slice(0, head)}…${text.slice(text.length - tail)}`;
}

export function createSidebar({ store, listEl, filterEl, summaryEl, onSelectFunction, onSelectFile }) {
  const expanded = new Set();
  let filterText = '';
  let activeFile = null;
  let activeFunction = null;

  const files = [...store.context.files];
  const functionsFor = (path) => (store.functionsByFile.get(path) || [])
    .slice().sort((left, right) => right.risk_score - left.risk_score || left.start_line - right.start_line);

  // Expand everything for small PRs; for big ones only files containing focus functions
  const totalFunctions = store.context.functions.length;
  for (const file of files) {
    const fns = functionsFor(file.path);
    if (totalFunctions <= AUTO_EXPAND_LIMIT || fns.some((fn) => fn.focus)) expanded.add(file.path);
  }

  filterEl.addEventListener('input', () => {
    filterText = filterEl.value.trim().toLowerCase();
    render();
  });
  filterEl.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
      const first = visibleFunctions()[0];
      if (first) onSelectFunction(first.id);
      filterEl.blur();
    }
  });
  store.on('finding', () => render());

  // Name truncation depends on the sidebar width, so re-render when it is resized
  let lastWidth = 0;
  let resizeTimer = null;
  new ResizeObserver(() => {
    if (Math.abs(listEl.clientWidth - lastWidth) < 8) return;
    lastWidth = listEl.clientWidth;
    window.clearTimeout(resizeTimer);
    resizeTimer = window.setTimeout(render, 80);
  }).observe(listEl);

  /** Which sidebar group a file belongs to */
  function groupOf(file) {
    const findings = store.findingsForFile(file.path);
    if (findings.some((finding) => finding.severity === 'critical' || finding.severity === 'warning')) return 'attention';
    const hasFocus = functionsFor(file.path).some((fn) => fn.focus);
    if (hasFocus || file.risk_score >= FOCUS_FILE_SCORE) return 'focus';
    return 'other';
  }

  /** Files ordered by AI severity first (criticals, then warnings), then by Kaizen risk */
  function orderedFiles(list) {
    const weight = (file) => {
      const findings = store.findingsForFile(file.path);
      const critical = findings.filter((finding) => finding.severity === 'critical').length;
      const warning = findings.filter((finding) => finding.severity === 'warning').length;
      return critical * 1000 + warning * 100 + file.risk_score;
    };
    return [...list].sort((left, right) => weight(right) - weight(left) || left.path.localeCompare(right.path));
  }

  function matches(file, fn) {
    if (!filterText) return true;
    return file.path.toLowerCase().includes(filterText) || (fn && fn.name.toLowerCase().includes(filterText));
  }

  function visibleFiles() {
    const shown = files.filter((file) => matches(file) || functionsFor(file.path).some((fn) => matches(file, fn)));
    const ordered = orderedFiles(shown);
    return GROUPS.flatMap(({ key }) => ordered.filter((file) => groupOf(file) === key));
  }

  /** Changed functions in sidebar order (respecting the filter) */
  function visibleFunctions() {
    return visibleFiles().flatMap((file) => functionsFor(file.path).filter((fn) => matches(file, fn)));
  }

  function render() {
    const scrollTop = listEl.scrollTop;
    clear(listEl);
    const shown = visibleFiles();
    summaryEl.textContent = `${files.length} files · ${totalFunctions} functions`;
    if (!shown.length) {
      listEl.append(h('div', { class: 'list-empty', text: 'No files or functions match the filter.' }));
      return;
    }
    for (const { key, title, hint } of GROUPS) {
      const group = orderedFiles(shown).filter((file) => groupOf(file) === key);
      if (!group.length) continue;
      listEl.append(h('div', { class: `risk-group-title group-${key}` },
        h('span', { class: 'group-name', text: title }),
        h('span', { class: 'group-count', text: String(group.length) }),
        hint ? h('span', { class: 'group-hint', text: hint }) : null));
      for (const file of group) listEl.append(renderFile(file));
    }
    listEl.scrollTop = scrollTop;
  }

  function renderFile(file) {
    const isOpen = expanded.has(file.path) || Boolean(filterText);
    const { dir, base } = splitPath(file.path);
    const findings = store.findingsForFile(file.path);
    const row = h('button', {
      type: 'button',
      class: `file-row${file.path === activeFile ? ' active' : ''}`,
      'aria-expanded': String(isOpen),
      title: [file.path, ...(file.risk_reasons || [])].join('\n'),
      dataset: { path: file.path },
    },
    h('span', { class: 'chevron', text: '▶' }),
    h('span', { class: 'file-status', dataset: { status: file.status }, text: file.status[0].toUpperCase() }),
    h('span', { class: 'file-name' }, h('bdi', {}, h('span', { class: 'dir', text: dir }), h('span', { class: 'base', text: base }))),
    h('span', { class: 'file-side' }, severityBadges(findings, { animateIds: store.freshFindingIds }),
      file.risk_score > 0 ? riskChip(file.risk_score) : null));
    row.addEventListener('click', (event) => {
      if (event.target.closest('.chevron') || file.path === activeFile) toggle(file.path);
      else expanded.add(file.path);
      onSelectFile(file.path);
    });

    const wrapper = h('div', { class: 'file-group' }, row);
    if (isOpen) {
      const fns = functionsFor(file.path).filter((fn) => matches(file, fn));
      wrapper.append(h('div', { class: 'fn-list' }, fns.map(renderFunction)));
    }
    return wrapper;
  }

  function renderFunction(fn) {
    const findings = store.findingsForFunction(fn);
    const row = h('button', {
      type: 'button',
      class: `fn-row${fn.id === activeFunction ? ' active' : ''}`,
      title: `${fn.id}\nrisk ${Math.round(fn.risk_score)}${fn.focus ? ' · focus' : ''}`,
      dataset: { fn: fn.id },
      onClick: () => onSelectFunction(fn.id),
    },
    h('span', { class: 'star', text: fn.focus ? '★' : '' }),
    renderFunctionName(fn, severityCounts(findings).length),
    h('span', { class: 'badges' }, severityBadges(findings, { animateIds: store.freshFindingIds })),
    makeRiskBar(fn.risk_score),
    h('span', { class: 'risk-score', text: Math.round(fn.risk_score) }));
    return row;
  }

  /** Receiver-qualified name, truncated in the middle so both ends stay readable */
  function renderFunctionName(fn, badgeCount) {
    const receiver = fn.receiver_type ? fn.receiver_type.replace('*', '') : '';
    // row chrome: padding+star (50px), risk bar+score (70px), gaps (18px), ~22px per badge; 12px mono ≈ 7.3px/char
    const available = (listEl.clientWidth || 300) - 138 - badgeCount * 22;
    const budget = Math.max(10, Math.floor(available / 7.3));
    const full = receiver ? `${receiver}.${fn.name}` : fn.name;
    if (full.length <= budget) {
      return h('span', { class: 'fn-name' }, receiver ? h('span', { class: 'recv', text: `${receiver}.` }) : null, fn.name);
    }
    return h('span', { class: 'fn-name', text: truncateMiddle(full, budget) });
  }

  function toggle(path) {
    if (expanded.has(path)) expanded.delete(path);
    else expanded.add(path);
    render();
  }

  function setActive({ file, functionId }) {
    activeFile = file || null;
    activeFunction = functionId || null;
    if (activeFile) expanded.add(activeFile);
    render();
    const selector = activeFunction ? `[data-fn="${CSS.escape(activeFunction)}"]` : activeFile ? `[data-path="${CSS.escape(activeFile)}"]` : null;
    const element = selector && listEl.querySelector(selector);
    if (element) element.scrollIntoView({ block: 'nearest' });
  }

  render();
  return {
    setActive,
    visibleFunctions,
    visibleFiles,
    focusFilter: () => { filterEl.focus(); filterEl.select(); },
  };
}
