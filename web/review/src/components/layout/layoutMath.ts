// Pure pane-width maths for the workspace: clamping, persistence parsing and the grid
// template. No DOM access here so it can be unit-tested.

export type Side = 'sidebar' | 'context';
export type PaneWidths = Record<Side, number>;

export const DEFAULT_WIDTHS: PaneWidths = { sidebar: 300, context: 440 };
export const MIN_WIDTH: PaneWidths = { sidebar: 200, context: 320 };
/** Largest share of the workspace each side pane may take */
export const MAX_SHARE: PaneWidths = { sidebar: 0.4, context: 0.6 };
/** The code view never gets narrower than this while both side panes are open */
export const MIN_CODE_WIDTH = 360;
/** Width of the restore rail shown in place of a collapsed pane */
export const RAIL_WIDTH = 28;
export const STORAGE_KEY = 'kaizen-review-layout';

/**
 * Clamp a desired side-pane width against the workspace width and the other pane.
 * The minimum wins over the maximum on very narrow screens.
 */
export function clampWidth(side: Side, desired: number, workspaceWidth: number, otherWidth: number): number {
  const shareLimit = workspaceWidth * MAX_SHARE[side];
  const codeLimit = workspaceWidth - otherWidth - MIN_CODE_WIDTH;
  const max = Math.min(shareLimit, codeLimit);
  return Math.round(Math.max(MIN_WIDTH[side], Math.min(max, desired)));
}

/** Effective widths for the current workspace width (context is clamped first: it is the larger) */
export function effectiveWidths(preferred: PaneWidths, workspaceWidth: number, collapsed: Record<Side, boolean>): PaneWidths {
  if (workspaceWidth <= 0) return preferred;
  const sidebarSpace = collapsed.sidebar ? RAIL_WIDTH : MIN_WIDTH.sidebar;
  const context = collapsed.context ? RAIL_WIDTH : clampWidth('context', preferred.context, workspaceWidth, sidebarSpace);
  const sidebar = collapsed.sidebar ? RAIL_WIDTH : clampWidth('sidebar', preferred.sidebar, workspaceWidth, context);
  return { sidebar, context };
}

/** Width implied by a pointer position while dragging a divider */
export function widthFromPointer(side: Side, clientX: number, bounds: { left: number; right: number }): number {
  return side === 'sidebar' ? clientX - bounds.left : bounds.right - clientX;
}

/** Parse stored widths, ignoring anything malformed */
export function parseStoredWidths(raw: string | null): PaneWidths {
  if (!raw) return { ...DEFAULT_WIDTHS };
  try {
    const value = JSON.parse(raw) as Partial<Record<Side, unknown>>;
    return {
      sidebar: validWidth(value.sidebar) ?? DEFAULT_WIDTHS.sidebar,
      context: validWidth(value.context) ?? DEFAULT_WIDTHS.context,
    };
  } catch {
    return { ...DEFAULT_WIDTHS };
  }
}

function validWidth(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 && value < 10000 ? value : null;
}

export function loadWidths(): PaneWidths {
  try {
    return parseStoredWidths(window.localStorage.getItem(STORAGE_KEY));
  } catch {
    return { ...DEFAULT_WIDTHS };
  }
}

export function saveWidths(widths: PaneWidths): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(widths));
  } catch {
    // Storage unavailable (private mode): widths are simply not remembered
  }
}
