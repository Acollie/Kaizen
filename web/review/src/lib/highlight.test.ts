import { highlightLanguage, highlightLine, languageForPath, MAX_HIGHLIGHT_LENGTH } from './highlight';

const kinds = (text: string, language: string) =>
  highlightLine(text, language).filter((token) => token.kind !== 'plain').map((token) => [token.kind, token.text]);

describe('highlightLine', () => {
  it('colours Go keywords, strings, numbers, calls and comments', () => {
    expect(kinds('if x := parse("a", 42); x { // done', 'Go')).toEqual([
      ['keyword', 'if'], ['func', 'parse'], ['string', '"a"'], ['number', '42'], ['comment', '// done'],
    ]);
  });

  it('keeps every character in order', () => {
    const text = 'return fmt.Sprintf("%d", count) /* note */';
    expect(highlightLine(text, 'Go').map((token) => token.text).join('')).toBe(text);
  });

  it('does not treat keywords inside strings as keywords', () => {
    expect(kinds('x := "if for return"', 'Go')).toEqual([['string', '"if for return"']]);
  });

  it('uses hash comments for Python, Shell and YAML', () => {
    expect(kinds('x = 1  # note', 'Python')).toContainEqual(['comment', ' # note']);
    expect(kinds('run: echo hi # note', 'YAML')).toContainEqual(['comment', ' # note']);
  });

  it('returns plain text for unknown languages and very long lines', () => {
    expect(highlightLine('if x', '')).toEqual([{ kind: 'plain', text: 'if x' }]);
    const long = 'if '.repeat(MAX_HIGHLIGHT_LENGTH);
    expect(highlightLine(long, 'Go')).toEqual([{ kind: 'plain', text: long }]);
    expect(highlightLine('', 'Go')).toEqual([]);
  });
});

describe('languageForPath / highlightLanguage', () => {
  it('maps extensions', () => {
    expect(languageForPath('a/b.go')).toBe('Go');
    expect(languageForPath('action.yml')).toBe('YAML');
    expect(languageForPath('test.sh')).toBe('Shell');
    expect(languageForPath('x.tsx')).toBe('TypeScript');
    expect(languageForPath('README.md')).toBe('');
  });

  it('prefers a known analyzer language and falls back to the path', () => {
    expect(highlightLanguage('x.kt', 'Kotlin')).toBe('Kotlin');
    expect(highlightLanguage('x.yml', '')).toBe('YAML');
    expect(highlightLanguage('x.rs', 'Rust')).toBe('');
  });
});
