// "Go to callers" (g r): switch to the Function tab and focus its callers list. The
// request is remembered until the Function panel is mounted to consume it, so it works
// from any tab. Idempotent, so more than one component may register the command.

import type { ReviewStore } from '../../state/store';

type Listener = () => void;

let pending = false;
const listeners = new Set<Listener>();

export function requestCallersFocus(store: ReviewStore): void {
  pending = true;
  store.getState().setTab('function');
  for (const listener of listeners) listener();
}

/** Returns true (once) if a request is waiting */
export function takeCallersRequest(): boolean {
  const waiting = pending;
  pending = false;
  return waiting;
}

export function onCallersRequest(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
