// Minimal per-line token colouring for keywords, strings, comments and numbers.
// Output is built from text nodes and spans; it never parses HTML.

const KEYWORDS = {
  Go: ['break', 'case', 'chan', 'const', 'continue', 'default', 'defer', 'else', 'fallthrough', 'for', 'func',
    'go', 'goto', 'if', 'import', 'interface', 'map', 'package', 'range', 'return', 'select', 'struct', 'switch',
    'type', 'var', 'nil', 'true', 'false', 'error', 'string', 'int', 'bool', 'byte', 'float64', 'any'],
  Python: ['and', 'as', 'assert', 'async', 'await', 'break', 'class', 'continue', 'def', 'del', 'elif', 'else',
    'except', 'finally', 'for', 'from', 'global', 'if', 'import', 'in', 'is', 'lambda', 'None', 'nonlocal', 'not',
    'or', 'pass', 'raise', 'return', 'True', 'False', 'try', 'while', 'with', 'yield', 'self'],
  Kotlin: ['fun', 'val', 'var', 'class', 'object', 'if', 'else', 'when', 'for', 'while', 'return', 'null', 'true',
    'false', 'import', 'package', 'private', 'public', 'override', 'data', 'suspend', 'is', 'in'],
  Swift: ['func', 'let', 'var', 'class', 'struct', 'enum', 'if', 'else', 'guard', 'for', 'while', 'return', 'nil',
    'true', 'false', 'import', 'private', 'public', 'switch', 'case', 'self', 'in', 'throws', 'try'],
};

const COMMENT = { Go: '//', Kotlin: '//', Swift: '//', Python: '#' };
const patternCache = new Map();

function patternFor(language) {
  if (patternCache.has(language)) return patternCache.get(language);
  const keywords = KEYWORDS[language];
  if (!keywords) return null;
  const comment = COMMENT[language] === '#' ? '#.*$' : '\\/\\/.*$|\\/\\*.*?(?:\\*\\/|$)';
  const strings = '"(?:[^"\\\\]|\\\\.)*"?|\'(?:[^\'\\\\]|\\\\.)*\'?|`[^`]*`?';
  const source = `(${comment})|(${strings})|\\b(${keywords.join('|')})\\b|\\b(\\d[\\d_.xXa-fA-F]*)\\b`;
  const pattern = new RegExp(source, 'g');
  patternCache.set(language, pattern);
  return pattern;
}

/** Append the coloured tokens of one line of code to element */
export function highlightInto(element, text, language) {
  const pattern = patternFor(language);
  if (!pattern || text.length > 2000) {
    element.textContent = text;
    return element;
  }
  let cursor = 0;
  pattern.lastIndex = 0;
  for (const match of text.matchAll(pattern)) {
    if (match.index > cursor) element.append(document.createTextNode(text.slice(cursor, match.index)));
    const span = document.createElement('span');
    span.className = match[1] ? 'tok-c' : match[2] ? 'tok-s' : match[3] ? 'tok-k' : 'tok-n';
    span.textContent = match[0];
    element.append(span);
    cursor = match.index + match[0].length;
  }
  if (cursor < text.length) element.append(document.createTextNode(text.slice(cursor)));
  return element;
}

export function languageForPath(path) {
  if (path.endsWith('.go')) return 'Go';
  if (path.endsWith('.py')) return 'Python';
  if (path.endsWith('.kt') || path.endsWith('.kts')) return 'Kotlin';
  if (path.endsWith('.swift')) return 'Swift';
  return '';
}
