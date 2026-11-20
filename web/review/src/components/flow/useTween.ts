// Tweens node boxes between layouts so re-layouts move smoothly instead of jumping.
// New nodes fade in at their final position; nodes that leave are dropped.

import { useEffect, useRef, useState } from 'react';
import type { LaidOutNode, NodeBox } from '../../lib/flowLayout';

export const MOVE_MS = 420;

export interface AnimatedBox extends NodeBox { opacity: number }

const easeOutCubic = (t: number) => 1 - (1 - t) ** 3;
const lerp = (from: number, to: number, t: number) => from + (to - from) * t;

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches);
}

function finalBoxes(nodes: readonly LaidOutNode[]): Map<string, AnimatedBox> {
  return new Map(nodes.map((node) => [node.id, { id: node.id, x: node.x, y: node.y, width: node.width, height: node.height, opacity: 1 }]));
}

/** Interpolate from the boxes on screen to the target layout */
export function interpolateBoxes(from: ReadonlyMap<string, AnimatedBox>, to: ReadonlyMap<string, AnimatedBox>, t: number): Map<string, AnimatedBox> {
  const result = new Map<string, AnimatedBox>();
  for (const [id, target] of to) {
    const start = from.get(id);
    if (!start) {
      result.set(id, { ...target, opacity: t });
      continue;
    }
    result.set(id, {
      id,
      x: lerp(start.x, target.x, t),
      y: lerp(start.y, target.y, t),
      width: lerp(start.width, target.width, t),
      height: target.height,
      opacity: lerp(start.opacity, 1, t),
    });
  }
  return result;
}

export function useTweenedBoxes(nodes: readonly LaidOutNode[], animate: boolean): Map<string, AnimatedBox> {
  const [boxes, setBoxes] = useState(() => finalBoxes(nodes));
  const shown = useRef(boxes);
  shown.current = boxes;

  useEffect(() => {
    const target = finalBoxes(nodes);
    if (!animate || prefersReducedMotion()) {
      setBoxes(target);
      return undefined;
    }
    const from = shown.current;
    const startedAt = performance.now();
    let frame = 0;
    const tick = (now: number) => {
      const progress = Math.min(1, (now - startedAt) / MOVE_MS);
      setBoxes(progress >= 1 ? target : interpolateBoxes(from, target, easeOutCubic(progress)));
      if (progress < 1) frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [nodes, animate]);

  return boxes;
}
