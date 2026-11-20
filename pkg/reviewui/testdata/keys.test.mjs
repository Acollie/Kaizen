// Unit tests for the keymap parser in ../static/keys.js. Run with: node --test
// keys.js is loaded through a data: URL so this works on any node with ESM support,
// regardless of how the package treats .js files.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('../static/keys.js', import.meta.url), 'utf8');
const keys = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const { createKeyParser, BINDINGS, helpGroups, keyToken } = keys;

const action = (result) => (result.type === 'action' ? `${result.binding.action}x${result.count}` : result.type);

test('single keys run immediately with count 1', () => {
  const parser = createKeyParser();
  assert.equal(action(parser.feed('j', 0)), 'nextFunctionx1');
  assert.equal(action(parser.feed('G', 10)), 'bottomx1');
});

test('count prefix applies to the next binding', () => {
  const parser = createKeyParser();
  assert.equal(parser.feed('5', 0).type, 'pending');
  assert.equal(parser.display(), '5');
  assert.equal(action(parser.feed('j', 50)), 'nextFunctionx5');
  parser.feed('1', 100);
  parser.feed('2', 150);
  assert.equal(action(parser.feed('n', 200)), 'nextFindingx12');
});

test('multi-key sequences: gg, ]c, zR', () => {
  const parser = createKeyParser();
  assert.deepEqual(parser.feed('g', 0), { type: 'pending', display: 'g' });
  assert.equal(action(parser.feed('g', 100)), 'topx1');
  parser.feed(']', 200);
  assert.equal(action(parser.feed('c', 300)), 'nextHunkx1');
  parser.feed('z', 400);
  assert.equal(action(parser.feed('R', 500)), 'expandAllx1');
  parser.feed('3', 600);
  parser.feed(']', 650);
  assert.equal(action(parser.feed('f', 700)), 'nextFindingx3');
});

test('sequences time out after 800ms', () => {
  const parser = createKeyParser();
  parser.feed('g', 0);
  assert.equal(parser.expire(500), null, 'not expired yet');
  assert.deepEqual(parser.expire(900), { type: 'none' });
  assert.equal(parser.isPending(), false);
  parser.feed('g', 1000);
  // the second g arrives too late: it starts a fresh sequence instead of completing gg
  assert.equal(parser.feed('g', 1900).type, 'pending');
  assert.equal(action(parser.feed('g', 2000)), 'topx1');
});

test('tabs use g-prefixed keys; digits after g are keys, not counts', () => {
  const parser = createKeyParser();
  parser.feed('g', 0);
  const result = parser.feed('3', 100);
  assert.equal(result.binding.action, 'tab');
  assert.equal(result.binding.argument, 'activity');
  parser.feed('g', 200);
  assert.equal(action(parser.feed('t', 300)), 'nextTabx1');
  parser.feed('g', 400);
  assert.equal(action(parser.feed('T', 500)), 'prevTabx1');
});

test('a bare count with no motion shows as pending, then times out', () => {
  const parser = createKeyParser();
  assert.equal(parser.feed('2', 0).type, 'pending');
  assert.equal(parser.display(), '2');
  assert.deepEqual(parser.expire(900), { type: 'none' });
  assert.equal(parser.display(), '');
});

test('unknown keys reset, and an aborted prefix retries the new key', () => {
  const parser = createKeyParser();
  assert.equal(parser.feed('q', 0).type, 'none');
  parser.feed('g', 10);
  assert.equal(action(parser.feed('j', 20)), 'nextFunctionx1');
  assert.equal(parser.isPending(), false);
});

test('Esc clears pending keys and reports escape', () => {
  const parser = createKeyParser();
  parser.feed('4', 0);
  parser.feed('z', 10);
  assert.equal(action(parser.feed('Esc', 20)), 'escapex1');
  assert.equal(parser.display(), '');
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
    assert.ok(labels.includes(label), `missing ${label}`);
  }
});

test('keyToken normalises keyboard events', () => {
  assert.equal(keyToken({ key: 'd', ctrlKey: true }), 'C-d');
  assert.equal(keyToken({ key: ' ' }), 'Space');
  assert.equal(keyToken({ key: 'Escape' }), 'Esc');
  assert.equal(keyToken({ key: 'Shift' }), null);
  assert.equal(keyToken({ key: 'j', metaKey: true }), null);
});
