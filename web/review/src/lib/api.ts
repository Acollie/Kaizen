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

/**
 * The reviewer's saved work for this PR head (GET /api/state), or null when the server
 * has none, does not keep state, or cannot be reached
 */
export async function fetchState(): Promise<unknown> {
  try {
    const response = await fetch('/api/state', {
      headers: { [TOKEN_HEADER]: sessionToken(), Accept: 'application/json' },
      credentials: 'same-origin',
    });
    if (!response.ok || response.status === 204) return null;
    return (await response.json()) as unknown;
  } catch {
    return null;
  }
}

function putStateRequest(document: unknown, keepalive: boolean): Promise<Response> {
  return fetch('/api/state', {
    method: 'PUT',
    headers: { [TOKEN_HEADER]: sessionToken(), 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(document),
    keepalive,
  });
}

/** Save the reviewer's work (PUT /api/state); rejects when the server refuses it */
export async function putState(document: unknown): Promise<void> {
  const response = await putStateRequest(document, false);
  if (!response.ok) throw new Error(`save state: ${response.status}`);
}

/**
 * Best-effort save while the page unloads. keepalive lets the request outlive the page,
 * but browsers cap keepalive bodies (about 64 KB); larger states fall back to a normal
 * request, which the debounced save has usually already covered.
 */
export function putStateOnUnload(document: unknown): void {
  try {
    void putStateRequest(document, true).catch(() => putStateRequest(document, false).catch(() => undefined));
  } catch {
    void putStateRequest(document, false).catch(() => undefined);
  }
}

/** Thrown by postReview when this session already posted and confirm_repost was not set */
export class AlreadyPostedError extends Error {
  readonly previousUrl: string;

  constructor(previousUrl: string) {
    super('A review was already posted in this session');
    this.name = 'AlreadyPostedError';
    this.previousUrl = previousUrl;
  }
}

export async function postReview(request: PostRequest): Promise<PostResult> {
  const response = await fetch('/api/post', {
    method: 'POST',
    headers: { [TOKEN_HEADER]: sessionToken(), 'Content-Type': 'application/json', Accept: 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(request),
  });
  if (response.status === 409) {
    const body = (await response.json().catch(() => ({}))) as { previous_url?: string };
    throw new AlreadyPostedError(body.previous_url ?? '');
  }
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
