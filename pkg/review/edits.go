package review

import (
	"fmt"
	"slices"
	"strings"
)

// Mechanical-edit detection. A changed function whose removed and added lines
// are, almost all, rewrites of the same statement (a rename, a reformat, or an
// API swap such as sb.WriteString(fmt.Sprintf(...)) -> fmt.Fprintf(&sb, ...))
// changes little behaviour, so its pre-existing complexity should not rank it
// above code that actually changes what the program does.
const (
	// RewriteSimilarity is the minimum normalised token similarity for a removed
	// and an added line to count as a rewrite of the same statement
	RewriteSimilarity = 0.8
	// MechanicalShare is the share of a function's changed lines that must be
	// rewrites for the edit to count as mechanical
	MechanicalShare = 0.8
	// MechanicalEditDiscount scales the intrinsic factors (complexity, nesting,
	// maintainability, lines changed) of a mechanical edit
	MechanicalEditDiscount = 0.3
)

// functionEdit is what a PR changed inside one function
type functionEdit struct {
	added         []string // every added line in the function, in order
	unpairedAdded []string // added lines that are not a rewrite of a removed line
	changed       int      // non-blank added plus removed lines
	rewritten     int      // lines (added plus removed) that belong to a rewrite pair
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
// lines [start, end] and pairs removed lines with their rewrites, hunk by hunk
func analyzeFunctionEdit(file *FileDiff, start, end int) functionEdit {
	var edit functionEdit
	if file == nil || end < start {
		return edit
	}
	for _, hunk := range file.Hunks {
		removed, added := hunkChangesInRange(hunk, start, end)
		edit.added = append(edit.added, added...)
		removed, added = nonBlank(removed), nonBlank(added)
		edit.changed += len(removed) + len(added)
		pairedAdded := pairRewrites(removed, added)
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

// hunkChangesInRange returns the removed lines whose removal point is inside
// [start, end] and the added lines inside it
func hunkChangesInRange(hunk Hunk, start, end int) (removed, added []string) {
	nextHeadLine := hunk.NewStart
	for _, line := range hunk.Lines {
		switch line.Kind {
		case LineRemoved:
			if nextHeadLine >= start && nextHeadLine <= end {
				removed = append(removed, line.Text)
			}
			continue
		case LineAdded:
			if line.NewLine >= start && line.NewLine <= end {
				added = append(added, line.Text)
			}
		}
		nextHeadLine = line.NewLine + 1
	}
	return removed, added
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
func pairRewrites(removed, added []string) []bool {
	paired := make([]bool, len(added))
	addedShapes := make([]lineShape, len(added))
	for index, line := range added {
		addedShapes[index] = shapeOf(line)
	}
	for _, line := range removed {
		removedShape := shapeOf(line)
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

// lineShape is a line reduced for comparison: tokens with identifiers and
// string literals collapsed, the meaning-bearing tokens, and the raw identifiers
type lineShape struct {
	tokens      []string
	critical    []string // keywords, operators that change meaning, and number literals, sorted
	identifiers []string
}

func shapeOf(line string) lineShape {
	var shape lineShape
	for _, token := range tokenizeLine(line) {
		switch {
		case token.kind == tokenIdentifier:
			shape.tokens = append(shape.tokens, "id")
			shape.identifiers = append(shape.identifiers, token.text)
		case token.kind == tokenString:
			shape.tokens = append(shape.tokens, "str")
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
	slices.Sort(shape.critical)
	return shape
}

// rewriteScore returns the similarity of two lines and whether the added line
// is a rewrite of the removed one: similar token shape, the same keywords,
// operators and numbers, and not merely the same identifiers in another order
// (a swapped argument list is a behaviour change)
func rewriteScore(removed, added lineShape) (float64, bool) {
	if len(removed.tokens) == 0 || len(added.tokens) == 0 {
		return 0, false
	}
	if !slices.Equal(removed.critical, added.critical) {
		return 0, false
	}
	if isReordering(removed.identifiers, added.identifiers) {
		return 0, false
	}
	score := tokenSimilarity(removed.tokens, added.tokens)
	return score, score >= RewriteSimilarity
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
	kind tokenKind
	text string // lower-case
}

// lineKeywords are kept verbatim by the normaliser (Go, Python, Kotlin, Swift)
var lineKeywords = map[string]bool{
	"if": true, "else": true, "elif": true, "for": true, "while": true, "range": true, "switch": true,
	"case": true, "default": true, "select": true, "when": true, "guard": true, "match": true,
	"go": true, "defer": true, "return": true, "break": true, "continue": true, "goto": true,
	"fallthrough": true, "panic": true, "raise": true, "throw": true, "try": true, "catch": true,
	"except": true, "finally": true, "nil": true, "null": true, "none": true, "true": true,
	"false": true, "and": true, "or": true, "not": true, "in": true, "is": true, "await": true,
	"func": true, "fun": true, "def": true, "var": true, "val": true, "let": true, "const": true,
}

// meaningfulOperators are punctuation tokens whose change alters behaviour
var meaningfulOperators = map[string]bool{
	"==": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true, "&&": true, "||": true,
	"!": true, "+": true, "-": true, "/": true, "%": true, "<-": true, "++": true, "--": true,
	"+=": true, "-=": true, "<<": true, ">>": true, "^": true, "|": true, "??": true,
}

// pointerOperators are ignored when comparing lines
var pointerOperators = map[string]bool{"&": true, "*": true}

// multiCharOperators are matched before single characters, longest first
var multiCharOperators = []string{
	"...", "<<=", ">>=", "&^=", "==", "!=", "<=", ">=", "&&", "||", ":=", "<-", "++", "--",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>", "&^", "->", "=>", "??", "::",
}

// tokenizeLine splits one line of code into lower-cased tokens, dropping a
// trailing // comment and merging qualified names (a.b.c) into one identifier
func tokenizeLine(line string) []lineToken {
	var tokens []lineToken
	for position := 0; position < len(line); {
		character := line[position]
		switch {
		case character == ' ' || character == '\t' || character == '\r':
			position++
		case strings.HasPrefix(line[position:], "//"):
			return mergeQualifiedNames(tokens)
		case character == '"' || character == '\'' || character == '`':
			position = skipLiteral(line, position) + 1
			tokens = append(tokens, lineToken{kind: tokenString, text: "str"})
		case isWordStart(character):
			end := wordEnd(line, position)
			word := strings.ToLower(line[position:end])
			kind := tokenIdentifier
			if lineKeywords[word] {
				kind = tokenKeyword
			}
			tokens = append(tokens, lineToken{kind: kind, text: word})
			position = end
		case character >= '0' && character <= '9':
			end := wordEnd(line, position)
			tokens = append(tokens, lineToken{kind: tokenNumber, text: strings.ToLower(line[position:end])})
			position = end
		default:
			operator := operatorAt(line, position)
			tokens = append(tokens, lineToken{kind: tokenPunctuation, text: operator})
			position += len(operator)
		}
	}
	return mergeQualifiedNames(tokens)
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
			index += 2
		}
		merged = append(merged, token)
	}
	return merged
}
