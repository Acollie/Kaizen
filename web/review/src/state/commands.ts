// Command registry. Keyboard actions that need a component's internal state (scroll
// position, the current hunk, the flow diagram's zoom) are registered by that component
// with useCommands(); the keymap dispatches to whichever component registered the
// action. Store-level actions (selection, tabs) are handled in useKeymap directly.

import { useEffect, useRef } from 'react';
import type { Action, Binding } from '../keys/keymap';

export type CommandHandler = (count: number, binding: Binding) => void;
export type CommandMap = Partial<Record<Action, CommandHandler>>;

const registry = new Map<symbol, CommandMap>();

export function runRegisteredCommand(action: Action, count: number, binding: Binding): boolean {
  let handled = false;
  for (const commands of registry.values()) {
    const handler = commands[action];
    if (handler) {
      handler(count, binding);
      handled = true;
    }
  }
  return handled;
}

/**
 * Register commands for the lifetime of a component. The latest handlers are always
 * used (they may close over fresh state) without re-registering on every render.
 */
export function useCommands(commands: CommandMap): void {
  const latest = useRef(commands);
  latest.current = commands;
  useEffect(() => {
    const key = Symbol('commands');
    const actions = Object.keys(latest.current) as Action[];
    const proxy: CommandMap = {};
    for (const action of actions) {
      proxy[action] = (count, binding) => latest.current[action]?.(count, binding);
    }
    registry.set(key, proxy);
    return () => {
      registry.delete(key);
    };
    // Re-register only when the set of actions changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Object.keys(commands).sort().join(',')]);
}
