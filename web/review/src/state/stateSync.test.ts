// Server-side persistence on the client: debounced PUTs, the unload flush, and which
// copy wins on load (server over localStorage).
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStateSync, SAVE_DELAY_MS, type StateRemote } from './stateSync';
import { bindPersistence, createReviewStore } from './store';
import { makeContext, makeFinding } from '../components/post/testFixtures';
import { reviewStorageKey, saveReview, serializeReview, type KeyValueStorage } from './persistence';
import { EMPTY_VIEWED } from './viewing';

function memoryStorage(): KeyValueStorage & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return { data, getItem: (key) => data.get(key) ?? null, setItem: (key, value) => { data.set(key, value); } };
}

function fakeRemote(options: { fail?: boolean } = {}) {
  const saves: unknown[] = [];
  const unloads: unknown[] = [];
  const remote: StateRemote = {
    save: vi.fn(async (document: unknown) => {
      saves.push(document);
      if (options.fail) throw new Error('offline');
    }),
    saveOnUnload: vi.fn((document: unknown) => { unloads.push(document); }),
  };
  return { remote, saves, unloads };
}

function storeWithFindings() {
  const store = createReviewStore(makeContext());
  store.getState().applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'w', severity: 'warning' }) } as never);
  store.getState().applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 's', severity: 'suggestion' }) } as never);
  return store;
}

beforeEach(() => { vi.useFakeTimers(); });
afterEach(() => { vi.useRealTimers(); });

describe('createStateSync', () => {
  it('debounces bursts of changes into one PUT of the latest document', async () => {
    const { remote, saves } = fakeRemote();
    const sync = createStateSync(remote);
    sync.schedule({ n: 1 });
    vi.advanceTimersByTime(SAVE_DELAY_MS - 100);
    sync.schedule({ n: 2 });
    vi.advanceTimersByTime(SAVE_DELAY_MS - 100);
    expect(saves).toEqual([]);
    sync.schedule({ n: 3 });
    vi.advanceTimersByTime(SAVE_DELAY_MS);
    expect(saves).toEqual([{ n: 3 }]);
  });

  it('flushes a pending change with a keepalive save on unload', () => {
    const { remote, saves, unloads } = fakeRemote();
    const sync = createStateSync(remote);
    sync.schedule({ n: 1 });
    sync.flush();
    expect(unloads).toEqual([{ n: 1 }]);
    vi.advanceTimersByTime(SAVE_DELAY_MS * 2);
    expect(saves).toEqual([]);
  });

  it('sends nothing on unload once the server confirmed the save', async () => {
    const { remote, unloads } = fakeRemote();
    const sync = createStateSync(remote);
    sync.schedule({ n: 1 });
    await vi.advanceTimersByTimeAsync(SAVE_DELAY_MS);
    sync.flush();
    expect(unloads).toEqual([]);
  });

  it('re-sends on unload when the debounced save failed', async () => {
    const { remote, saves, unloads } = fakeRemote({ fail: true });
    const sync = createStateSync(remote);
    sync.schedule({ n: 1 });
    await vi.advanceTimersByTimeAsync(SAVE_DELAY_MS);
    expect(saves).toEqual([{ n: 1 }]);
    sync.flush();
    expect(unloads).toEqual([{ n: 1 }]);
  });
});

describe('bindPersistence with the server', () => {
  it('saves every change to localStorage and, debounced, to the server', async () => {
    const storage = memoryStorage();
    const { remote, saves } = fakeRemote();
    const store = storeWithFindings();
    bindPersistence(store, storage, { saved: null, sync: createStateSync(remote) });
    store.getState().acceptFinding('w', 'checked');
    store.getState().markViewed('file', 'pkg/a.go');
    expect(storage.data.has(reviewStorageKey(store.getState().context.pr))).toBe(true);
    expect(saves).toEqual([]);
    await vi.advanceTimersByTimeAsync(SAVE_DELAY_MS);
    expect(saves).toHaveLength(1);
    expect(saves[0]).toMatchObject({
      version: 1,
      key: reviewStorageKey(store.getState().context.pr),
      triage: { w: { state: 'accepted', note: 'checked' } },
      viewed: { files: ['pkg/a.go'], functions: [] },
    });
  });

  it('restores from the server on a new origin with empty localStorage (a restart)', async () => {
    const { remote, saves } = fakeRemote();
    const first = storeWithFindings();
    bindPersistence(first, memoryStorage(), { saved: null, sync: createStateSync(remote) });
    first.getState().dismissFinding('s', 'False positive', '');
    await vi.advanceTimersByTimeAsync(SAVE_DELAY_MS);

    const restarted = storeWithFindings();
    bindPersistence(restarted, memoryStorage(), { saved: saves.at(-1), sync: createStateSync(fakeRemote().remote) });
    expect(restarted.getState().triage).toEqual({ s: { state: 'dismissed', reason: 'False positive' } });
  });

  it('prefers the server copy over localStorage', () => {
    const storage = memoryStorage();
    const store = storeWithFindings();
    const key = reviewStorageKey(store.getState().context.pr);
    saveReview(storage, key, { triage: { w: { state: 'dismissed' } }, viewed: EMPTY_VIEWED, comments: {}, revealed: false });
    const serverDoc = serializeReview(key, { triage: { w: { state: 'accepted' } }, viewed: EMPTY_VIEWED, comments: {}, revealed: false });
    bindPersistence(store, storage, { saved: serverDoc, sync: createStateSync(fakeRemote().remote) });
    expect(store.getState().triage).toEqual({ w: { state: 'accepted' } });
  });

  it('falls back to localStorage from an older run and copies it to the server', async () => {
    const storage = memoryStorage();
    const store = storeWithFindings();
    const key = reviewStorageKey(store.getState().context.pr);
    // An older run wrote no key field: still accepted
    storage.setItem(key, JSON.stringify({ version: 1, triage: { w: { state: 'accepted' } }, viewed: { functions: [], files: [] }, comments: {} }));
    const { remote, saves } = fakeRemote();
    bindPersistence(store, storage, { saved: null, sync: createStateSync(remote) });
    expect(store.getState().triage).toEqual({ w: { state: 'accepted' } });
    await vi.advanceTimersByTimeAsync(SAVE_DELAY_MS);
    expect(saves).toHaveLength(1);
    expect(saves[0]).toMatchObject({ key, triage: { w: { state: 'accepted' } } });
  });

  it('ignores a server document saved for another PR head', () => {
    const store = storeWithFindings();
    const stale = serializeReview('kaizen-review:o/r#7@old-sha', { triage: { w: { state: 'accepted' } }, viewed: EMPTY_VIEWED, comments: {}, revealed: false });
    bindPersistence(store, memoryStorage(), { saved: stale, sync: createStateSync(fakeRemote().remote) });
    expect(store.getState().triage).toEqual({});
  });

  it('does not PUT on load when there is nothing to copy', async () => {
    const { remote, saves } = fakeRemote();
    bindPersistence(storeWithFindings(), memoryStorage(), { saved: null, sync: createStateSync(remote) });
    await vi.advanceTimersByTimeAsync(SAVE_DELAY_MS * 2);
    expect(saves).toEqual([]);
  });
});
