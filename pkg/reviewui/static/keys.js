// Vim-style keymap: the binding table, a pure key-sequence parser, and the
// document listener. This module has no imports so it can be unit tested in node.

/** Every binding. The help overlay is generated from this table. */
export const BINDINGS = [
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
export function keyToken(event) {
  if (event.metaKey || event.altKey) return null;
  if (event.key === 'Escape') return 'Esc';
  if (event.key === ' ') return event.ctrlKey ? null : 'Space';
  if (event.key.length !== 1) return null;
  if (event.ctrlKey) return `C-${event.key.toLowerCase()}`;
  return event.key;
}

const sameKeys = (left, right) => left.length === right.length && left.every((key, index) => key === right[index]);
const startsWith = (keys, prefix) => prefix.length <= keys.length && prefix.every((key, index) => keys[index] === key);

/**
 * Create a key-sequence parser. feed(token, now) returns one of:
 *   { type: 'action', binding, count }   a complete binding (count defaults to 1)
 *   { type: 'pending', display }         waiting for more keys
 *   { type: 'none' }                     the keys matched nothing (state reset)
 * Digits typed before a binding form its count (5j); digits inside a sequence
 * are ordinary keys (g2). expire(now) clears a timed-out sequence.
 */
export function createKeyParser(bindings = BINDINGS, options = {}) {
  const sequenceTimeout = options.sequenceTimeout ?? SEQUENCE_TIMEOUT_MS;
  let count = '';
  let sequence = [];
  let lastTime = 0;

  const reset = () => { count = ''; sequence = []; };
  const display = () => count + sequence.join('');
  const isPending = () => count !== '' || sequence.length > 0;
  const deadline = () => lastTime + sequenceTimeout;
  const exact = (keys) => bindings.find((binding) => sameKeys(binding.keys, keys));
  const hasLonger = (keys) => bindings.some((binding) => binding.keys.length > keys.length && startsWith(binding.keys, keys));

  function complete(binding) {
    const repeat = count === '' ? 1 : Math.max(1, Math.min(999, parseInt(count, 10)));
    reset();
    return { type: 'action', binding, count: repeat };
  }

  function matchSequence(keys) {
    const binding = exact(keys);
    if (binding && !hasLonger(keys)) return complete(binding);
    if (hasLonger(keys)) {
      sequence = keys;
      return { type: 'pending', display: display() };
    }
    return null;
  }

  function feed(token, now = Date.now()) {
    if (isPending() && now > deadline()) reset();
    lastTime = now;

    if (token === 'Esc') {
      reset();
      return { type: 'action', binding: exact(['Esc']), count: 1 };
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

  function expire(now = Date.now()) {
    if (!isPending() || now < deadline()) return null;
    reset();
    return { type: 'none' };
  }

  return { feed, expire, reset, display, isPending, deadline };
}

/** Human-readable key label for the help overlay */
export function keyLabel(token) {
  if (token.startsWith('C-')) return `Ctrl-${token.slice(2)}`;
  return token;
}

/** Group bindings for the help overlay: [{ group, rows: [{ labels, description }] }] */
export function helpGroups(bindings = BINDINGS) {
  const groups = new Map();
  for (const binding of bindings) {
    if (!groups.has(binding.group)) groups.set(binding.group, new Map());
    const rows = groups.get(binding.group);
    const description = binding.description;
    const sequence = binding.keys.map(keyLabel).join('');
    const key = `${binding.action}:${binding.argument ?? ''}`;
    if (rows.has(key)) rows.get(key).labels.push(sequence);
    else rows.set(key, { labels: [sequence], description });
  }
  return [...groups.entries()].map(([group, rows]) => ({ group, rows: [...rows.values()] }));
}

function isTypingTarget(target) {
  if (!target || !target.closest) return false;
  return Boolean(target.closest('input, textarea, select, [contenteditable="true"], dialog[open]'));
}

/**
 * Install the keymap on document. handlers[action](count, binding) runs bindings;
 * onPending(display, mode) reports the indicator text. While focus is in a field
 * or a dialog is open, only Esc is handled (it blurs or lets the dialog close).
 */
export function installKeymap({ handlers, onPending, isModalOpen }) {
  const parser = createKeyParser();
  let timer = null;

  const report = (mode = 'NORMAL') => onPending && onPending(parser.display(), mode);

  function run(result) {
    if (!result || result.type !== 'action' || !result.binding) return false;
    const handler = handlers[result.binding.action];
    if (!handler) return false;
    handler(result.count, result.binding);
    return true;
  }

  function scheduleExpiry() {
    window.clearTimeout(timer);
    if (!parser.isPending()) return;
    timer = window.setTimeout(() => {
      run(parser.expire());
      report();
    }, Math.max(0, parser.deadline() - Date.now()) + 5);
  }

  document.addEventListener('keydown', (event) => {
    const token = keyToken(event);
    if (!token) return;
    if ((isModalOpen && isModalOpen()) || isTypingTarget(event.target)) {
      if (token === 'Esc' && isTypingTarget(event.target) && !event.target.closest('dialog[open]')) {
        event.target.blur();
        event.preventDefault();
        report();
      }
      return;
    }
    const result = parser.feed(token);
    if (result.type !== 'none') event.preventDefault();
    run(result);
    report();
    scheduleExpiry();
  });

  document.addEventListener('focusin', (event) => {
    if (isTypingTarget(event.target)) onPending && onPending('', 'INPUT');
  });
  document.addEventListener('focusout', () => window.setTimeout(() => {
    if (!isTypingTarget(document.activeElement)) report();
  }, 0));

  return { parser };
}
