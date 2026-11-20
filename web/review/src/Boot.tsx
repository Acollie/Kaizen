// Loads the review context, then mounts the app with its store.

import { useEffect, useState } from 'react';
import { fetchContext, sessionToken } from './lib/api';
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
    fetchContext()
      .then((context) => {
        const store = createReviewStore(context);
        bindPersistence(store, safeLocalStorage());
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
