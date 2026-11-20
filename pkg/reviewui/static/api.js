// API client. The session token comes from ?t= and is sent on every call.

const TOKEN = new URLSearchParams(window.location.search).get('t') || '';
const TOKEN_HEADER = 'X-Kaizen-Token';

export function hasToken() {
  return TOKEN !== '';
}

function withToken(path) {
  const url = new URL(path, window.location.origin);
  url.searchParams.set('t', TOKEN);
  return url.pathname + url.search;
}

async function getJSON(path) {
  const response = await fetch(withToken(path), {
    headers: { [TOKEN_HEADER]: TOKEN, Accept: 'application/json' },
    credentials: 'same-origin',
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `${response.status} ${response.statusText}`);
  return body;
}

export function fetchContext() {
  return getJSON('/api/context');
}

/** Head-side lines from..to (1-based, inclusive). Resolves to {lines:[{n,text}], total}. */
export function fetchFileLines(path, from, to) {
  const params = new URLSearchParams({ path, from: String(from), to: String(to) });
  return getJSON(`/api/file?${params}`);
}

export async function postReview(request) {
  const response = await fetch('/api/post', {
    method: 'POST',
    headers: { [TOKEN_HEADER]: TOKEN, 'Content-Type': 'application/json', Accept: 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(request),
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `${response.status} ${response.statusText}`);
  return body;
}

/**
 * Subscribe to the SSE event stream. EventSource reconnects on its own and sends
 * Last-Event-ID; onEvent receives parsed events, deduplicated by seq.
 */
export function openEventStream({ onEvent, onOpen, onError }) {
  let lastSeq = 0;
  const source = new EventSource(withToken('/api/events'));
  source.onopen = () => onOpen && onOpen();
  source.onerror = () => onError && onError(source.readyState === EventSource.CLOSED);
  source.onmessage = (message) => {
    let event;
    try {
      event = JSON.parse(message.data);
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
  return source;
}
