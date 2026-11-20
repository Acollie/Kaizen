// Vim-style keymap: the binding table and a pure key-sequence parser. The help overlay
// is generated from BINDINGS, so it can never drift from what the keys actually do.
// DOM wiring lives in useKeymap.ts; this module has no DOM dependencies.

export type Action =
  | 'nextFunction' | 'prevFunction' | 'nextFile' | 'prevFile' | 'nextHunk' | 'prevHunk'
  | 'nextFinding' | 'prevFinding' | 'toggleInclude'
  | 'top' | 'bottom' | 'halfPageDown' | 'halfPageUp' | 'pageDown' | 'pageUp'
  | 'expandAbove' | 'expandBelow' | 'expandAll' | 'gotoDefinition' | 'gotoCallers' | 'back'
  | 'focusLeft' | 'focusRight' | 'focusFilter' | 'nextTab' | 'prevTab' | 'tab'
  | 'toggleWrap' | 'fitGraph' | 'expandFlow' | 'toggleLayered' | 'post' | 'help' | 'escape';

export interface Binding {
  keys: readonly string[];
  action: Action;
  /** Extra data for the action, e.g. the tab name for 'tab' */
  argument?: string;
  group: string;
  description: string;
}

export type ParseResult =
  | { type: 'action'; binding: Binding; count: number }
  | { type: 'pending'; display: string }
  | { type: 'none' };

/** Every binding. The help overlay is generated from this table. */
export const BINDINGS: readonly Binding[] = [
  { keys: ['j'], action: 'nextFunction', group: 'Navigate', description: 'Next changed function' },
  { keys: ['k'], action: 'prevFunction', group: 'Navigate', description: 'Previous changed function' },
  { keys: ['J'], action: 'nextFile', group: 'Navigate', description: 'Next file' },
  { keys: ['K'], action: 'prevFile', group: 'Navigate', description: 'Previous file' },
  { keys: [']', 'c'], action: 'nextHunk', group: 'Navigate', description: 'Next hunk' },
  { keys: ['[', 'c'], action: 'prevHunk', group: 'Navigate', description: 'Previous hunk' },
  { keys: [']', 'f'], action: 'nextFinding', group: 'Findings', description: 'Next finding (across files)' },
  { keys: ['[', 'f'], action: 'prevFinding', group: 'Findings', description: 'Previous finding (across files)' },
  { keys: ['n'], action: 'nextFinding', group: 'Findings', description: 'Next finding' },
  { keys: ['N'], action: 'prevFinding', group: 'Findings', description: 'Previous finding' },
  { keys: ['x'], action: 'toggleInclude', group: 'Findings', description: 'Toggle "include in post" on the current finding' },
  { keys: ['Space'], action: 'toggleInclude', group: 'Findings', description: 'Toggle "include in post"' },
  { keys: ['g', 'g'], action: 'top', group: 'Scroll', description: 'Top of the current file' },
  { keys: ['G'], action: 'bottom', group: 'Scroll', description: 'Bottom of the current file' },
  { keys: ['C-d'], action: 'halfPageDown', group: 'Scroll', description: 'Half page down' },
  { keys: ['C-u'], action: 'halfPageUp', group: 'Scroll', description: 'Half page up' },
  { keys: ['C-f'], action: 'pageDown', group: 'Scroll', description: 'Page down' },
  { keys: ['C-b'], action: 'pageUp', group: 'Scroll', description: 'Page up' },
  { keys: ['z', 'o'], action: 'expandAbove', group: 'Context', description: 'Expand context above the current hunk' },
  { keys: ['z', 'c'], action: 'expandBelow', group: 'Context', description: 'Expand context below the current hunk' },
  { keys: ['e'], action: 'expandAbove', group: 'Context', description: 'Expand context above' },
  { keys: ['E'], action: 'expandBelow', group: 'Context', description: 'Expand context below' },
  { keys: ['z', 'R'], action: 'expandAll', group: 'Context', description: 'Expand the whole file' },
  { keys: ['g', 'd'], action: 'gotoDefinition', group: 'Context', description: "Go to the selected callee's definition" },
  { keys: ['g', 'r'], action: 'gotoCallers', group: 'Context', description: 'Show callers (list and graph)' },
  { keys: ['C-o'], action: 'back', group: 'Context', description: 'Jump back' },
  { keys: ['h'], action: 'focusLeft', group: 'Panes', description: 'Focus the pane to the left' },
  { keys: ['l'], action: 'focusRight', group: 'Panes', description: 'Focus the pane to the right' },
  { keys: ['/'], action: 'focusFilter', group: 'Panes', description: 'Filter files and functions' },
  { keys: ['g', 't'], action: 'nextTab', group: 'Panes', description: 'Next context tab' },
  { keys: ['g', 'T'], action: 'prevTab', group: 'Panes', description: 'Previous context tab' },
  { keys: ['g', '1'], action: 'tab', argument: 'review', group: 'Panes', description: 'AI review tab' },
  { keys: ['g', '2'], action: 'tab', argument: 'graph', group: 'Panes', description: 'Call flow tab' },
  { keys: ['g', '3'], action: 'tab', argument: 'function', group: 'Panes', description: 'Function tab' },
  { keys: ['g', '4'], action: 'tab', argument: 'activity', group: 'Panes', description: 'Agent activity tab' },
  { keys: ['w'], action: 'toggleWrap', group: 'Scroll', description: 'Toggle soft-wrap in the code view' },
  { keys: ['f'], action: 'fitGraph', group: 'Flow', description: 'Fit the call flow to view' },
  { keys: ['F'], action: 'expandFlow', group: 'Flow', description: 'Expand the call flow to full width (again to restore)' },
  { keys: ['L'], action: 'toggleLayered', group: 'Flow', description: "Switch between the selected function's flow and the whole PR" },
  { keys: ['P'], action: 'post', group: 'Review', description: 'Open the Post review dialog' },
  { keys: ['?'], action: 'help', group: 'Review', description: 'Show this help' },
  { keys: ['Esc'], action: 'escape', group: 'Review', description: 'Close overlays, clear pending keys' },
];

export const SEQUENCE_TIMEOUT_MS = 800;

/** Normalise a KeyboardEvent to a token: "j", "G", "C-d", "Space", "Esc"; null to ignore */
export function keyToken(event: Pick<KeyboardEvent, 'key' | 'metaKey' | 'altKey' | 'ctrlKey'>): string | null {
  if (event.metaKey || event.altKey) return null;
  if (event.key === 'Escape') return 'Esc';
  if (event.key === ' ') return event.ctrlKey ? null : 'Space';
  if (event.key.length !== 1) return null;
  if (event.ctrlKey) return `C-${event.key.toLowerCase()}`;
  return event.key;
}

const sameKeys = (left: readonly string[], right: readonly string[]) =>
  left.length === right.length && left.every((key, index) => key === right[index]);
const startsWith = (keys: readonly string[], prefix: readonly string[]) =>
  prefix.length <= keys.length && prefix.every((key, index) => keys[index] === key);

export interface KeyParser {
  feed(token: string, now?: number): ParseResult;
  expire(now?: number): ParseResult | null;
  reset(): void;
  display(): string;
  isPending(): boolean;
  deadline(): number;
}

/**
 * Create a key-sequence parser. Digits typed before a binding form its count (5j);
 * digits inside a sequence are ordinary keys (g2). expire(now) clears a timed-out sequence.
 */
export function createKeyParser(bindings: readonly Binding[] = BINDINGS, options: { sequenceTimeout?: number } = {}): KeyParser {
  const sequenceTimeout = options.sequenceTimeout ?? SEQUENCE_TIMEOUT_MS;
  let count = '';
  let sequence: string[] = [];
  let lastTime = 0;

  const reset = () => { count = ''; sequence = []; };
  const display = () => count + sequence.join('');
  const isPending = () => count !== '' || sequence.length > 0;
  const deadline = () => lastTime + sequenceTimeout;
  const exact = (keys: readonly string[]) => bindings.find((binding) => sameKeys(binding.keys, keys));
  const hasLonger = (keys: readonly string[]) =>
    bindings.some((binding) => binding.keys.length > keys.length && startsWith(binding.keys, keys));

  function complete(binding: Binding): ParseResult {
    const repeat = count === '' ? 1 : Math.max(1, Math.min(999, parseInt(count, 10)));
    reset();
    return { type: 'action', binding, count: repeat };
  }

  function matchSequence(keys: string[]): ParseResult | null {
    const binding = exact(keys);
    if (binding && !hasLonger(keys)) return complete(binding);
    if (hasLonger(keys)) {
      sequence = keys;
      return { type: 'pending', display: display() };
    }
    return null;
  }

  function feed(token: string, now = Date.now()): ParseResult {
    if (isPending() && now > deadline()) reset();
    lastTime = now;

    if (token === 'Esc') {
      reset();
      const escape = exact(['Esc']);
      return escape ? { type: 'action', binding: escape, count: 1 } : { type: 'none' };
    }
    if (sequence.length === 0 && /^[0-9]$/.test(token) && (count !== '' || token !== '0')) {
      count += token;
      return { type: 'pending', display: display() };
    }

    const matched = matchSequence([...sequence, token]);
    if (matched) return matched;
    // An unfinished sequence followed by a key that starts a new one: retry it alone
    if (sequence.length > 0) {
      const savedCount = count;
      reset();
      count = savedCount;
      const retried = matchSequence([token]);
      if (retried) return retried;
    }
    reset();
    return { type: 'none' };
  }

  function expire(now = Date.now()): ParseResult | null {
    if (!isPending() || now < deadline()) return null;
    reset();
    return { type: 'none' };
  }

  return { feed, expire, reset, display, isPending, deadline };
}

/** Human-readable key label for the help overlay */
export function keyLabel(token: string): string {
  if (token.startsWith('C-')) return `Ctrl-${token.slice(2)}`;
  return token;
}

export interface HelpGroup {
  group: string;
  rows: { labels: string[]; description: string }[];
}

/** Group bindings for the help overlay */
export function helpGroups(bindings: readonly Binding[] = BINDINGS): HelpGroup[] {
  const groups = new Map<string, Map<string, { labels: string[]; description: string }>>();
  for (const binding of bindings) {
    if (!groups.has(binding.group)) groups.set(binding.group, new Map());
    const rows = groups.get(binding.group)!;
    const sequence = binding.keys.map(keyLabel).join('');
    const key = `${binding.action}:${binding.argument ?? ''}`;
    const existing = rows.get(key);
    if (existing) existing.labels.push(sequence);
    else rows.set(key, { labels: [sequence], description: binding.description });
  }
  return [...groups.entries()].map(([group, rows]) => ({ group, rows: [...rows.values()] }));
}
