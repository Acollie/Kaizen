package review

import (
	"path/filepath"
	"strings"
)

// languageSyntax is what the line heuristics (mechanical-edit detection, the
// behaviour-change and decision-logic factors) need to know about a language:
// how it writes comments and literals, and which words are keywords
type languageSyntax struct {
	lineComments    []string // "//", "#"
	hashNeedsSpace  bool     // "#" starts a comment only at line start or after whitespace (shell: $#, ${#x})
	blockComments   bool     // /* ... */
	quotes          string   // single-line literal delimiters
	multilineQuotes []string // literal delimiters that may span lines: "`", `"""`, "'''"

	keywords      map[string]bool // case-sensitive; kept verbatim by the normaliser
	colonKeywords map[string]bool // keywords legitimately followed by ":" in key position (else:, default:)

	branches   map[string]bool // open a new path through the code
	throws     map[string]bool // raise an error the caller handles (throw, raise)
	panics     map[string]bool // abort the normal flow
	goroutines map[string]bool
	defers     map[string]bool
}

func wordSet(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, word := range words {
		set[word] = true
	}
	return set
}

var (
	goSyntax = &languageSyntax{
		lineComments: []string{"//"}, blockComments: true, quotes: `"'`, multilineQuotes: []string{"`"},
		keywords: wordSet("if", "else", "for", "range", "switch", "case", "default", "select", "go", "defer",
			"return", "break", "continue", "goto", "fallthrough", "func", "var", "const", "nil", "true", "false", "panic"),
		colonKeywords: wordSet("default"),
		branches:      wordSet("if", "for", "switch", "case", "select"),
		throws:        wordSet(),
		panics:        wordSet("panic"),
		goroutines:    wordSet("go"),
		defers:        wordSet("defer"),
	}
	pythonSyntax = &languageSyntax{
		lineComments: []string{"#"}, quotes: `"'`, multilineQuotes: []string{`"""`, `'''`},
		keywords: wordSet("if", "elif", "else", "for", "while", "match", "case", "try", "except", "finally",
			"raise", "return", "break", "continue", "def", "lambda", "class", "with", "yield", "await",
			"async", "pass", "None", "True", "False", "and", "or", "not", "in", "is"),
		colonKeywords: wordSet("else", "try", "except", "finally", "lambda"),
		branches:      wordSet("if", "elif", "for", "while", "match", "case", "except"),
		throws:        wordSet("raise"),
		panics:        wordSet(),
		goroutines:    wordSet(),
		defers:        wordSet(),
	}
	kotlinSyntax = &languageSyntax{
		lineComments: []string{"//"}, blockComments: true, quotes: `"'`, multilineQuotes: []string{`"""`},
		keywords: wordSet("if", "else", "for", "while", "do", "when", "try", "catch", "finally", "throw",
			"return", "break", "continue", "fun", "val", "var", "null", "true", "false", "in", "is", "as"),
		colonKeywords: wordSet(),
		branches:      wordSet("if", "for", "while", "when", "catch"),
		throws:        wordSet("throw"),
		panics:        wordSet(),
		goroutines:    wordSet(),
		defers:        wordSet(),
	}
	swiftSyntax = &languageSyntax{
		lineComments: []string{"//"}, blockComments: true, quotes: `"`, multilineQuotes: []string{`"""`},
		keywords: wordSet("if", "else", "guard", "for", "while", "repeat", "switch", "case", "default", "defer",
			"do", "try", "catch", "throw", "return", "break", "continue", "fallthrough", "func", "let", "var",
			"nil", "true", "false", "in", "is", "as", "await"),
		colonKeywords: wordSet("default"),
		branches:      wordSet("if", "guard", "for", "while", "switch", "case", "catch"),
		throws:        wordSet("throw"),
		panics:        wordSet(),
		goroutines:    wordSet(),
		defers:        wordSet("defer"),
	}
	javascriptSyntax = &languageSyntax{
		lineComments: []string{"//"}, blockComments: true, quotes: `"'`, multilineQuotes: []string{"`"},
		keywords: wordSet("if", "else", "for", "while", "do", "switch", "case", "default", "try", "catch",
			"finally", "throw", "return", "break", "continue", "function", "const", "let", "var", "null",
			"undefined", "true", "false", "new", "await", "async", "yield", "typeof", "instanceof", "in", "of"),
		colonKeywords: wordSet("default"),
		branches:      wordSet("if", "for", "while", "switch", "case", "catch"),
		throws:        wordSet("throw"),
		panics:        wordSet(),
		goroutines:    wordSet(),
		defers:        wordSet(),
	}
	// cFamilySyntax covers Java, C, C++, C# and Rust closely enough for these heuristics
	cFamilySyntax = &languageSyntax{
		lineComments: []string{"//"}, blockComments: true, quotes: `"'`,
		keywords: wordSet("if", "else", "for", "while", "do", "switch", "case", "default", "try", "catch",
			"finally", "throw", "return", "break", "continue", "goto", "match", "loop", "null", "true", "false", "new"),
		colonKeywords: wordSet("default"),
		branches:      wordSet("if", "for", "while", "switch", "case", "catch", "match", "loop"),
		throws:        wordSet("throw"),
		panics:        wordSet(),
		goroutines:    wordSet(),
		defers:        wordSet(),
	}
	shellSyntax = &languageSyntax{
		lineComments: []string{"#"}, hashNeedsSpace: true, quotes: `"'`,
		keywords: wordSet("if", "then", "elif", "else", "fi", "for", "while", "until", "do", "done", "case",
			"esac", "in", "function", "return", "exit", "break", "continue", "true", "false"),
		colonKeywords: wordSet(),
		branches:      wordSet("if", "elif", "for", "while", "until", "case"),
		throws:        wordSet(),
		panics:        wordSet(),
		goroutines:    wordSet(),
		defers:        wordSet(),
	}
	// yamlSyntax has no control flow: a key such as `if:` is data
	yamlSyntax = &languageSyntax{
		lineComments: []string{"#"}, hashNeedsSpace: true, quotes: `"'`,
		keywords: wordSet("true", "false", "null"), colonKeywords: wordSet(),
		branches: wordSet(), throws: wordSet(), panics: wordSet(), goroutines: wordSet(), defers: wordSet(),
	}
	// defaultSyntax is used when the language is unknown: C-style comments (as
	// before languages were told apart) and the union of the keyword sets
	defaultSyntax = unionSyntax(goSyntax, pythonSyntax, kotlinSyntax, swiftSyntax, javascriptSyntax)
)

// unionSyntax merges keyword sets, keeping C-style comments and literals
func unionSyntax(languages ...*languageSyntax) *languageSyntax {
	union := &languageSyntax{
		lineComments: []string{"//"}, blockComments: true, quotes: `"'` + "`",
		keywords: wordSet(), colonKeywords: wordSet(), branches: wordSet(), throws: wordSet(),
		panics: wordSet(), goroutines: wordSet(), defers: wordSet(),
	}
	for _, language := range languages {
		for _, pair := range []struct{ into, from map[string]bool }{
			{union.keywords, language.keywords}, {union.colonKeywords, language.colonKeywords},
			{union.branches, language.branches}, {union.throws, language.throws}, {union.panics, language.panics},
			{union.goroutines, language.goroutines}, {union.defers, language.defers},
		} {
			for word := range pair.from {
				pair.into[word] = true
			}
		}
	}
	return union
}

// syntaxByExtension maps lower-case file extensions to their syntax
var syntaxByExtension = map[string]*languageSyntax{
	".go": goSyntax,
	".py": pythonSyntax, ".pyi": pythonSyntax,
	".kt": kotlinSyntax, ".kts": kotlinSyntax,
	".swift": swiftSyntax,
	".js":    javascriptSyntax, ".jsx": javascriptSyntax, ".mjs": javascriptSyntax, ".cjs": javascriptSyntax,
	".ts": javascriptSyntax, ".tsx": javascriptSyntax, ".mts": javascriptSyntax, ".cts": javascriptSyntax,
	".java": cFamilySyntax, ".c": cFamilySyntax, ".h": cFamilySyntax, ".cc": cFamilySyntax, ".cpp": cFamilySyntax,
	".hpp": cFamilySyntax, ".cs": cFamilySyntax, ".rs": cFamilySyntax, ".scala": cFamilySyntax,
	".sh": shellSyntax, ".bash": shellSyntax, ".zsh": shellSyntax, ".rb": shellSyntax,
	".yml": yamlSyntax, ".yaml": yamlSyntax, ".toml": yamlSyntax,
}

// syntaxByLanguage maps the analyzer language names to their syntax
var syntaxByLanguage = map[string]*languageSyntax{
	"Go": goSyntax, "Python": pythonSyntax, "Kotlin": kotlinSyntax, "Swift": swiftSyntax,
}

// syntaxFor picks the syntax for a file from its extension, then its analyzer
// language, falling back to defaultSyntax
func syntaxFor(path, language string) *languageSyntax {
	if syntax, ok := syntaxByExtension[strings.ToLower(filepath.Ext(path))]; ok {
		return syntax
	}
	if syntax, ok := syntaxByLanguage[language]; ok {
		return syntax
	}
	base := strings.ToLower(filepath.Base(path))
	if base == "makefile" || strings.HasPrefix(base, "dockerfile") {
		return shellSyntax
	}
	return defaultSyntax
}

// commentScanner removes comments from consecutive lines of one side of a diff.
// It tracks block comments and multi-line literals across the lines it is fed,
// so prose inside a /* ... */ block, a docstring or a multi-line raw string is
// not mistaken for code. It is a heuristic: a hunk that starts inside a block
// comment is not detected, and one that starts inside a multi-line literal is
// recognised only by a line that begins with the closing delimiter (`) or `,).
type commentScanner struct {
	syntax        *languageSyntax
	blankLiterals bool   // replace literal contents with "" (pattern matching) instead of keeping them (comparison)
	inBlock       bool   // inside /* ... */
	openQuote     string // the multi-line literal delimiter currently open, or ""
	sawMultiline  bool   // a multi-line literal has opened or closed on an earlier line
}

func newCommentScanner(syntax *languageSyntax, blankLiterals bool) *commentScanner {
	if syntax == nil {
		syntax = defaultSyntax
	}
	return &commentScanner{syntax: syntax, blankLiterals: blankLiterals}
}

// cleanLines strips comments from consecutive lines of one file
func cleanLines(lines []string, syntax *languageSyntax) []string {
	scanner := newCommentScanner(syntax, false)
	cleaned := make([]string, len(lines))
	for index, line := range lines {
		cleaned[index] = scanner.clean(line)
	}
	return cleaned
}

// clean returns the line's code with comments removed. Literals are kept
// verbatim, or blanked to "" when blankLiterals is set; the content of a
// multi-line literal is re-quoted so each line's share of it is one literal.
func (scanner *commentScanner) clean(line string) string {
	var code strings.Builder
	position := scanner.resume(line, &code)
	for position < len(line) {
		rest := line[position:]
		if scanner.startsLineComment(line, position) {
			break
		}
		if scanner.syntax.blockComments && strings.HasPrefix(rest, "/*") {
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				scanner.inBlock = true
				break
			}
			code.WriteByte(' ')
			position += end + 4
			continue
		}
		if quote := scanner.multilineQuoteAt(rest); quote != "" {
			position = scanner.consumeMultiline(line, position, quote, &code)
			continue
		}
		if strings.IndexByte(scanner.syntax.quotes, rest[0]) >= 0 {
			closing := skipLiteral(line, position)
			scanner.writeLiteral(&code, line[position+1:min(closing, len(line))])
			position = closing + 1
			continue
		}
		code.WriteByte(rest[0])
		position++
	}
	return code.String()
}

// resume finishes a block comment or multi-line literal left open by an earlier
// line and returns where the line's code starts (len(line) when none does)
func (scanner *commentScanner) resume(line string, code *strings.Builder) int {
	if !scanner.sawMultiline && scanner.openQuote == "" && !scanner.inBlock {
		if end, quote := scanner.leadingCloser(line); quote != "" {
			// the hunk started inside a multi-line literal that this line closes
			scanner.sawMultiline = true
			scanner.writeLiteral(code, line[:end])
			return end + len(quote)
		}
	}
	switch {
	case scanner.openQuote != "":
		quote := scanner.openQuote
		end := strings.Index(line, quote)
		if end < 0 {
			scanner.writeLiteral(code, line)
			return len(line)
		}
		scanner.writeLiteral(code, line[:end])
		scanner.openQuote = ""
		scanner.sawMultiline = true
		return end + len(quote)
	case scanner.inBlock:
		end := strings.Index(line, "*/")
		if end < 0 {
			return len(line)
		}
		scanner.inBlock = false
		return end + 2
	case scanner.syntax.blockComments && isCommentContinuation(line):
		return len(line)
	}
	return 0
}

// leadingCloser returns the position and delimiter of a multi-line literal
// delimiter that starts the line (after indentation) and is followed by the
// punctuation that ends an expression, as in "`)" or "`, value)"; a lone "`"
// counts too, since a raw string's opener has the string's text after it
func (scanner *commentScanner) leadingCloser(line string) (int, string) {
	start := len(line) - len(strings.TrimLeft(line, " \t"))
	for _, quote := range scanner.syntax.multilineQuotes {
		if !strings.HasPrefix(line[start:], quote) {
			continue
		}
		after := strings.TrimSpace(line[start+len(quote):])
		if (after == "" && quote == "`") || (after != "" && strings.ContainsRune("),;.}]+", rune(after[0]))) {
			return start, quote
		}
	}
	return 0, ""
}

// isCommentContinuation reports whether a line is the middle or end of a block
// comment written in the " * text" style
func isCommentContinuation(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "*") {
		return false
	}
	return len(trimmed) == 1 || trimmed[1] == ' ' || trimmed[1] == '/' || trimmed[1] == '*'
}

func (scanner *commentScanner) startsLineComment(line string, position int) bool {
	for _, marker := range scanner.syntax.lineComments {
		if !strings.HasPrefix(line[position:], marker) {
			continue
		}
		if marker == "#" && scanner.syntax.hashNeedsSpace && position > 0 && line[position-1] != ' ' && line[position-1] != '\t' {
			continue
		}
		return true
	}
	return false
}

func (scanner *commentScanner) multilineQuoteAt(rest string) string {
	for _, quote := range scanner.syntax.multilineQuotes {
		if strings.HasPrefix(rest, quote) {
			return quote
		}
	}
	return ""
}

// consumeMultiline handles a literal opened at position with a multi-line delimiter
func (scanner *commentScanner) consumeMultiline(line string, position int, quote string, code *strings.Builder) int {
	start := position + len(quote)
	end := strings.Index(line[start:], quote)
	scanner.sawMultiline = true
	if end < 0 {
		scanner.openQuote = quote
		scanner.writeLiteral(code, line[start:])
		return len(line)
	}
	scanner.writeLiteral(code, line[start:start+end])
	return start + end + len(quote)
}

// writeLiteral writes a literal as "content" (quotes and backslashes removed
// from the content), or as "" when literals are blanked
func (scanner *commentScanner) writeLiteral(code *strings.Builder, content string) {
	code.WriteByte('"')
	if !scanner.blankLiterals {
		code.WriteString(strings.NewReplacer(`"`, "", "'", "", "`", "", `\`, "").Replace(content))
	}
	code.WriteByte('"')
}
