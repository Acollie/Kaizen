// Main code view: a file's diff with old/new gutters, expandable context between
// hunks, inline finding cards and a keyboard cursor. Files outside the diff (for
// example a caller's file) render as read-only context fetched from /api/file.

import { fetchFileLines } from './api.js';
import { h, clear, riskChip, riskColor, splitPath } from './dom.js';
import { highlightInto, languageForPath } from './highlight.js';
import { renderFindingCard, updateFindingCard } from './findings.js';

const EXPAND_STEP = 20;
const MAX_FETCH = 500;
const PLAIN_WINDOW = 40;

export function createCodeView({ store, scrollEl, bodyEl, headerEl, onSelectFunction, onCursorFinding, onBack, canGoBack }) {
  const models = new Map();
  let current = null;
  let view = null; // per-render lookups for the current model
  let cursorEl = null;

  // Cards, gaps and notes are sized to the visible pane width so they never run off-screen
  new ResizeObserver(() => {
    bodyEl.style.setProperty('--code-view-w', `${scrollEl.clientWidth}px`);
  }).observe(scrollEl);

  function toggleWrap() {
    bodyEl.classList.toggle('wrap');
    return bodyEl.classList.contains('wrap');
  }

  store.on('finding', ({ finding }) => {
    if (!current || finding.file !== current.path) return;
    const card = view.cards.get(finding.id);
    if (card) updateFindingCard(card, finding, store);
    else placeFinding(finding);
  });
  store.on('included', ({ id }) => {
    const card = view && view.cards.get(id);
    if (card) updateFindingCard(card, store.findings.get(id), store);
  });

  // ---------- Models ----------

  function hunkBegin(hunk) {
    return hunk.new_count === 0 ? hunk.new_start + 1 : hunk.new_start;
  }
  function hunkEnd(hunk) {
    return hunkBegin(hunk) + hunk.new_count - 1;
  }
  function oldBegin(hunk) {
    return hunk.old_count === 0 ? hunk.old_start + 1 : hunk.old_start;
  }
  function oldEnd(hunk) {
    return oldBegin(hunk) + hunk.old_count - 1;
  }

  function gapRow(from, to, offset, edge) {
    return { type: 'gap', from, to, offset, edge };
  }

  function buildDiffModel(file) {
    const rows = [];
    const expandable = file.status !== 'deleted' && !file.binary;
    file.hunks.forEach((hunk, index) => {
      const previous = file.hunks[index - 1];
      const gapFrom = previous ? hunkEnd(previous) + 1 : 1;
      const gapTo = hunkBegin(hunk) - 1;
      if (expandable && gapTo >= gapFrom) {
        const offset = previous ? oldEnd(previous) - hunkEnd(previous) : oldBegin(hunk) - hunkBegin(hunk);
        rows.push(gapRow(gapFrom, gapTo, offset, previous ? 'middle' : 'top'));
      }
      rows.push({ type: 'hunk', index, hunk });
      for (const line of hunk.lines) {
        rows.push({ type: 'line', kind: line.kind, old: line.old_line, new: line.new_line, text: line.text });
      }
    });
    const last = file.hunks[file.hunks.length - 1];
    if (expandable && last && file.line_count > hunkEnd(last)) {
      rows.push(gapRow(hunkEnd(last) + 1, file.line_count, oldEnd(last) - hunkEnd(last), 'bottom'));
    }
    return { path: file.path, kind: 'diff', file, rows, language: file.language || languageForPath(file.path) };
  }

  async function buildPlainModel(path, focusLine) {
    const from = Math.max(1, focusLine - PLAIN_WINDOW);
    const slice = await fetchFileLines(path, from, focusLine + PLAIN_WINDOW);
    const rows = [];
    const first = slice.lines.length ? slice.lines[0].n : 1;
    const lastLine = slice.lines.length ? slice.lines[slice.lines.length - 1].n : 0;
    if (first > 1) rows.push(gapRow(1, first - 1, 0, 'top'));
    for (const line of slice.lines) rows.push({ type: 'line', kind: 'context', old: 0, new: line.n, text: line.text });
    if (slice.total > lastLine) rows.push(gapRow(lastLine + 1, slice.total, 0, 'bottom'));
    return { path, kind: 'plain', rows, total: slice.total, language: languageForPath(path) };
  }

  // ---------- Rendering ----------

  function render(model) {
    current = model;
    view = { rowEls: new Map(), slots: new Map(), cards: new Map(), gapEls: new Map(), rowOf: new WeakMap() };
    cursorEl = null;
    renderHeader(model);
    clear(bodyEl);

    const notes = h('div', { class: 'file-notes' });
    view.notes = notes;
    const table = h('div', { class: 'code-table', role: 'table', 'aria-label': model.path });
    view.table = table;
    bodyEl.append(notes, table);

    if (model.kind === 'diff' && (model.file.binary || model.file.hunks.length === 0)) {
      table.append(h('div', { class: 'empty-state', text: model.file.binary ? 'Binary file not shown.' : 'No textual changes.' }));
    }
    let lastNew = 0;
    for (const row of model.rows) {
      table.append(renderRow(row, lastNew));
      if (row.type === 'line' && row.new) lastNew = row.new;
    }
    for (const finding of store.findingsForFile(model.path)) placeFinding(finding);
    scrollEl.scrollTop = 0;
    scrollEl.scrollLeft = 0;
  }

  function renderHeader(model) {
    clear(headerEl);
    const { dir, base } = splitPath(model.path);
    const pathEl = h('span', { class: 'path', title: model.path }, h('span', { class: 'dir', text: dir }), base);
    headerEl.append(pathEl);
    if (model.kind === 'diff') {
      const file = model.file;
      headerEl.append(
        h('span', { class: 'tag', text: file.status }),
        file.language ? h('span', { class: 'tag', text: file.language }) : null,
        h('span', { class: 'stat-add', text: `+${file.additions}` }),
        h('span', { class: 'stat-del', text: `−${file.deletions}` }),
        file.risk_score > 0 ? riskChip(file.risk_score) : null,
      );
    } else {
      headerEl.append(h('span', { class: 'tag readonly', text: 'Not changed in this PR · read-only context' }));
    }
    headerEl.append(h('span', { class: 'spacer' }));
    if (canGoBack()) {
      headerEl.append(h('button', { type: 'button', class: 'small-button back-button', title: 'Jump back (Ctrl-o)', onClick: onBack, text: '← Back' }));
    }
  }

  function renderRow(row, lastNew) {
    if (row.type === 'hunk') return renderHunkHeader(row);
    if (row.type === 'gap') return renderGap(row);
    return renderLine(row, lastNew);
  }

  function renderLine(row, lastNew) {
    const sign = row.kind === 'added' ? '+' : row.kind === 'removed' ? '−' : ' ';
    const text = highlightInto(h('span', { class: 'text' }), row.text, current.language);
    const element = h('div', { class: `row ${row.kind}`, role: 'row' },
      h('span', { class: 'ln old', text: row.old || '' }),
      h('span', { class: 'ln new', text: row.new || '' }),
      h('span', { class: 'sign', text: sign }),
      text);
    if (current.kind === 'plain') element.classList.add('plain');
    const line = row.new || lastNew;
    const fn = current.kind === 'diff' && line ? store.functionAtLine(current.path, line) : null;
    if (fn) {
      element.classList.add('in-fn');
      element.style.setProperty('--fn-color', riskColor(fn.risk_score));
      element.dataset.fn = fn.id;
    }
    if (row.new) {
      element.dataset.line = String(row.new);
      view.rowEls.set(row.new, element);
    }
    view.rowOf.set(element, row);
    element.addEventListener('click', () => setCursor(element, { scroll: false }));
    return element;
  }

  function renderHunkHeader(row) {
    const { hunk } = row;
    const begin = hunkBegin(hunk);
    const end = Math.max(begin, hunkEnd(hunk));
    const enclosing = (store.functionsByFile.get(current.path) || [])
      .filter((fn) => fn.start_line <= end && fn.end_line >= begin)
      .sort((left, right) => right.risk_score - left.risk_score)[0];
    const range = `@@ -${hunk.old_start},${hunk.old_count} +${hunk.new_start},${hunk.new_count} @@`;
    const label = enclosing
      ? h('button', { type: 'button', class: 'enclosing link-like', title: 'Select function', onClick: () => onSelectFunction(enclosing.id) },
        enclosing.receiver_type ? `(${enclosing.receiver_type}) ${enclosing.name}` : enclosing.name)
      : h('span', { class: 'enclosing-text', text: hunk.header || '' });
    const element = h('div', { class: 'hunk-header', dataset: { hunk: String(row.index) } },
      h('span', { class: 'range', text: range }), label, enclosing ? riskChip(enclosing.risk_score) : null);
    if (label.tagName === 'BUTTON') {
      label.classList.add('enclosing');
      label.style.background = 'none';
      label.style.border = 'none';
      label.style.padding = '0';
    }
    view.rowOf.set(element, row);
    element.addEventListener('click', (event) => {
      if (event.target === element || event.target.classList.contains('range')) setCursor(element, { scroll: false });
    });
    return element;
  }

  function renderGap(row) {
    const size = row.to - row.from + 1;
    const button = (text, title, handler) => h('button', { type: 'button', title, onClick: handler, text });
    const controls = [];
    if (size <= EXPAND_STEP) {
      controls.push(button(`Show ${size} hidden line${size === 1 ? '' : 's'}`, 'Expand', () => expandGap(row, 'all')));
    } else {
      if (row.edge !== 'bottom') controls.push(button(`⋯ Show ${EXPAND_STEP} more lines above`, 'Expand upward from the next hunk (e / zo)', () => expandGap(row, 'above')));
      if (row.edge !== 'top') controls.push(button(`⋯ Show ${EXPAND_STEP} more lines below`, 'Expand downward from the previous hunk (E / zc)', () => expandGap(row, 'below')));
      controls.push(button(`Show all ${size} hidden lines`, 'Expand the whole gap', () => expandGap(row, 'all')));
    }
    const element = h('div', { class: 'gap', role: 'row' },
      h('span', { class: 'dots', text: '⋯' }), controls,
      h('span', { class: 'gap-count', text: `lines ${row.from}–${row.to}` }));
    view.gapEls.set(row, element);
    view.rowOf.set(element, row);
    return element;
  }

  // ---------- Context expansion ----------

  async function fetchRange(path, from, to) {
    const lines = [];
    for (let start = from; start <= to; start += MAX_FETCH) {
      const slice = await fetchFileLines(path, start, Math.min(to, start + MAX_FETCH - 1));
      lines.push(...slice.lines);
      if (slice.lines.length === 0) break;
    }
    return lines;
  }

  function rangeFor(row, direction) {
    if (direction === 'above') return [Math.max(row.from, row.to - EXPAND_STEP + 1), row.to];
    if (direction === 'below') return [row.from, Math.min(row.to, row.from + EXPAND_STEP - 1)];
    return [row.from, row.to];
  }

  function expandGap(row, direction) {
    const [from, to] = rangeFor(row, direction);
    return revealRange(row, from, to);
  }

  /** Replace part of a gap with real lines, keeping the remainder(s) as smaller gaps */
  async function revealRange(row, from, to) {
    const model = current;
    const element = view.gapEls.get(row);
    if (!element || element.classList.contains('loading')) return;
    element.classList.add('loading');
    let lines;
    try {
      lines = await fetchRange(model.path, from, to);
    } catch (error) {
      element.classList.remove('loading');
      element.querySelector('.gap-count').textContent = `Could not load lines: ${error.message}`;
      return;
    }
    if (model !== current) return;

    const replacement = [];
    if (from > row.from) replacement.push(gapRow(row.from, from - 1, row.offset, row.edge === 'top' ? 'top' : 'middle'));
    for (const line of lines) {
      replacement.push({ type: 'line', kind: 'context', old: model.kind === 'diff' ? line.n + row.offset : 0, new: line.n, text: line.text });
    }
    if (to < row.to) replacement.push(gapRow(to + 1, row.to, row.offset, row.edge === 'bottom' ? 'bottom' : 'middle'));

    model.rows.splice(model.rows.indexOf(row), 1, ...replacement);
    const elements = replacement.map((newRow) => {
      const rendered = renderRow(newRow, 0);
      if (newRow.type === 'line') rendered.classList.add('revealed');
      return rendered;
    });
    const wasCursor = cursorEl === element;
    element.replaceWith(...elements);
    view.gapEls.delete(row);
    if (wasCursor) setCursor(elements[0], { scroll: false });
    relocateNotes();
  }

  async function revealLine(line) {
    if (view.rowEls.has(line)) return view.rowEls.get(line);
    const gap = current.rows.find((row) => row.type === 'gap' && line >= row.from && line <= row.to);
    if (!gap) return null;
    await revealRange(gap, Math.max(gap.from, line - 10), Math.min(gap.to, line + 10));
    return view.rowEls.get(line) || null;
  }

  async function expandAll() {
    for (const row of current.rows.filter((candidate) => candidate.type === 'gap')) {
      await revealRange(row, row.from, row.to);
    }
  }

  // ---------- Findings ----------

  function findingLine(finding) {
    return finding.end_line || finding.line || 0;
  }

  function placeFinding(finding) {
    const card = renderFindingCard(finding, store, {
      onFocus: (focusedCard) => setCursor(focusedCard, { scroll: false }),
      onShowInContext: (line) => scrollToLine(line, { flash: true }),
    });
    view.cards.set(finding.id, card);
    const line = findingLine(finding);
    if (line && view.rowEls.has(line)) insertInline(card, line);
    else insertNote(card, finding);
  }

  function insertInline(card, line) {
    let slot = view.slots.get(line);
    if (!slot) {
      slot = h('div', { class: 'finding-slot', dataset: { line: String(line) } });
      view.rowEls.get(line).after(slot);
      view.slots.set(line, slot);
    }
    slot.append(card);
  }

  function insertNote(card, finding) {
    if (!view.notes.firstChild) {
      view.notes.append(h('div', { class: 'file-notes-title', text: 'File-level findings and findings outside the visible diff' }));
    }
    if (!finding.line) view.notes.firstChild.after(card);
    else view.notes.append(card);
  }

  function relocateNotes() {
    for (const card of [...view.notes.querySelectorAll('.finding-card')]) {
      const finding = store.findings.get(card.dataset.findingId);
      const line = finding && findingLine(finding);
      if (line && view.rowEls.has(line)) insertInline(card, line);
    }
    if (!view.notes.querySelector('.finding-card')) clear(view.notes);
  }

  // ---------- Cursor and scrolling ----------

  function setCursor(element, { scroll = true, block = 'nearest' } = {}) {
    if (!element) return;
    if (cursorEl) cursorEl.classList.remove('cursor');
    cursorEl = element;
    element.classList.add('cursor');
    if (scroll) element.scrollIntoView({ block, inline: 'nearest' });
    if (element.classList.contains('finding-card')) onCursorFinding(element.dataset.findingId);
  }

  function flash(elements) {
    for (const element of elements) {
      element.classList.remove('flash');
      void element.offsetWidth; // restart the animation
      element.classList.add('flash');
    }
  }

  function navigableElements() {
    return [...view.table.querySelectorAll('.row, .hunk-header, .gap, .finding-card'), ...view.notes.querySelectorAll('.finding-card')];
  }

  function sortedNavigable() {
    return navigableElements().sort((left, right) => (left.compareDocumentPosition(right) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1));
  }

  async function show(path, options = {}) {
    let model = models.get(path);
    const inDiff = store.files.has(path);
    if (!model && inDiff) model = buildDiffModel(store.files.get(path));
    if (!model) {
      model = await buildPlainModel(path, options.line || 1);
    }
    models.set(path, model);
    if (model !== current || options.rerender) render(model);
    else renderHeader(model);
    if (options.line) await scrollToLine(options.line, options);
  }

  async function scrollToLine(line, { flash: shouldFlash = true, callSite = false, block = 'center' } = {}) {
    const element = await revealLine(line);
    if (!element) return;
    if (callSite) {
      for (const previous of view.table.querySelectorAll('.call-site')) previous.classList.remove('call-site');
      element.classList.add('call-site');
    }
    setCursor(element, { block });
    if (shouldFlash) flash([element]);
  }

  async function showFunction(fn) {
    await show(fn.file);
    const start = await revealLine(fn.start_line);
    if (!start) {
      // the function's first line is not available (e.g. deleted); fall back to the first changed row
      const any = view.table.querySelector(`[data-fn="${CSS.escape(fn.id)}"]`);
      if (any) setCursor(any, { block: 'start' });
      return;
    }
    setCursor(start, { block: 'start' });
    flash([...view.table.querySelectorAll(`.row[data-fn="${CSS.escape(fn.id)}"]`)]);
  }

  async function showFinding(finding) {
    await show(finding.file);
    let card = view.cards.get(finding.id);
    const line = findingLine(finding);
    if (line && !view.rowEls.has(line)) {
      await revealLine(line);
      card = view.cards.get(finding.id);
    }
    if (!card) return;
    setCursor(card, { block: 'center' });
    flash([card]);
  }

  function currentHunkIndex(hunks) {
    if (!cursorEl) return -1;
    let index = -1;
    hunks.forEach((hunk, position) => {
      if (hunk === cursorEl || hunk.compareDocumentPosition(cursorEl) & Node.DOCUMENT_POSITION_FOLLOWING) index = position;
    });
    return index;
  }

  function moveHunk(delta) {
    const hunks = [...view.table.querySelectorAll('.hunk-header')];
    if (!hunks.length) return;
    const index = currentHunkIndex(hunks);
    const target = index === -1 ? (delta > 0 ? 0 : -1) : index + delta;
    const clamped = Math.max(0, Math.min(hunks.length - 1, target));
    if (target < 0 && index === -1) return;
    setCursor(hunks[clamped], { block: 'start' });
    flash([hunks[clamped]]);
  }

  async function expandAroundHunk(direction) {
    const hunks = [...view.table.querySelectorAll('.hunk-header')];
    let gap = null;
    if (!hunks.length) {
      gap = current.rows.find((row) => row.type === 'gap' && row.edge === (direction === 'above' ? 'top' : 'bottom'));
    } else {
      const hunk = hunks[Math.max(0, currentHunkIndex(hunks))];
      gap = direction === 'above' ? gapBefore(hunk) : gapAfter(hunk);
    }
    if (gap) await expandGap(gap, direction === 'above' ? 'above' : 'below');
  }

  function gapBefore(hunkEl) {
    const previous = hunkEl.previousElementSibling;
    const row = previous && view.rowOf.get(previous);
    return row && row.type === 'gap' ? row : null;
  }

  function gapAfter(hunkEl) {
    let next = hunkEl.nextElementSibling;
    while (next && !next.classList.contains('hunk-header')) {
      const row = view.rowOf.get(next);
      if (row && row.type === 'gap') return row;
      next = next.nextElementSibling;
    }
    return null;
  }

  function firstVisible() {
    const top = scrollEl.getBoundingClientRect().top;
    return sortedNavigable().find((element) => element.getBoundingClientRect().bottom > top + 4) || null;
  }

  function scrollPage(fraction) {
    scrollEl.scrollBy({ top: scrollEl.clientHeight * fraction });
    window.requestAnimationFrame(() => setCursor(firstVisible(), { scroll: false }));
  }

  function edge(where) {
    const elements = sortedNavigable();
    if (!elements.length) return;
    if (where === 'top') {
      scrollEl.scrollTop = 0;
      setCursor(elements[0], { scroll: false });
    } else {
      const lastRow = [...view.table.querySelectorAll('.row, .gap')].pop() || elements[elements.length - 1];
      setCursor(lastRow, { block: 'end' });
    }
  }

  function cursorFindingId() {
    return cursorEl && cursorEl.classList.contains('finding-card') ? cursorEl.dataset.findingId : null;
  }

  function cursorText() {
    const row = cursorEl && view.rowOf.get(cursorEl);
    return row && row.type === 'line' ? row.text : '';
  }

  function cursorLine() {
    const row = cursorEl && view.rowOf.get(cursorEl);
    return row && row.type === 'line' ? row.new : 0;
  }

  return {
    show,
    toggleWrap,
    showFunction,
    showFinding,
    scrollToLine,
    currentPath: () => (current ? current.path : null),
    nextHunk: (count) => moveHunk(count),
    prevHunk: (count) => moveHunk(-count),
    expandAbove: () => expandAroundHunk('above'),
    expandBelow: () => expandAroundHunk('below'),
    expandAll,
    top: () => edge('top'),
    bottom: () => edge('bottom'),
    scrollPage,
    cursorFindingId,
    cursorText,
    cursorLine,
    refreshHeader: () => current && renderHeader(current),
  };
}
