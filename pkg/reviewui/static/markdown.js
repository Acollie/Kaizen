// A tiny, safe Markdown renderer. It builds DOM nodes directly (text goes in via
// text nodes), so no input can produce markup. Supported: paragraphs, "- " lists,
// fenced code, `code`, **bold**, *italic* / _italic_, and [links](https://...).

import { h, isSafeHttpUrl } from './dom.js';

const INLINE_PATTERN = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\*[^*\s][^*]*\*)|(\b_[^_\s][^_]*_\b)|(\[[^\]]+\]\([^)\s]+\))/g;

export function renderMarkdown(source) {
  const container = h('div', { class: 'md' });
  for (const block of splitBlocks(String(source || ''))) {
    container.append(renderBlock(block));
  }
  return container;
}

/** Split into fenced code, list and paragraph blocks */
function splitBlocks(source) {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const blocks = [];
  let index = 0;
  while (index < lines.length) {
    const line = lines[index];
    if (line.trimStart().startsWith('```')) {
      const code = [];
      index++;
      while (index < lines.length && !lines[index].trimStart().startsWith('```')) code.push(lines[index++]);
      index++;
      blocks.push({ type: 'code', text: code.join('\n') });
    } else if (/^\s*[-*] /.test(line)) {
      const items = [];
      while (index < lines.length && /^\s*[-*] /.test(lines[index])) items.push(lines[index++].replace(/^\s*[-*] /, ''));
      blocks.push({ type: 'list', items });
    } else if (line.trim() === '') {
      index++;
    } else {
      const paragraph = [];
      while (index < lines.length && lines[index].trim() !== '' && !/^\s*([-*] |```)/.test(lines[index])) paragraph.push(lines[index++]);
      blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
    }
  }
  return blocks;
}

function renderBlock(block) {
  if (block.type === 'code') return h('pre', {}, h('code', { text: block.text }));
  if (block.type === 'list') return h('ul', {}, block.items.map((item) => h('li', {}, renderInline(item))));
  return h('p', {}, renderInline(block.text));
}

/** Inline spans: returns an array of nodes */
export function renderInline(text) {
  const nodes = [];
  let cursor = 0;
  for (const match of text.matchAll(INLINE_PATTERN)) {
    if (match.index > cursor) nodes.push(document.createTextNode(text.slice(cursor, match.index)));
    nodes.push(renderSpan(match[0]));
    cursor = match.index + match[0].length;
  }
  if (cursor < text.length) nodes.push(document.createTextNode(text.slice(cursor)));
  return nodes;
}

function renderSpan(token) {
  if (token.startsWith('`')) return h('code', { text: token.slice(1, -1) });
  if (token.startsWith('**')) return h('strong', {}, renderInline(token.slice(2, -2)));
  if (token.startsWith('[')) return renderLink(token);
  return h('em', {}, renderInline(token.slice(1, -1)));
}

function renderLink(token) {
  const split = token.indexOf('](');
  const label = token.slice(1, split);
  const href = token.slice(split + 2, -1);
  if (!isSafeHttpUrl(href)) return document.createTextNode(label);
  return h('a', { href, target: '_blank', rel: 'noopener noreferrer' }, renderInline(label));
}
