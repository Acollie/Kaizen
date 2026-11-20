// Keymap parser tests (ported from the node --test suite for keys.js)
import { expect, test } from 'vitest';
import { BINDINGS, createKeyParser, helpGroups, keyToken, type ParseResult } from './keymap';

const action = (result: ParseResult | null) => (result && result.type === 'action' ? `${(result as { binding: { action: string } }).binding.action}x${result.count}` : (result ? result.type : 'null'));

test('single keys run immediately with count 1', () => {
  const parser = createKeyParser();
  expect(action(parser.feed('j', 0))).toEqual('nextFunctionx1');
  expect(action(parser.feed('G', 10))).toEqual('bottomx1');
});

test('count prefix applies to the next binding', () => {
  const parser = createKeyParser();
  expect(parser.feed('5', 0).type).toEqual('pending');
  expect(parser.display()).toEqual('5');
  expect(action(parser.feed('j', 50))).toEqual('nextFunctionx5');
  parser.feed('1', 100);
  parser.feed('2', 150);
  expect(action(parser.feed('n', 200))).toEqual('nextFindingx12');
});

test('multi-key sequences: gg, ]c, zR', () => {
  const parser = createKeyParser();
  expect(parser.feed('g', 0)).toEqual({ type: 'pending', display: 'g' });
  expect(action(parser.feed('g', 100))).toEqual('topx1');
  parser.feed(']', 200);
  expect(action(parser.feed('c', 300))).toEqual('nextHunkx1');
  parser.feed('z', 400);
  expect(action(parser.feed('R', 500))).toEqual('expandAllx1');
  parser.feed('3', 600);
  parser.feed(']', 650);
  expect(action(parser.feed('f', 700))).toEqual('nextFindingx3');
});

test('sequences time out after 800ms', () => {
  const parser = createKeyParser();
  parser.feed('g', 0);
  expect(parser.expire(500)).toEqual(null);
  expect(parser.expire(900)).toEqual({ type: 'none' });
  expect(parser.isPending()).toEqual(false);
  parser.feed('g', 1000);
  // the second g arrives too late: it starts a fresh sequence instead of completing gg
  expect(parser.feed('g', 1900).type).toEqual('pending');
  expect(action(parser.feed('g', 2000))).toEqual('topx1');
});

test('tabs use g-prefixed keys; digits after g are keys, not counts', () => {
  const parser = createKeyParser();
  parser.feed('g', 0);
  const result = parser.feed('4', 100);
  expect((result as { binding: { action: string } }).binding.action).toEqual('tab');
  expect((result as { binding: { argument?: string } }).binding.argument).toEqual('activity');
  parser.feed('g', 150);
  expect((parser.feed('1', 160) as { binding: { argument?: string } }).binding.argument).toEqual('review');
  parser.feed('g', 200);
  expect(action(parser.feed('t', 300))).toEqual('nextTabx1');
  parser.feed('g', 400);
  expect(action(parser.feed('T', 500))).toEqual('prevTabx1');
});

test('a bare count with no motion shows as pending, then times out', () => {
  const parser = createKeyParser();
  expect(parser.feed('2', 0).type).toEqual('pending');
  expect(parser.display()).toEqual('2');
  expect(parser.expire(900)).toEqual({ type: 'none' });
  expect(parser.display()).toEqual('');
});

test('unknown keys reset, and an aborted prefix retries the new key', () => {
  const parser = createKeyParser();
  expect(parser.feed('q', 0).type).toEqual('none');
  parser.feed('g', 10);
  expect(action(parser.feed('j', 20))).toEqual('nextFunctionx1');
  expect(parser.isPending()).toEqual(false);
});

test('Esc clears pending keys and reports escape', () => {
  const parser = createKeyParser();
  parser.feed('4', 0);
  parser.feed('z', 10);
  expect(action(parser.feed('Esc', 20))).toEqual('escapex1');
  expect(parser.display()).toEqual('');
});

test('no binding is a strict prefix of another (sequences are unambiguous)', () => {
  for (const binding of BINDINGS) {
    for (const other of BINDINGS) {
      if (binding === other || other.keys.length <= binding.keys.length) continue;
      const prefix = other.keys.slice(0, binding.keys.length);
      assert.notDeepEqual(prefix, binding.keys, `${binding.keys.join('')} shadows ${other.keys.join('')}`);
    }
  }
});

test('help groups cover every binding', () => {
  const labels = helpGroups().flatMap((group) => group.rows.flatMap((row) => row.labels));
  for (const binding of BINDINGS) {
    const label = binding.keys.map((key) => (key.startsWith('C-') ? `Ctrl-${key.slice(2)}` : key)).join('');
    expect(labels.includes(label)).toBe(true);
  }
});

test('keyToken normalises keyboard events', () => {
  expect(keyToken({ key: 'd', ctrlKey: true, metaKey: false, altKey: false })).toEqual('C-d');
  expect(keyToken({ key: ' ', ctrlKey: false, metaKey: false, altKey: false })).toEqual('Space');
  expect(keyToken({ key: 'Escape', ctrlKey: false, metaKey: false, altKey: false })).toEqual('Esc');
  expect(keyToken({ key: 'Shift', ctrlKey: false, metaKey: false, altKey: false })).toEqual(null);
  expect(keyToken({ key: 'j', metaKey: true, ctrlKey: false, altKey: false })).toEqual(null);
});
