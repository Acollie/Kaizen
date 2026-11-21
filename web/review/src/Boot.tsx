// Loads the review context, then mounts the app with its store.

import { useEffect, useState } from 'react';
import { fetchContext, fetchState, putState, putStateOnUnload, sessionToken } from './lib/api';
import { createStateSync } from './state/stateSync';
import { bindPersistence, createReviewStore, type ReviewStore } from './state/store';
import { safeLocalStorage } from './state/persistence';
import { StoreProvider } from './state/StoreProvider';
import { App } from './App';

type BootState = { kind: 'loading' } | { kind: 'error'; message: string } | { kind: 'ready'; store: ReviewStore };

export function Boot() {
  const [state, setState] = useState<BootState>({ kind: 'loading' });

  useEffect(() => {
    if (!sessionToken()) {
      setState({ kind: 'error', message: 'This page needs its session token. Open the URL printed by `kaizen review` (it ends in ?t=…).' });
      return;
    }
    Promise.all([fetchContext(), fetchState()])
      .then(([context, saved]) => {
        const store = createReviewStore(context);
        const sync = createStateSync({ save: putState, saveOnUnload: putStateOnUnload });
        bindPersistence(store, safeLocalStorage(), { saved, sync });
        // Closing the tab or stopping the server mid-debounce must not lose the last change
        window.addEventListener('beforeunload', () => sync.flush());
        window.addEventListener('pagehide', () => sync.flush());
        setState({ kind: 'ready', store });
      })
      .catch((error: Error) => setState({ kind: 'error', message: `Could not load the review: ${error.message}` }));
  }, []);

  if (state.kind === 'loading') return <div className="boot"><span className="spinner" /> Loading review…</div>;
  if (state.kind === 'error') return <div className="boot boot-error">{state.message}</div>;
  return (
    <StoreProvider store={state.store}>
      <App />
    </StoreProvider>
  );
}
