// A tiny, safe Markdown renderer for PR and AI text. It produces React elements from
// parsed tokens (never HTML strings), so no input can inject markup. Supported:
// paragraphs, "- " lists, fenced code, `code`, **bold**, *italic* / _italic_ and
// [links](https://...) (http/https only).

import { Fragment, type ReactNode } from 'react';
import { isSafeHttpUrl } from '../../lib/format';
import './Markdown.css';

const INLINE_PATTERN = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\*[^*\s][^*]*\*)|(\b_[^_\s][^_]*_\b)|(\[[^\]]+\]\([^)\s]+\))/g;

type Block =
  | { type: 'code'; text: string }
  | { type: 'list'; items: string[] }
  | { type: 'paragraph'; text: string };

export function Markdown({ source, className }: { source: string | undefined | null; className?: string }) {
  const blocks = splitBlocks(String(source ?? ''));
  return (
    <div className={className ? `md ${className}` : 'md'}>
      {blocks.map((block, index) => <MarkdownBlock key={index} block={block} />)}
    </div>
  );
}

function MarkdownBlock({ block }: { block: Block }) {
  if (block.type === 'code') return <pre><code>{block.text}</code></pre>;
  if (block.type === 'list') return <ul>{block.items.map((item, index) => <li key={index}>{renderInline(item)}</li>)}</ul>;
  return <p>{renderInline(block.text)}</p>;
}

/** Split into fenced code, list and paragraph blocks */
export function splitBlocks(source: string): Block[] {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const blocks: Block[] = [];
  let index = 0;
  const at = (position: number) => lines[position] ?? '';
  while (index < lines.length) {
    const line = at(index);
    if (line.trimStart().startsWith('```')) {
      const code: string[] = [];
      index++;
      while (index < lines.length && !at(index).trimStart().startsWith('```')) code.push(at(index++));
      index++;
      blocks.push({ type: 'code', text: code.join('\n') });
    } else if (/^\s*[-*] /.test(line)) {
      const items: string[] = [];
      while (index < lines.length && /^\s*[-*] /.test(at(index))) items.push(at(index++).replace(/^\s*[-*] /, ''));
      blocks.push({ type: 'list', items });
    } else if (line.trim() === '') {
      index++;
    } else {
      const paragraph: string[] = [];
      while (index < lines.length && at(index).trim() !== '' && !/^\s*([-*] |```)/.test(at(index))) paragraph.push(at(index++));
      blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
    }
  }
  return blocks;
}

/** Inline spans as React nodes */
export function renderInline(text: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  let cursor = 0;
  let key = 0;
  for (const match of text.matchAll(INLINE_PATTERN)) {
    const start = match.index ?? 0;
    if (start > cursor) nodes.push(<Fragment key={key++}>{text.slice(cursor, start)}</Fragment>);
    nodes.push(<Fragment key={key++}>{renderSpan(match[0])}</Fragment>);
    cursor = start + match[0].length;
  }
  if (cursor < text.length) nodes.push(<Fragment key={key++}>{text.slice(cursor)}</Fragment>);
  return nodes;
}

function renderSpan(token: string): ReactNode {
  if (token.startsWith('`')) return <code>{token.slice(1, -1)}</code>;
  if (token.startsWith('**')) return <strong>{renderInline(token.slice(2, -2))}</strong>;
  if (token.startsWith('[')) {
    const split = token.indexOf('](');
    const label = token.slice(1, split);
    const href = token.slice(split + 2, -1);
    if (!isSafeHttpUrl(href)) return label;
    return <a href={href} target="_blank" rel="noopener noreferrer">{renderInline(label)}</a>;
  }
  return <em>{renderInline(token.slice(1, -1))}</em>;
}
