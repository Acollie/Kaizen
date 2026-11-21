// Minimal per-line syntax tokenizer for keywords, strings, comments, numbers and
// function calls. It returns plain tokens (never HTML), so callers render them as
// React text. State does not carry across lines (block comments and multi-line
// strings are only coloured on the line they start).

export type TokenKind = 'plain' | 'keyword' | 'string' | 'comment' | 'number' | 'func';

export interface Token {
  kind: TokenKind;
  text: string;
}

/** Lines longer than this are not tokenized (minified code, data blobs) */
export const MAX_HIGHLIGHT_LENGTH = 2000;

const KEYWORDS: Record<string, readonly string[]> = {
  Go: ['break', 'case', 'chan', 'const', 'continue', 'default', 'defer', 'else', 'fallthrough', 'for', 'func',
    'go', 'goto', 'if', 'import', 'interface', 'map', 'package', 'range', 'return', 'select', 'struct', 'switch',
    'type', 'var', 'nil', 'true', 'false', 'error', 'string', 'int', 'int64', 'bool', 'byte', 'float64', 'any'],
  Python: ['and', 'as', 'assert', 'async', 'await', 'break', 'class', 'continue', 'def', 'del', 'elif', 'else',
    'except', 'finally', 'for', 'from', 'global', 'if', 'import', 'in', 'is', 'lambda', 'None', 'nonlocal', 'not',
    'or', 'pass', 'raise', 'return', 'True', 'False', 'try', 'while', 'with', 'yield', 'self'],
  Kotlin: ['fun', 'val', 'var', 'class', 'object', 'if', 'else', 'when', 'for', 'while', 'return', 'null', 'true',
    'false', 'import', 'package', 'private', 'public', 'override', 'data', 'suspend', 'is', 'in'],
  Swift: ['func', 'let', 'var', 'class', 'struct', 'enum', 'if', 'else', 'guard', 'for', 'while', 'return', 'nil',
    'true', 'false', 'import', 'private', 'public', 'switch', 'case', 'self', 'in', 'throws', 'try'],
  TypeScript: ['const', 'let', 'var', 'function', 'return', 'if', 'else', 'for', 'while', 'switch', 'case', 'break',
    'continue', 'import', 'export', 'from', 'default', 'class', 'extends', 'new', 'this', 'null', 'undefined', 'true',
    'false', 'type', 'interface', 'async', 'await', 'try', 'catch', 'throw', 'of', 'in', 'typeof'],
  Shell: ['if', 'then', 'else', 'elif', 'fi', 'for', 'while', 'do', 'done', 'case', 'esac', 'function', 'return',
    'export', 'local', 'set', 'echo', 'exit', 'in'],
  YAML: ['true', 'false', 'null', 'yes', 'no', 'on'],
};

const HASH_COMMENT = new Set(['Python', 'Shell', 'YAML']);
const patternCache = new Map<string, RegExp | null>();

function patternFor(language: string): RegExp | null {
  const cached = patternCache.get(language);
  if (cached !== undefined) return cached;
  const keywords = KEYWORDS[language];
  let pattern: RegExp | null = null;
  if (keywords) {
    const comment = HASH_COMMENT.has(language) ? '(?:^|\\s)#.*$' : '\\/\\/.*$|\\/\\*.*?(?:\\*\\/|$)';
    const strings = '"(?:[^"\\\\]|\\\\.)*"?|\'(?:[^\'\\\\]|\\\\.)*\'?|`[^`]*`?';
    const source = `(${comment})|(${strings})|\\b(${keywords.join('|')})\\b|\\b(\\d[\\d_.xXa-fA-F]*)\\b|\\b([A-Za-z_]\\w*)(?=\\()`;
    pattern = new RegExp(source, 'g');
  }
  patternCache.set(language, pattern);
  return pattern;
}

function kindOf(match: RegExpMatchArray): TokenKind {
  if (match[1] !== undefined) return 'comment';
  if (match[2] !== undefined) return 'string';
  if (match[3] !== undefined) return 'keyword';
  if (match[4] !== undefined) return 'number';
  return 'func';
}

/** Split one line of code into coloured tokens; unknown languages yield a single plain token */
export function highlightLine(text: string, language: string): Token[] {
  const pattern = patternFor(language);
  if (!pattern || text.length > MAX_HIGHLIGHT_LENGTH) return text ? [{ kind: 'plain', text }] : [];
  const tokens: Token[] = [];
  let cursor = 0;
  for (const match of text.matchAll(pattern)) {
    const start = match.index ?? 0;
    if (start > cursor) tokens.push({ kind: 'plain', text: text.slice(cursor, start) });
    tokens.push({ kind: kindOf(match), text: match[0] });
    cursor = start + match[0].length;
  }
  if (cursor < text.length) tokens.push({ kind: 'plain', text: text.slice(cursor) });
  return tokens;
}

const EXTENSIONS: readonly [RegExp, string][] = [
  [/\.go$/, 'Go'],
  [/\.py$/, 'Python'],
  [/\.kts?$/, 'Kotlin'],
  [/\.swift$/, 'Swift'],
  [/\.(ts|tsx|js|jsx|mjs|cjs)$/, 'TypeScript'],
  [/\.(sh|bash|zsh)$/, 'Shell'],
  [/\.ya?ml$/, 'YAML'],
];

/** Highlighter language for a path ("" when unknown) */
export function languageForPath(path: string): string {
  return EXTENSIONS.find(([pattern]) => pattern.test(path))?.[1] ?? '';
}

/** Prefer the analyzer's language when the highlighter knows it, else infer from the path */
export function highlightLanguage(path: string, analyzerLanguage?: string): string {
  if (analyzerLanguage && KEYWORDS[analyzerLanguage]) return analyzerLanguage;
  return languageForPath(path);
}
