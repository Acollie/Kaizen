// Finding cards shown inline in the code view.

import { h, agentTag } from './dom.js';
import { renderMarkdown } from './markdown.js';

export function renderFindingCard(finding, store, { onFocus, onShowInContext }) {
  const card = h('article', { class: 'finding-card', dataset: { findingId: finding.id }, tabindex: '-1' });
  card.addEventListener('click', (event) => {
    if (!event.target.closest('input, a, button, label')) onFocus(card);
  });
  card._onShowInContext = onShowInContext;
  updateFindingCard(card, finding, store);
  return card;
}

export function updateFindingCard(card, finding, store) {
  if (!finding) return;
  const included = store.included.has(finding.id);
  card.className = `finding-card sev-${finding.severity}${included ? '' : ' excluded'}${card.classList.contains('cursor') ? ' cursor' : ''}`;
  const lineText = finding.line ? (finding.end_line && finding.end_line !== finding.line ? `L${finding.line}–${finding.end_line}` : `L${finding.line}`) : 'file';

  const checkbox = h('input', { type: 'checkbox', 'aria-label': 'Include in post' });
  checkbox.checked = included;
  checkbox.addEventListener('change', () => store.setIncluded(finding.id, checkbox.checked));

  const outsideDiff = finding.line && !store.isInlineFinding(finding);
  card.replaceChildren(...[
    h('div', { class: 'finding-head' },
      h('span', { class: 'sev-label', text: finding.severity }),
      finding.category ? h('span', { class: 'cat-label', text: finding.category }) : null,
      h('span', { class: 'line-ref', text: lineText }),
      h('h4', { class: 'finding-title', text: finding.title || '(untitled finding)' })),
    h('div', { class: 'finding-body' }, renderMarkdown(finding.body)),
    finding.suggestion ? renderSuggestion(finding.suggestion) : null,
    h('div', { class: 'finding-foot' },
      h('label', {}, checkbox, 'Include in post'),
      outsideDiff ? h('button', {
        type: 'button', class: 'small-button', text: `Show line ${finding.line}`,
        onClick: () => card._onShowInContext && card._onShowInContext(finding.line),
      }) : null,
      typeof finding.confidence === 'number' && finding.confidence > 0
        ? h('span', { class: 'confidence', title: 'Model confidence', text: `${Math.round(finding.confidence * 100)}%` }) : null,
      h('span', { class: 'source' }, agentTag(finding.source || 'lead'))),
  ].filter(Boolean));
}

function renderSuggestion(suggestion) {
  const lines = String(suggestion).replace(/\n$/, '').split('\n');
  return h('div', { class: 'suggestion' },
    h('div', { class: 'suggestion-title', text: 'Suggested change' }),
    h('pre', {}, lines.map((line) => h('span', { class: 's-add', text: `+ ${line}` }))));
}
