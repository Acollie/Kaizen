// Auto-scroll a feed to the bottom as items arrive, unless the user has scrolled up;
// then report unseen items so the panel can offer a "New activity" jump.

import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import { isNearBottom } from './activityModel';

export function useStickToBottom<T extends HTMLElement>(itemCount: number) {
  const ref = useRef<T>(null);
  const following = useRef(true);
  const seenCount = useRef(itemCount);
  const [unseen, setUnseen] = useState(false);

  const scrollToBottom = useCallback(() => {
    const element = ref.current;
    if (!element) return;
    element.scrollTop = element.scrollHeight;
    following.current = true;
    seenCount.current = itemCount;
    setUnseen(false);
  }, [itemCount]);

  useLayoutEffect(() => {
    if (following.current) scrollToBottom();
    else if (itemCount > seenCount.current) setUnseen(true);
  }, [itemCount, scrollToBottom]);

  const onScroll = useCallback(() => {
    const element = ref.current;
    if (!element) return;
    following.current = isNearBottom(element);
    if (following.current) {
      seenCount.current = itemCount;
      setUnseen(false);
    }
  }, [itemCount]);

  return { ref, onScroll, unseen, scrollToBottom };
}
