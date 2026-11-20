// "Review" tab: the AI review front and centre. Verdict and summary once the review
// finishes; findings grouped by what to do about them (must fix, should fix, consider,
// good) with near-duplicates from different reviewer agents folded together; and,
// while the review runs, live progress plus Kaizen's own focus list so there is
// something useful to read immediately.

import { h, clear, agentTag, severityRank, splitPath } from './dom.js';
import { renderMarkdown } from './markdown.js';

const GROUPS = [
  { severity: 'critical', title: 'Must fix', glyph: '▲' },
  { severity: 'warning', title: 'Should fix', glyph: '▲' },
  { severity: 'suggestion', title: 'Consider', glyph: '●' },
  { severity: 'praise', title: 'Good', glyph: '✓' },
];
const VERDICTS = {
  request_changes: { label: 'Changes requested', className: 'verdict-request_changes' },
  comment: { label: 'Comments', className: 'verdict-comment' },
  approve: { label: 'Approved', className: 'verdict-approve' },
};
const SIMILAR_LINE_DISTANCE = 3;

/**
 * Groups findings that point at (nearly) the same lines of the same file with the same
 * category: different reviewer agents often report one problem in different words.
 * The highest-severity, highest-confidence finding of each cluster leads it.
 */
export function clusterFindings(findings) {
  const ordered = [...findings].sort((left, right) =>
    severityRank(left.severity) - severityRank(right.severity) || (right.confidence || 0) - (left.confidence || 0));
  const clusters = [];
  for (const finding of ordered) {
    const home = clusters.find((cluster) => isSimilar(cluster.lead, finding));
    if (home) home.similar.push(finding);
    else clusters.push({ lead: finding, similar: [] });
  }
  return clusters;
}

function isSimilar(left, right) {
  if (left.file !== right.file || left.category !== right.category) return false;
  if (!left.line || !right.line) return left.line === right.line;
  return Math.abs(left.line - right.line) <= SIMILAR_LINE_DISTANCE;
}

export function renderReviewPanel(panelEl, { store, currentFindingId, onOpenFinding, onSelectFunction }) {
  const scrollTop = panelEl.scrollTop;
  clear(panelEl);
  const findings = store.findingsList();
  const container = h('div', { class: 'review-panel' });
  container.append(renderHeadline(store, findings));
  if (findings.length) {
    for (const group of GROUPS) {
      const members = findings.filter((finding) => finding.severity === group.severity);
      if (members.length) container.append(renderGroup(group, members, { currentFindingId, onOpenFinding }));
    }
  }
  if (store.result && store.result.summary) container.append(renderSummary(store.result.summary));
  container.append(renderFocusList(store, onSelectFunction));
  panelEl.append(container);
  panelEl.scrollTop = scrollTop;
}

function renderHeadline(store, findings) {
  const { result, status } = store;
  if (result) {
    const verdict = VERDICTS[result.verdict] || { label: result.verdict || 'Reviewed', className: '' };
    const meta = [result.engine, result.cost_usd ? `$${result.cost_usd.toFixed(2)}` : null].filter(Boolean).join(' · ');
    return h('div', { class: `review-headline ${verdict.className}` },
      h('div', { class: 'verdict-label', text: verdict.label }),
      h('div', { class: 'verdict-counts', text: countsSentence(findings) }),
      meta ? h('div', { class: 'verdict-meta', text: meta }) : null);
  }
  if (status.state === 'error') {
    return h('div', { class: 'review-headline verdict-error' },
      h('div', { class: 'verdict-label', text: 'Review failed' }),
      h('div', { class: 'verdict-counts', text: status.message || 'The review engine stopped with an error.' }));
  }
  if (status.state === 'done') {
    return h('div', { class: 'review-headline' },
      h('div', { class: 'verdict-label', text: findings.length ? 'Review stopped' : 'Kaizen analysis' }),
      h('div', { class: 'verdict-counts', text: status.message || '' }));
  }
  return h('div', { class: 'review-headline running' },
    h('div', { class: 'verdict-label' }, h('span', { class: 'spinner' }), 'Claude is reviewing'),
    h('div', { class: 'verdict-counts', text: findings.length ? `${countsSentence(findings)} so far` : 'Findings appear here as the reviewers report them.' }),
    status.agent ? h('div', { class: 'verdict-meta' }, agentTag(status.agent), ` ${status.message || ''}`) : null);
}

function countsSentence(findings) {
  if (!findings.length) return 'No findings';
  const parts = GROUPS
    .map((group) => [group, findings.filter((finding) => finding.severity === group.severity).length])
    .filter(([, count]) => count > 0)
    .map(([group, count]) => `${count} ${group.title.toLowerCase()}`);
  return parts.join(' · ');
}

function renderGroup(group, members, { currentFindingId, onOpenFinding }) {
  const clusters = clusterFindings(members);
  return h('section', { class: `review-group sev-${group.severity}` },
    h('h3', { class: 'review-group-title' },
      h('span', { class: 'group-glyph', text: group.glyph }),
      group.title,
      h('span', { class: 'group-count', text: String(clusters.length) })),
    clusters.map((cluster) => renderFindingItem(cluster, { currentFindingId, onOpenFinding })));
}

function renderFindingItem({ lead, similar }, { currentFindingId, onOpenFinding }) {
  const { base } = splitPath(lead.file || '');
  const location = lead.line ? `${base}:${lead.line}` : base || 'PR';
  const isCurrent = [lead, ...similar].some((finding) => finding.id === currentFindingId);
  const agents = [...new Set([lead, ...similar].map((finding) => finding.source || 'lead'))];
  const item = h('button', {
    type: 'button',
    class: `review-item sev-${lead.severity}${isCurrent ? ' current' : ''}`,
    title: `${lead.file}${lead.line ? `:${lead.line}` : ''}`,
    dataset: { findingId: lead.id },
    onClick: () => onOpenFinding(lead.id),
  },
  h('div', { class: 'review-item-top' },
    h('span', { class: 'review-location', text: location }),
    lead.category ? h('span', { class: 'review-category', text: lead.category }) : null,
    typeof lead.confidence === 'number' && lead.confidence > 0
      ? h('span', { class: 'review-confidence', text: `${Math.round(lead.confidence * 100)}%` }) : null),
  h('div', { class: 'review-title', text: lead.title || '(untitled finding)' }),
  h('div', { class: 'review-item-foot' },
    agents.map((agent) => agentTag(agent)),
    similar.length ? h('span', { class: 'review-similar', text: `+${similar.length} similar` }) : null));
  return item;
}

function renderSummary(summary) {
  const body = h('div', { class: 'review-summary-body' }, renderMarkdown(summary));
  const details = h('details', { class: 'review-summary' },
    h('summary', { text: 'Reviewer summary' }),
    body);
  return details;
}

function renderFocusList(store, onSelectFunction) {
  const focus = store.context.functions.filter((fn) => fn.focus);
  const riskyFiles = store.context.files.filter((file) => (file.risk_reasons || []).length && !store.functionsByFile.has(file.path));
  if (!focus.length && !riskyFiles.length) return null;
  return h('section', { class: 'review-group kaizen-focus' },
    h('h3', { class: 'review-group-title' },
      h('span', { class: 'group-glyph', text: '★' }),
      'Kaizen focus',
      h('span', { class: 'group-hint', text: 'metrics-based, before AI' })),
    focus.map((fn) => h('button', {
      type: 'button',
      class: 'review-item focus-item',
      onClick: () => onSelectFunction(fn.id),
    },
    h('div', { class: 'review-item-top' },
      h('span', { class: 'review-location', text: `${splitPath(fn.file).base}:${fn.start_line}` }),
      h('span', { class: 'review-confidence', text: `risk ${Math.round(fn.risk_score)}` })),
    h('div', { class: 'review-title', text: fn.name }),
    fn.risk_reasons && fn.risk_reasons.length ? h('div', { class: 'review-reason', text: fn.risk_reasons[0] }) : null)),
    riskyFiles.map((file) => h('div', { class: 'review-item focus-item static' },
      h('div', { class: 'review-item-top' },
        h('span', { class: 'review-location', text: splitPath(file.path).base }),
        h('span', { class: 'review-confidence', text: `risk ${Math.round(file.risk_score)}` })),
      h('div', { class: 'review-reason', text: file.risk_reasons[0] }))));
}
