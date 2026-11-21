// Keeps the active row of a list visible: scrolls it into view (nearest edge) when it
// becomes active and again whenever `trigger` changes (e.g. selection.reveal).

import { useEffect, useRef } from 'react';

export function useScrollIntoView<T extends HTMLElement>(active: boolean, trigger: unknown) {
  const ref = useRef<T>(null);
  useEffect(() => {
    if (active) ref.current?.scrollIntoView?.({ block: 'nearest' });
  }, [active, trigger]);
  return ref;
}
