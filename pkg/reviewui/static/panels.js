// Context panel tabs: Function details, the live Activity feed and the Summary.

import { h, clear, agentTag, formatTime, makeRiskBar, severityCounts, truncate } from './dom.js';
import { renderMarkdown } from './markdown.js';

// ---------- Function tab ----------

const METRIC_ROWS = [
  ['Cyclomatic complexity', (metrics) => metrics.cyclomatic_complexity, [10, 20]],
  ['Cognitive complexity', (metrics) => metrics.cognitive_complexity, [15, 25]],
  ['Nesting depth', (metrics) => metrics.nesting_depth, [4, 6]],
  ['Maintainability index', (metrics) => round(metrics.maintainability_index), null],
  ['Length (lines)', (metrics) => metrics.length, [50, 100]],
  ['Parameters', (metrics) => metrics.parameter_count, [5, 7]],
  ['Fan-out', (metrics) => metrics.fan_out, [8, 15]],
  ['Halstead volume', (metrics) => round(metrics.halstead_volume), null],
  ['Churn (commits)', (metrics) => (metrics.churn ? metrics.churn.total_commits : '—'), [10, 20]],
];

function round(value) {
  return typeof value === 'number' ? Math.round(value * 10) / 10 : '—';
}

function levelClass(value, thresholds) {
  if (!thresholds || typeof value !== 'number') return '';
  if (value >= thresholds[1]) return 'level-high';
  if (value >= thresholds[0]) return 'level-mid';
  return '';
}

export function renderFunctionPanel(panelEl, { store, selection, onOpenRef, onSelectFunction }) {
  clear(panelEl);
  const fn = selection.functionId && store.functions.get(selection.functionId);
  const node = selection.nodeId && store.nodes.get(selection.nodeId);
  if (fn) panelEl.append(renderChangedFunction(fn, store, onOpenRef));
  else if (node) panelEl.append(renderContextNode(node, store, onOpenRef, onSelectFunction));
  else panelEl.append(h('div', { class: 'panel-empty', text: 'Select a function in the Changes list, the code or the graph.' }));
}

function renderChangedFunction(fn, store, onOpenRef) {
  const metrics = fn.metrics;
  const findings = store.findingsForFunction(fn);
  const fanIn = `${fn.fan_in_approximate ? '≈' : ''}${fn.fan_in}`;
  return h('div', { class: 'fn-panel' },
    h('div', {},
      h('h3', {}, fn.receiver_type ? `(${fn.receiver_type}) ${fn.name}` : fn.name, fn.is_new ? h('span', { class: 'tag-new', text: 'new' }) : null),
      h('div', { class: 'subtle', text: `${fn.file}:${fn.start_line}–${fn.end_line} · +${fn.lines_added} −${fn.lines_removed}${fn.focus ? ' · ★ focus' : ''}` })),
    h('div', {},
      h('div', { class: 'section-title', text: 'Risk' }),
      h('div', { class: 'risk-meter' }, makeRiskBar(fn.risk_score), h('span', { class: 'score', text: `${Math.round(fn.risk_score)} / 100` }))),
    fn.risk_reasons && fn.risk_reasons.length
      ? h('div', {}, h('div', { class: 'section-title', text: 'Why it is risky' }),
        h('ul', { class: 'reasons' }, fn.risk_reasons.map((reason) => h('li', { text: reason }))))
      : null,
    h('div', {},
      h('div', { class: 'section-title', text: 'Metrics' }),
      metrics
        ? h('table', { class: 'metrics-table' }, h('tbody', {},
          METRIC_ROWS.map(([label, read, thresholds]) => {
            const value = read(metrics);
            return h('tr', {}, h('td', { text: label }), h('td', { class: levelClass(value, thresholds), text: value ?? '—' }));
          }),
          h('tr', {}, h('td', { text: 'Fan-in' }), h('td', { text: fanIn, title: fn.fan_in_approximate ? 'Approximate (text search)' : '' }))))
        : h('div', { class: 'subtle', text: 'Metrics unavailable for this language.' })),
    renderRefList('Callers', fn.callers, 'callers', onOpenRef),
    renderRefList('Callees', fn.callees, 'callees', onOpenRef),
    findings.length ? h('div', {}, h('div', { class: 'section-title', text: `Findings · ${findings.length}` }),
      h('ul', { class: 'reasons' }, findings.map((finding) => h('li', { text: `${finding.severity}: ${finding.title}` })))) : null);
}

function renderRefList(title, refs, kind, onOpenRef) {
  const list = h('ul', { class: 'ref-list', dataset: { refs: kind } },
    (refs || []).map((ref) => h('li', {}, h('button', {
      type: 'button',
      title: ref.id,
      dataset: { refId: ref.id },
      disabled: ref.external || !ref.file,
      onClick: () => onOpenRef(ref, kind),
    }, h('span', { class: 'ref-name', text: ref.name }),
    h('span', { class: 'ref-loc', text: ref.external || !ref.file ? 'external' : `${ref.file.split('/').pop()}:${ref.line}` })))));
  return h('div', {}, h('div', { class: 'section-title', text: `${title} · ${(refs || []).length}` }),
    refs && refs.length ? list : h('div', { class: 'subtle', text: `No ${title.toLowerCase()} found.` }));
}

function renderContextNode(node, store, onOpenRef, onSelectFunction) {
  const related = store.context.functions.filter((fn) =>
    fn.callers.some((ref) => ref.id === node.id) || fn.callees.some((ref) => ref.id === node.id));
  return h('div', { class: 'fn-panel' },
    h('div', {}, h('h3', { text: node.name }),
      h('div', { class: 'subtle', text: `${node.kind}${node.file ? ` · ${node.file}:${node.line}` : ''} · fan-in ${node.fan_in}${node.complexity ? ` · complexity ${node.complexity}` : ''}` })),
    node.file ? h('button', { type: 'button', class: 'small-button', text: 'Open definition',
      onClick: () => onOpenRef({ id: node.id, name: node.name, file: node.file, line: node.line }, 'definition') }) : null,
    h('div', {}, h('div', { class: 'section-title', text: 'Connected changed functions' }),
      h('ul', { class: 'ref-list' }, related.map((fn) => h('li', {}, h('button', { type: 'button', onClick: () => onSelectFunction(fn.id) },
        h('span', { class: 'ref-name', text: fn.name }), h('span', { class: 'ref-loc', text: `risk ${Math.round(fn.risk_score)}` })))))));
}

// ---------- Activity tab ----------

export function createActivityFeed({ feedEl, panelEl, latestButton, onFindingClick }) {
  let stickToBottom = true;
  panelEl.addEventListener('scroll', () => {
    stickToBottom = panelEl.scrollHeight - panelEl.scrollTop - panelEl.clientHeight < 40;
    if (stickToBottom) latestButton.hidden = true;
  });
  latestButton.addEventListener('click', () => {
    panelEl.scrollTop = panelEl.scrollHeight;
    latestButton.hidden = true;
  });

  function append(event) {
    const element = renderActivity(event, onFindingClick);
    if (!element) return;
    feedEl.append(element);
    if (stickToBottom) panelEl.scrollTop = panelEl.scrollHeight;
    else if (!panelEl.hidden) latestButton.hidden = false;
  }

  return { append, scrollToEnd: () => { if (stickToBottom) panelEl.scrollTop = panelEl.scrollHeight; } };
}

function renderActivity(event, onFindingClick) {
  const data = event.data || {};
  let agent = data.agent || data.source || 'lead';
  let body;
  switch (event.type) {
    case 'status':
      body = h('span', { class: 'activity-text', text: data.message || '' });
      break;
    case 'thinking': {
      const text = String(data.text || '');
      body = text.length > 160
        ? h('details', {}, h('summary', { text: truncate(text, 160) }), h('div', { class: 'full', text }))
        : h('span', { class: 'activity-text', text });
      break;
    }
    case 'tool_call':
      body = h('span', { class: 'activity-text', text: data.summary || data.tool || '' });
      break;
    case 'finding':
      body = h('span', { class: 'activity-text' }, h('span', { class: `badge sev-${data.severity}`, text: data.severity }), ' ', data.title || '');
      break;
    case 'result':
      agent = 'lead';
      body = h('span', { class: 'activity-text', text: `Review complete: ${data.verdict || 'no verdict'} · ${(data.findings || []).length} findings` });
      break;
    case 'error':
      body = h('span', { class: 'activity-text', text: data.text || 'Engine error' });
      break;
    case 'done':
      body = h('span', { class: 'activity-text', text: 'Stream finished' });
      break;
    default:
      return null;
  }
  const item = h('div', { class: `activity-item kind-${event.type}` },
    h('span', { class: 'activity-time', text: formatTime(event.time) }),
    h('div', { class: 'activity-main' },
      h('div', { class: 'activity-line' }, agentTag(agent), h('span', { class: 'activity-kind', text: event.type.replace('_', ' ') })),
      body));
  if (event.type === 'finding') item.addEventListener('click', () => onFindingClick(data.id));
  return item;
}

// ---------- Summary tab ----------

const VERDICT_LABELS = { approve: '✓ Approve', comment: '💬 Comment', request_changes: '✕ Request changes' };

export function renderSummaryPanel(panelEl, { store, onOpenLocation }) {
  clear(panelEl);
  const result = store.result;
  const sections = [];
  if (result) {
    sections.push(h('div', { class: `verdict verdict-${result.verdict}`, text: VERDICT_LABELS[result.verdict] || result.verdict || 'No verdict' }));
    const engine = [result.engine, result.model, result.cost_usd ? `$${result.cost_usd.toFixed(2)}` : null].filter(Boolean).join(' · ');
    if (engine) sections.push(h('div', { class: 'engine-line', text: engine }));
    sections.push(h('div', { class: 'summary-text' }, renderMarkdown(result.summary || '_No summary._')));
    const counts = severityCounts(result.findings || []);
    if (counts.length) sections.push(h('div', { class: 'engine-line', text: counts.map(([severity, count]) => `${count} ${severity}`).join(' · ') }));
  } else if (store.status.state === 'error') {
    sections.push(h('div', { class: 'panel-empty', text: `The review engine failed: ${store.status.message}` }));
  } else {
    sections.push(h('div', { class: 'spinner-line' }, h('span', { class: 'spinner' }), 'The AI review is still running. Findings appear as they arrive.'));
  }
  sections.push(renderConcerns(store.context.concerns || [], onOpenLocation));
  panelEl.append(h('div', { class: 'summary-panel' }, sections));
}

function renderConcerns(concerns, onOpenLocation) {
  if (!concerns.length) return h('div', {}, h('div', { class: 'section-title', text: 'Kaizen concerns' }), h('div', { class: 'subtle', text: 'None detected.' }));
  return h('div', {},
    h('div', { class: 'section-title', text: `Kaizen concerns · ${concerns.length}` }),
    concerns.map((concern) => h('div', { class: `concern sev-${concern.severity}` },
      h('div', { class: 'concern-title', text: concern.title }),
      h('div', { class: 'concern-desc', text: concern.description }),
      h('div', { class: 'concern-items' }, (concern.affected_items || []).map((item) => h('div', {},
        h('button', { type: 'button', onClick: () => onOpenLocation(item.file_path, item.line),
          text: `${item.file_path}${item.line ? `:${item.line}` : ''}${item.function_name ? ` (${item.function_name})` : ''}` })))))));
}
