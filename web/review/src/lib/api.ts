// API client for the local kaizen review server. The session token comes from ?t= on
// the page URL and is sent on every call (query param for GET/SSE, header for POST).

import type { FileSlice, PostRequest, PostResult, ReviewContext, ReviewEvent } from '../types';

const TOKEN_HEADER = 'X-Kaizen-Token';

export function sessionToken(): string {
  return new URLSearchParams(window.location.search).get('t') ?? '';
}

function withToken(path: string): string {
  const url = new URL(path, window.location.origin);
  url.searchParams.set('t', sessionToken());
  return url.pathname + url.search;
}

async function readJSON<T>(response: Response): Promise<T> {
  const body = (await response.json().catch(() => ({}))) as { error?: string };
  if (!response.ok) throw new Error(body.error ?? `${response.status} ${response.statusText}`);
  return body as T;
}

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(withToken(path), {
    headers: { [TOKEN_HEADER]: sessionToken(), Accept: 'application/json' },
    credentials: 'same-origin',
  });
  return readJSON<T>(response);
}

export function fetchContext(): Promise<ReviewContext> {
  return getJSON<ReviewContext>('/api/context');
}

/** Head-side lines from..to (1-based, inclusive) of a PR file or a caller/callee's file */
export function fetchFileLines(path: string, from: number, to: number): Promise<FileSlice> {
  const params = new URLSearchParams({ path, from: String(from), to: String(to) });
  return getJSON<FileSlice>(`/api/file?${params}`);
}

export async function postReview(request: PostRequest): Promise<PostResult> {
  const response = await fetch('/api/post', {
    method: 'POST',
    headers: { [TOKEN_HEADER]: sessionToken(), 'Content-Type': 'application/json', Accept: 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(request),
  });
  return readJSON<PostResult>(response);
}

export interface EventStreamHandlers {
  onEvent(event: ReviewEvent): void;
  onOpen?(): void;
  /** closed is true when the browser gave up (the server ended the stream) */
  onError?(closed: boolean): void;
}

/**
 * Subscribe to the SSE stream. EventSource reconnects by itself and sends
 * Last-Event-ID; events are deduplicated by seq. Returns a function that closes it.
 */
export function openEventStream({ onEvent, onOpen, onError }: EventStreamHandlers): () => void {
  let lastSeq = 0;
  const source = new EventSource(withToken('/api/events'));
  source.onopen = () => onOpen?.();
  source.onerror = () => onError?.(source.readyState === EventSource.CLOSED);
  source.onmessage = (message: MessageEvent<string>) => {
    let event: ReviewEvent;
    try {
      event = JSON.parse(message.data) as ReviewEvent;
    } catch {
      return;
    }
    if (typeof event.seq === 'number') {
      if (event.seq <= lastSeq) return;
      lastSeq = event.seq;
    }
    onEvent(event);
    if (event.type === 'done') source.close();
  };
  return () => source.close();
}
