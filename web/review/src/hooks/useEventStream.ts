// Connects the SSE stream to the store for the lifetime of the app.

import { useEffect } from 'react';
import { openEventStream } from '../lib/api';
import { useReviewStore } from '../state/StoreProvider';

export function useEventStream(): void {
  const store = useReviewStore();
  useEffect(() => {
    return openEventStream({
      onEvent: (event) => store.getState().applyEvent(event),
      onOpen: () => {
        const { status, setStatus } = store.getState();
        if (status.state === 'connecting') setStatus({ state: 'running', message: 'Waiting for the review engine', agent: '' });
      },
      onError: (closed) => {
        const { status, result, setStatus } = store.getState();
        if (status.state === 'done' || status.state === 'error') return;
        if (closed && !result) setStatus({ state: 'done', message: 'Stream closed', agent: '' });
        else if (!closed) setStatus({ state: 'connecting', message: 'Reconnecting', agent: '' });
      },
    });
  }, [store]);
}
