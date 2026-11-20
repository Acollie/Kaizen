// DOM helpers for the code view's scroll container. Rows and cards carry data-key.

export type Placement = 'focus' | 'start' | 'end' | 'nearest';

/** Where a revealed row lands: a little above centre, so the lines and cards below it show */
const FOCUS_FRACTION = 0.32;
const EDGE_MARGIN = 40;

export function elementForKey(container: HTMLElement, key: string): HTMLElement | null {
  return container.querySelector<HTMLElement>(`[data-key="${CSS.escape(key)}"]`);
}

function offsetWithin(container: HTMLElement, element: HTMLElement): number {
  return element.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
}

export function scrollToElement(container: HTMLElement, element: HTMLElement, placement: Placement): void {
  const top = offsetWithin(container, element);
  const height = element.offsetHeight;
  const viewport = container.clientHeight;
  let target = container.scrollTop;
  if (placement === 'start') target = top - EDGE_MARGIN;
  else if (placement === 'end') target = top + height - viewport + EDGE_MARGIN;
  else if (placement === 'focus') target = height > viewport * (1 - FOCUS_FRACTION) ? top - EDGE_MARGIN : top - viewport * FOCUS_FRACTION;
  else if (top < container.scrollTop + EDGE_MARGIN) target = top - EDGE_MARGIN;
  else if (top + height > container.scrollTop + viewport - EDGE_MARGIN) target = top + height - viewport + EDGE_MARGIN;
  container.scrollTop = Math.max(0, target);
}

export function scrollToKey(container: HTMLElement, key: string, placement: Placement): boolean {
  const element = elementForKey(container, key);
  if (!element) return false;
  scrollToElement(container, element, placement);
  return true;
}

/**
 * Briefly highlight rows. A data attribute (not a class) carries the animation so a
 * React re-render of the row's className cannot cut it short.
 */
export function flashKeys(container: HTMLElement, keys: readonly string[]): void {
  for (const key of keys) {
    const element = elementForKey(container, key);
    if (!element) continue;
    delete element.dataset.flash;
    void element.offsetWidth;
    element.dataset.flash = 'on';
    element.addEventListener('animationend', () => { delete element.dataset.flash; }, { once: true });
  }
}

/** Key of the first row (or card) visible at the top of the scroll container */
export function keyAtTop(container: HTMLElement): string | null {
  const rect = container.getBoundingClientRect();
  for (const offset of [4, 24, 48]) {
    const hit = document.elementFromPoint(rect.left + rect.width / 2, rect.top + offset);
    const keyed = hit instanceof Element ? hit.closest<HTMLElement>('[data-key]') : null;
    if (keyed && container.contains(keyed)) return keyed.dataset.key ?? null;
  }
  return null;
}
