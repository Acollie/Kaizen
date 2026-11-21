package review

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Mechanical-edit detection. A changed function whose removed and added lines
// are, almost all, rewrites of the same statement (a reformat, or an API swap
// such as sb.WriteString(fmt.Sprintf(...)) -> fmt.Fprintf(&sb, ...)) changes
// little behaviour, so its pre-existing complexity should not rank it above
// code that actually changes what the program does.
const (
	// RewriteSimilarity is the minimum normalised token similarity for a removed
	// and an added line to count as a rewrite of the same statement
	RewriteSimilarity = 0.8
	// IdentifierSimilarity is the minimum share of identifier parts and literal
	// values two lines must share to be a rewrite: a line with the same shape
	// but other names or strings does something else (isAdmin(user) -> isGuest(user))
	IdentifierSimilarity = 0.7
	// MechanicalShare is the share of a function's changed lines that must be
	// rewrites for the edit to count as mechanical
	MechanicalShare = 0.8
	// MechanicalEditDiscount scales the intrinsic factors (complexity, nesting,
	// maintainability, lines changed) of a mechanical edit
	MechanicalEditDiscount = 0.3
)

// functionEdit is what a PR changed inside one function
type functionEdit struct {
	syntax        *languageSyntax
	added         []string // every added line in the function, in order, as written
	unpairedAdded []string // added lines, comments removed, that are not a rewrite of a removed line
	changed       int      // non-blank added plus removed lines, comments removed
	rewritten     int      // lines (added plus removed) that belong to a rewrite pair
}

// languageSyntax returns the syntax the edit's lines are written in
func (edit functionEdit) languageSyntax() *languageSyntax {
	if edit.syntax == nil {
		return defaultSyntax
	}
	return edit.syntax
}

// isMechanical reports whether nearly every changed line is a rewrite of the same statement
func (edit functionEdit) isMechanical() bool {
	return edit.changed >= 2 && float64(edit.rewritten) >= MechanicalShare*float64(edit.changed)
}

// mechanicalReason explains the discount
func (edit functionEdit) mechanicalReason() string {
	return fmt.Sprintf("mechanical edit (%d of %d changed lines are rewrites of the same statement): complexity weighted down", edit.rewritten, edit.changed)
}

// analyzeFunctionEdit collects the changed lines of the function at head-side
// lines [start, end], strips their comments in the file's language, and pairs
// removed lines with their rewrites, hunk by hunk
func analyzeFunctionEdit(file *FileDiff, start, end int) functionEdit {
	var edit functionEdit
	if file == nil || end < start {
		return edit
	}
	edit.syntax = syntaxFor(file.Path, file.Language)
	for _, hunk := range file.Hunks {
		changes := hunkChangesInRange(hunk, start, end, edit.syntax)
		edit.added = append(edit.added, changes.rawAdded...)
		removed, added := nonBlank(changes.removed), nonBlank(changes.added)
		edit.changed += len(removed) + len(added)
		pairedAdded := pairRewrites(removed, added, edit.syntax)
		for index, line := range added {
			if pairedAdded[index] {
				edit.rewritten += 2
				continue
			}
			edit.unpairedAdded = append(edit.unpairedAdded, line)
		}
	}
	return edit
}

// hunkChanges are one hunk's changed lines inside a function
type hunkChanges struct {
	rawAdded []string // added lines as written
	removed  []string // removed lines, comments removed
	added    []string // added lines, comments removed
}

// hunkChangesInRange returns the removed lines whose removal point is inside
// [start, end] and the added lines inside it. Each side of the hunk is scanned
// in order, context lines included, so a block comment or multi-line literal
// opened on one line is recognised on the next.
func hunkChangesInRange(hunk Hunk, start, end int, syntax *languageSyntax) hunkChanges {
	var changes hunkChanges
	headScanner, baseScanner := newCommentScanner(syntax, false), newCommentScanner(syntax, false)
	nextHeadLine := hunk.NewStart
	for _, line := range hunk.Lines {
		switch line.Kind {
		case LineRemoved:
			code := baseScanner.clean(line.Text)
			if nextHeadLine >= start && nextHeadLine <= end {
				changes.removed = append(changes.removed, code)
			}
			continue
		case LineAdded:
			code := headScanner.clean(line.Text)
			if line.NewLine >= start && line.NewLine <= end {
				changes.rawAdded = append(changes.rawAdded, line.Text)
				changes.added = append(changes.added, code)
			}
		default:
			headScanner.clean(line.Text)
			baseScanner.clean(line.Text)
		}
		nextHeadLine = line.NewLine + 1
	}
	return changes
}

func nonBlank(lines []string) []string {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			kept = append(kept, line)
		}
	}
	return kept
}

// pairRewrites matches each removed line with the most similar unmatched added
// line that rewrites the same statement, and reports which added lines matched
func pairRewrites(removed, added []string, syntax *languageSyntax) []bool {
	paired := make([]bool, len(added))
	addedShapes := make([]lineShape, len(added))
	for index, line := range added {
		addedShapes[index] = shapeOf(line, syntax)
	}
	for _, line := range removed {
		removedShape := shapeOf(line, syntax)
		best, bestScore := -1, 0.0
		for index := range added {
			if paired[index] {
				continue
			}
			if score, ok := rewriteScore(removedShape, addedShapes[index]); ok && score > bestScore {
				best, bestScore = index, score
			}
		}
		if best >= 0 {
			paired[best] = true
		}
	}
	return paired
}

// lineShape is a line of code (comments already removed) reduced for comparison
type lineShape struct {
	tokens      []string // identifiers and string literals collapsed to placeholders
	critical    []string // keywords, operators that change meaning, and number literals, sorted
	identifiers []string // qualified names, in order
	values      []string // identifier parts and literal contents, output calls folded together
	calls       []string // distinct called names, output calls folded together, sorted
	urls        []string // literal contents that look like URLs, sorted
	sensitive   []string // security and side-effect patterns the line uses, sorted
}

// formatCallFamily are output and formatting calls an API rewrite swaps freely
// (sb.WriteString(fmt.Sprintf(...)) -> fmt.Fprintf(&sb, ...)); they compare as one name
var formatCallFamily = wordSet("print", "printf", "println", "sprint", "sprintf", "sprintln",
	"fprint", "fprintf", "fprintln", "writestring", "writebyte", "writerune")

const formatCallPlaceholder = "<format>"

func shapeOf(line string, syntax *languageSyntax) lineShape {
	var shape lineShape
	tokens := tokenizeLine(line, syntax)
	calls := map[string]bool{}
	for index, token := range tokens {
		switch {
		case token.kind == tokenIdentifier:
			shape.tokens = append(shape.tokens, "id")
			shape.identifiers = append(shape.identifiers, token.text)
			for _, part := range strings.Split(token.text, ".") {
				shape.values = append(shape.values, foldFormatCall(part))
			}
			if index+1 < len(tokens) && tokens[index+1].text == "(" {
				calls[callName(token.text)] = true
			}
		case token.kind == tokenString:
			shape.tokens = append(shape.tokens, "str")
			shape.values = append(shape.values, `"`+token.value)
			if strings.Contains(token.value, "://") {
				shape.urls = append(shape.urls, token.value)
			}
		case pointerOperators[token.text]:
			// & and * (address-of, dereference) come and go in API rewrites
			// such as Sprintf -> Fprintf(&sb, ...); they are left out of the comparison
		default:
			shape.tokens = append(shape.tokens, token.text)
			if token.kind != tokenPunctuation || meaningfulOperators[token.text] {
				shape.critical = append(shape.critical, token.text)
			}
		}
	}
	for name := range calls {
		shape.calls = append(shape.calls, name)
	}
	slices.Sort(shape.calls)
	slices.Sort(shape.critical)
	slices.Sort(shape.urls)
	shape.sensitive = sensitivePatterns(line)
	return shape
}

func foldFormatCall(part string) string {
	if formatCallFamily[part] {
		return formatCallPlaceholder
	}
	return part
}

// callName is a called (lower-case, qualified) name, or the format placeholder
// for an output call, whose receiver an API rewrite moves around
func callName(qualified string) string {
	if formatCallFamily[qualified[strings.LastIndex(qualified, ".")+1:]] {
		return formatCallPlaceholder
	}
	return qualified
}

// rewriteScore returns the similarity of two lines and whether the added line
// is a rewrite of the removed one. A rewrite has a similar token shape, mostly
// the same identifiers and literals, and exactly the same keywords, operators,
// numbers, boolean literals, called functions, URLs and security or
// side-effect patterns. The same identifiers in another order (a swapped
// argument list) are a behaviour change, not a rewrite.
func rewriteScore(removed, added lineShape) (float64, bool) {
	if len(removed.tokens) == 0 || len(added.tokens) == 0 {
		return 0, false
	}
	if !slices.Equal(removed.critical, added.critical) || !callsMatch(removed.calls, added.calls) ||
		!slices.Equal(removed.urls, added.urls) || !slices.Equal(removed.sensitive, added.sensitive) {
		return 0, false
	}
	if isReordering(removed.identifiers, added.identifiers) {
		return 0, false
	}
	score := tokenSimilarity(removed.tokens, added.tokens)
	if score < RewriteSimilarity || valueSimilarity(removed.values, added.values) < IdentifierSimilarity {
		return score, false
	}
	return score, true
}

// callsMatch reports whether two lines call the same functions. A call may gain
// or lose a qualifier (openInBrowser -> visualization.OpenInBrowser, a function
// moved to a package) but not change it (cache.Remove -> os.Remove).
func callsMatch(left, right []string) bool {
	leftQualifiers, rightQualifiers := qualifiersByName(left), qualifiersByName(right)
	if len(leftQualifiers) != len(rightQualifiers) {
		return false
	}
	for name, leftSet := range leftQualifiers {
		rightSet, ok := rightQualifiers[name]
		if !ok {
			return false
		}
		if len(leftSet) > 0 && len(rightSet) > 0 && !maps.Equal(leftSet, rightSet) {
			return false
		}
	}
	return true
}

// qualifiersByName groups called names by their last part, collecting the
// qualifiers each is called with (unqualified calls add none)
func qualifiersByName(calls []string) map[string]map[string]bool {
	grouped := make(map[string]map[string]bool, len(calls))
	for _, call := range calls {
		dot := strings.LastIndex(call, ".")
		name := call[dot+1:]
		if grouped[name] == nil {
			grouped[name] = map[string]bool{}
		}
		if dot > 0 {
			grouped[name][call[:dot]] = true
		}
	}
	return grouped
}

func isReordering(left, right []string) bool {
	if slices.Equal(left, right) {
		return false
	}
	sortedLeft, sortedRight := slices.Clone(left), slices.Clone(right)
	slices.Sort(sortedLeft)
	slices.Sort(sortedRight)
	return slices.Equal(sortedLeft, sortedRight)
}

// valueSimilarity is the Dice coefficient of two multisets of identifier parts
// and literal values: 2*shared / (len(left)+len(right)), 1 when both are empty.
// It ignores order because an API rewrite moves the same names around
// (sb.WriteString(fmt.Sprintf(f, n)) -> fmt.Fprintf(&sb, f, n)); a pure
// reordering of the same names is rejected separately by isReordering.
func valueSimilarity(left, right []string) float64 {
	if len(left)+len(right) == 0 {
		return 1
	}
	remaining := make(map[string]int, len(left))
	for _, value := range left {
		remaining[value]++
	}
	shared := 0
	for _, value := range right {
		if remaining[value] > 0 {
			remaining[value]--
			shared++
		}
	}
	return 2 * float64(shared) / float64(len(left)+len(right))
}

// tokenSimilarity is 2*LCS / (len(left)+len(right)), 1 for identical sequences
func tokenSimilarity(left, right []string) float64 {
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for _, leftToken := range left {
		for column, rightToken := range right {
			if leftToken == rightToken {
				current[column+1] = previous[column] + 1
			} else {
				current[column+1] = max(previous[column+1], current[column])
			}
		}
		previous, current = current, previous
	}
	return 2 * float64(previous[len(right)]) / float64(len(left)+len(right))
}

type tokenKind int

const (
	tokenIdentifier tokenKind = iota
	tokenKeyword
	tokenNumber
	tokenString
	tokenPunctuation
)

type lineToken struct {
	kind  tokenKind
	text  string // lower-case
	value string // as written; a literal's content
}

// meaningfulOperators are punctuation tokens whose change alters behaviour
var meaningfulOperators = map[string]bool{
	"==": true, "!=": true, "===": true, "!==": true, "<": true, "<=": true, ">": true, ">=": true,
	"&&": true, "||": true, "!": true, "+": true, "-": true, "/": true, "%": true, "<-": true, "++": true,
	"--": true, "+=": true, "-=": true, "<<": true, ">>": true, "^": true, "|": true, "??": true,
}

// assignmentOperators follow a word that is being assigned, which makes it a name
var assignmentOperators = wordSet("=", ":=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"<<=", ">>=", "&^=", "??=", "||=", "&&=")

// pointerOperators are ignored when comparing lines
var pointerOperators = map[string]bool{"&": true, "*": true}

// multiCharOperators are matched before single characters, longest first
var multiCharOperators = []string{
	"===", "!==", "...", "<<=", ">>=", "&^=", "??=", "||=", "&&=", "==", "!=", "<=", ">=", "&&", "||",
	":=", "<-", "++", "--", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>", "&^", "->",
	"=>", "??", "::",
}

// tokenizeLine splits one line of code (comments already removed, see
// commentScanner) into tokens, marks the language's keywords, and merges
// qualified names (a.b.c) into one identifier
func tokenizeLine(line string, syntax *languageSyntax) []lineToken {
	if syntax == nil {
		syntax = defaultSyntax
	}
	tokens := scanTokens(line)
	markKeywords(tokens, syntax)
	return mergeQualifiedNames(tokens)
}

func scanTokens(line string) []lineToken {
	var tokens []lineToken
	for position := 0; position < len(line); {
		character := line[position]
		switch {
		case character == ' ' || character == '\t' || character == '\r':
			position++
		case character == '"' || character == '\'' || character == '`':
			closing := skipLiteral(line, position)
			tokens = append(tokens, lineToken{kind: tokenString, text: "str", value: line[position+1 : min(closing, len(line))]})
			position = closing + 1
		case isWordStart(character):
			end := wordEnd(line, position)
			word := line[position:end]
			tokens = append(tokens, lineToken{kind: tokenIdentifier, text: strings.ToLower(word), value: word})
			position = end
		case character >= '0' && character <= '9':
			end := wordEnd(line, position)
			tokens = append(tokens, lineToken{kind: tokenNumber, text: strings.ToLower(line[position:end])})
			position = end
		default:
			operator := operatorAt(line, position)
			tokens = append(tokens, lineToken{kind: tokenPunctuation, text: operator, value: operator})
			position += len(operator)
		}
	}
	return tokens
}

// markKeywords turns words in the language's (case-sensitive) keyword set into
// keywords, except where the word is used as a name: after "." (re.match,
// db.Select), before an assignment (go = 3, val when = 3), or as a key at the
// start of a literal or argument ({if: 1}, Go: true)
func markKeywords(tokens []lineToken, syntax *languageSyntax) {
	for index := range tokens {
		token := &tokens[index]
		if token.kind != tokenIdentifier || !syntax.keywords[token.value] {
			continue
		}
		previous, next := "", ""
		if index > 0 {
			previous = tokens[index-1].text
		}
		if index+1 < len(tokens) {
			next = tokens[index+1].text
		}
		keyPosition := previous == "" || previous == "{" || previous == "," || previous == "("
		isKey := next == ":" && keyPosition && !syntax.colonKeywords[token.value]
		if previous == "." || assignmentOperators[next] || isKey {
			continue
		}
		token.kind = tokenKeyword
	}
}

func isWordStart(character byte) bool {
	return character == '_' || character == '$' || character == '@' ||
		(character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character >= 0x80
}

func wordEnd(line string, start int) int {
	end := start + 1
	for end < len(line) && (isWordStart(line[end]) || (line[end] >= '0' && line[end] <= '9')) {
		end++
	}
	return end
}

func operatorAt(line string, position int) string {
	for _, operator := range multiCharOperators {
		if strings.HasPrefix(line[position:], operator) {
			return operator
		}
	}
	return line[position : position+1]
}

// mergeQualifiedNames turns identifier "." identifier runs into one identifier,
// so a method or package-qualified call compares as a single name
func mergeQualifiedNames(tokens []lineToken) []lineToken {
	merged := make([]lineToken, 0, len(tokens))
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		for token.kind == tokenIdentifier && index+2 < len(tokens) &&
			tokens[index+1].text == "." && tokens[index+2].kind == tokenIdentifier {
			token.text += "." + tokens[index+2].text
			token.value += "." + tokens[index+2].value
			index += 2
		}
		merged = append(merged, token)
	}
	return merged
}
