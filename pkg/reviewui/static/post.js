// "Post review" modal: editable summary, the selected findings split into inline
// comments vs. review-body items, and the POST to /api/post.

import { postReview } from './api.js';
import { h, clear, externalLink, severityRank } from './dom.js';

export function createPostModal({ store, onClose }) {
  const dialog = document.getElementById('post-modal');
  const summaryEl = document.getElementById('post-summary');
  const findingsEl = document.getElementById('post-findings');
  const outcomeEl = document.getElementById('post-outcome');
  const noteEl = document.getElementById('post-note');
  const confirmButton = document.getElementById('post-confirm');
  let summaryEdited = false;
  let posting = false;

  summaryEl.addEventListener('input', () => { summaryEdited = true; });
  document.getElementById('post-cancel').addEventListener('click', () => dialog.close());
  document.getElementById('post-cancel-x').addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => onClose && onClose());
  confirmButton.addEventListener('click', submit);
  store.on('included', () => { if (dialog.open) renderFindings(); });

  function open() {
    if (!summaryEdited) summaryEl.value = store.result ? store.result.summary || '' : '';
    outcomeEl.hidden = true;
    renderFindings();
    dialog.showModal();
    summaryEl.focus();
  }

  function sortedFindings() {
    return store.findingsList().sort((left, right) =>
      severityRank(left.severity) - severityRank(right.severity) || left.file.localeCompare(right.file) || (left.line || 0) - (right.line || 0));
  }

  function renderFindings() {
    clear(findingsEl);
    const all = sortedFindings();
    if (!all.length) {
      findingsEl.append(h('div', { class: 'subtle', text: 'No findings yet. You can still post the summary.' }));
    }
    const inline = all.filter((finding) => store.isInlineFinding(finding));
    const body = all.filter((finding) => !store.isInlineFinding(finding));
    if (inline.length) findingsEl.append(group('Inline comments', 'on a line in the diff', inline));
    if (body.length) findingsEl.append(group('In the review body', 'file-level or outside the diff', body));
    const selected = all.filter((finding) => store.included.has(finding.id));
    const selectedInline = selected.filter((finding) => store.isInlineFinding(finding)).length;
    noteEl.textContent = `${selected.length} selected: ${selectedInline} inline, ${selected.length - selectedInline} in body`;
  }

  function group(title, hint, findings) {
    return h('div', {},
      h('div', { class: 'post-group-title' }, `${title} · ${findings.length}`, h('span', { class: 'hint', text: hint })),
      findings.map((finding) => {
        const checkbox = h('input', { type: 'checkbox', 'aria-label': `Include ${finding.title}` });
        checkbox.checked = store.included.has(finding.id);
        checkbox.addEventListener('change', () => store.setIncluded(finding.id, checkbox.checked));
        return h('label', { class: 'post-item' }, checkbox,
          h('span', { class: `sev-label sev-${finding.severity}`, text: finding.severity }),
          h('span', { class: 'title', text: finding.title, title: finding.title }),
          h('span', { class: 'loc', text: `${finding.file.split('/').pop()}${finding.line ? `:${finding.line}` : ''}` }));
      }));
  }

  async function submit() {
    if (posting) return;
    posting = true;
    confirmButton.disabled = true;
    confirmButton.textContent = 'Posting…';
    outcomeEl.hidden = true;
    try {
      const findings = sortedFindings().filter((finding) => store.included.has(finding.id));
      const result = await postReview({ summary: summaryEl.value, findings });
      showOutcome(true, h('span', {}, 'Review posted: ', externalLink(result.url, result.url),
        ` (${result.inline_comments} inline, ${result.body_findings} in body)`));
      confirmButton.textContent = 'Posted';
    } catch (error) {
      showOutcome(false, `Posting failed: ${error.message}`);
      confirmButton.disabled = false;
      confirmButton.textContent = 'Retry';
    } finally {
      posting = false;
    }
  }

  function showOutcome(ok, content) {
    outcomeEl.className = `post-outcome ${ok ? 'ok' : 'err'}`;
    outcomeEl.replaceChildren(content);
    outcomeEl.hidden = false;
  }

  return { open, isOpen: () => dialog.open, close: () => dialog.close() };
}
