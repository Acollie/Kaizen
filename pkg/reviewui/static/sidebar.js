// "Changes" sidebar: files grouped by risk tier, each expanding to its changed
// functions with risk bars, focus stars and finding badges.

import { h, clear, makeRiskBar, riskChip, riskColor, riskTier, severityBadges, splitPath } from './dom.js';

const TIERS = [
  { tier: 'high', title: 'High risk', score: 80 },
  { tier: 'medium', title: 'Medium risk', score: 45 },
  { tier: 'low', title: 'Low risk', score: 10 },
];
const AUTO_EXPAND_LIMIT = 24;

export function createSidebar({ store, listEl, filterEl, summaryEl, onSelectFunction, onSelectFile }) {
  const expanded = new Set();
  let filterText = '';
  let activeFile = null;
  let activeFunction = null;

  const files = [...store.context.files].sort((left, right) => right.risk_score - left.risk_score || left.path.localeCompare(right.path));
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

  function matches(file, fn) {
    if (!filterText) return true;
    return file.path.toLowerCase().includes(filterText) || (fn && fn.name.toLowerCase().includes(filterText));
  }

  function visibleFiles() {
    return files.filter((file) => matches(file) || functionsFor(file.path).some((fn) => matches(file, fn)));
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
    for (const { tier, title, score } of TIERS) {
      const group = shown.filter((file) => riskTier(file.risk_score) === tier);
      if (!group.length) continue;
      const swatch = h('span', { class: 'swatch' });
      swatch.style.background = riskColor(score);
      listEl.append(h('div', { class: 'risk-group-title' }, swatch, `${title} · ${group.length}`));
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
      title: file.path,
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
    h('span', { class: 'fn-name' }, fn.receiver_type ? h('span', { class: 'recv', text: `${fn.receiver_type.replace('*', '')}.` }) : null, fn.name),
    makeRiskBar(fn.risk_score),
    h('span', { class: 'risk-score', text: Math.round(fn.risk_score) }),
    h('span', { class: 'badges' }, severityBadges(findings, { animateIds: store.freshFindingIds })));
    return row;
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
