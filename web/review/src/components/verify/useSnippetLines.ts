// Head-side lines for the verify queue's snippet, cached per file range so stepping
// back and forth is instant. prefetchSnippet warms the next finding's lines.

import { useEffect, useState } from 'react';
import { fetchFileLines } from '../../lib/api';
import type { SnippetRange } from './snippet';

type Lines = ReadonlyMap<number, string>;

const cache = new Map<string, Lines>();
const inFlight = new Map<string, Promise<Lines>>();

const cacheKey = (path: string, range: SnippetRange) => `${path}\u0000${range.from}\u0000${range.to}`;

function load(path: string, range: SnippetRange): Promise<Lines> {
  const key = cacheKey(path, range);
  const cached = cache.get(key);
  if (cached) return Promise.resolve(cached);
  const pending = inFlight.get(key);
  if (pending) return pending;
  const request = fetchFileLines(path, range.from, range.to)
    .then((slice) => {
      const lines = new Map(slice.lines.map((line) => [line.n, line.text]));
      cache.set(key, lines);
      return lines as Lines;
    })
    .finally(() => inFlight.delete(key));
  inFlight.set(key, request);
  return request;
}

export function prefetchSnippet(path: string, range: SnippetRange | null, needed: boolean): void {
  if (!range || !needed) return;
  void load(path, range).catch(() => undefined);
}

/** The lines (null until loaded, or when not needed); `failed` when the file is unavailable */
export function useSnippetLines(path: string, range: SnippetRange | null, needed: boolean): { lines: Lines | null; failed: boolean } {
  const key = range && needed ? cacheKey(path, range) : '';
  const [state, setState] = useState<{ key: string; lines: Lines | null; failed: boolean }>(() => ({ key, lines: key ? cache.get(key) ?? null : null, failed: false }));

  useEffect(() => {
    if (!key || !range) return;
    const cached = cache.get(key);
    if (cached) {
      setState({ key, lines: cached, failed: false });
      return;
    }
    let live = true;
    setState({ key, lines: null, failed: false });
    load(path, range)
      .then((lines) => { if (live) setState({ key, lines, failed: false }); })
      .catch(() => { if (live) setState({ key, lines: null, failed: true }); });
    return () => { live = false; };
    // range is captured by key
  }, [key, path]);

  if (!key) return { lines: null, failed: false };
  if (state.key !== key) return { lines: cache.get(key) ?? null, failed: false };
  return { lines: state.lines, failed: state.failed };
}
