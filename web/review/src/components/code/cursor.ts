// The code view's cursor lives outside React state: rows subscribe to "am I the
// cursor?" so moving it re-renders exactly two rows, not the whole file.

import { createContext, useCallback, useContext, useSyncExternalStore } from 'react';

export interface CursorStore {
  get(): string | null;
  set(key: string | null): void;
  subscribe(listener: () => void): () => void;
}

export function createCursorStore(): CursorStore {
  let current: string | null = null;
  const listeners = new Set<() => void>();
  return {
    get: () => current,
    set(key) {
      if (key === current) return;
      current = key;
      for (const listener of listeners) listener();
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

export const CursorContext = createContext<CursorStore>(createCursorStore());

export function useIsCursor(key: string): boolean {
  const cursor = useContext(CursorContext);
  const isCursor = useCallback(() => cursor.get() === key, [cursor, key]);
  return useSyncExternalStore(cursor.subscribe, isCursor, isCursor);
}
