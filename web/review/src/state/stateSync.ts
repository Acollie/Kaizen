// Debounced saving of the reviewer's work to the server (PUT /api/state). Changes are
// batched for ~500 ms; on page unload the latest unsaved document goes out with a
// keepalive request so a closing tab does not lose the last few seconds of triage.

export interface StateRemote {
  /** PUT the document; rejects on failure */
  save(document: unknown): Promise<void>;
  /** Best-effort PUT that survives the page unloading (fetch keepalive) */
  saveOnUnload(document: unknown): void;
}

export interface Timers {
  setTimeout(callback: () => void, delayMs: number): unknown;
  clearTimeout(handle: unknown): void;
}

export interface StateSync {
  /** Queue the document; it is sent once no newer change arrived for delayMs */
  schedule(document: unknown): void;
  /** Send anything not yet confirmed saved with saveOnUnload (beforeunload / pagehide) */
  flush(): void;
  /** Stop the timer without sending */
  dispose(): void;
}

export const SAVE_DELAY_MS = 500;

const defaultTimers: Timers = {
  setTimeout: (callback, delayMs) => globalThis.setTimeout(callback, delayMs),
  clearTimeout: (handle) => globalThis.clearTimeout(handle as ReturnType<typeof globalThis.setTimeout>),
};

export function createStateSync(remote: StateRemote, delayMs = SAVE_DELAY_MS, timers: Timers = defaultTimers): StateSync {
  let timer: unknown = null;
  let latest: unknown = undefined;
  // True from a change until the server confirmed a save of that (or a newer) document
  let unsaved = false;
  let version = 0;

  const send = () => {
    timer = null;
    const sentVersion = version;
    remote.save(latest).then(
      () => { if (sentVersion === version) unsaved = false; },
      () => { /* stays unsaved: the next change or the unload flush retries */ },
    );
  };

  return {
    schedule(document) {
      latest = document;
      unsaved = true;
      version += 1;
      if (timer !== null) timers.clearTimeout(timer);
      timer = timers.setTimeout(send, delayMs);
    },
    flush() {
      if (timer !== null) {
        timers.clearTimeout(timer);
        timer = null;
      }
      if (unsaved) remote.saveOnUnload(latest);
    },
    dispose() {
      if (timer !== null) timers.clearTimeout(timer);
      timer = null;
    },
  };
}
