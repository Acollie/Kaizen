// A draggable 1px hairline between panes with a wider invisible hit area, plus a
// collapse button revealed on hover. Arrow keys nudge the width when it is focused.

import { useRef, useState, type KeyboardEvent, type PointerEvent } from 'react';
import { widthFromPointer, type Side } from './layoutMath';
import type { TabBadge } from './tabBadges';

const NUDGE_PX = 16;

interface DividerProps {
  side: Side;
  width: number;
  /** Receives the unclamped desired width */
  onResize(width: number): void;
  onCommit(): void;
  onCollapse(): void;
  /** Double-click: back to the default width */
  onReset(): void;
}

const LABELS: Record<Side, string> = { sidebar: 'changes list', context: 'context panel' };

export function Divider({ side, width, onResize, onCommit, onCollapse, onReset }: DividerProps) {
  const [dragging, setDragging] = useState(false);
  const bounds = useRef({ left: 0, right: 0 });

  function onPointerDown(event: PointerEvent<HTMLDivElement>) {
    if (event.button !== 0) return;
    event.preventDefault();
    const workspace = event.currentTarget.parentElement?.getBoundingClientRect();
    if (!workspace) return;
    bounds.current = { left: workspace.left, right: workspace.right };
    capturePointer(event.currentTarget, event.pointerId, true);
    setDragging(true);
  }

  function onPointerMove(event: PointerEvent<HTMLDivElement>) {
    if (!dragging) return;
    onResize(widthFromPointer(side, event.clientX, bounds.current));
  }

  function onPointerUp(event: PointerEvent<HTMLDivElement>) {
    if (!dragging) return;
    capturePointer(event.currentTarget, event.pointerId, false);
    setDragging(false);
    onCommit();
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
    event.preventDefault();
    const grows = (event.key === 'ArrowRight') === (side === 'sidebar');
    onResize(width + (grows ? NUDGE_PX : -NUDGE_PX));
    onCommit();
  }

  return (
    <div
      className={`ws-divider ws-divider-${side}${dragging ? ' dragging' : ''}`}
      role="separator" aria-orientation="vertical" aria-label={`Resize ${LABELS[side]}`}
      aria-valuenow={width} tabIndex={0}
      onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp} onKeyDown={onKeyDown} onDoubleClick={onReset}
    >
      <button
        type="button" className="ws-collapse" title={`Collapse ${LABELS[side]}`} aria-label={`Collapse ${LABELS[side]}`}
        onPointerDown={(event) => event.stopPropagation()} onClick={onCollapse}
      >
        <Chevron direction={side === 'sidebar' ? 'left' : 'right'} />
      </button>
    </div>
  );
}

/** Pointer capture keeps the drag going outside the hairline; it throws for stale pointer ids */
function capturePointer(element: Element, pointerId: number, capture: boolean) {
  try {
    if (capture) element.setPointerCapture(pointerId);
    else if (element.hasPointerCapture(pointerId)) element.releasePointerCapture(pointerId);
  } catch {
    // Synthetic or already-released pointer: the drag still works while over the divider
  }
}

export function Chevron({ direction }: { direction: 'left' | 'right' | 'down' }) {
  const paths = { left: 'M6.5 2 3.5 5l3 3', right: 'M3.5 2l3 3-3 3', down: 'M2 3.5l3 3 3-3' };
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
      <path d={paths[direction]} fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/** Slim strip shown in place of a collapsed pane; clicking anywhere restores it */
interface RestoreRailProps {
  side: Side;
  label: string;
  badge?: TabBadge | null;
  onRestore(): void;
}

export function RestoreRail({ side, label, badge, onRestore }: RestoreRailProps) {
  return (
    <button type="button" className={`ws-rail ws-rail-${side}`} onClick={onRestore} title={`Show ${label}`} aria-label={`Show ${label}`}>
      <Chevron direction={side === 'sidebar' ? 'right' : 'left'} />
      <span className="ws-rail-label">{label}</span>
      {badge ? <span className={`cp-badge cp-badge-${badge.tone}`} title={badge.title}>{badge.text}</span> : null}
    </button>
  );
}
