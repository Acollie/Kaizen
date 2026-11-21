// Split a help label from helpGroups() ("gT", "]c", "Ctrl-d", "Space") back into the
// individual keys to draw one <kbd> per key.

const NAMED_KEY = /^(Ctrl-.|Shift-Enter|Esc|Space|Tab|Enter)/;

export function splitKeyLabel(label: string): string[] {
  const keys: string[] = [];
  let rest = label;
  while (rest.length > 0) {
    const named = NAMED_KEY.exec(rest);
    const key = named ? named[0] : rest.charAt(0);
    keys.push(key);
    rest = rest.slice(key.length);
  }
  return keys;
}
