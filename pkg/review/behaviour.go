package review

import (
	"fmt"
	"slices"
	"strings"
)

// Behaviour-change factor: added lines that introduce new control flow or new
// side effects change what the program does, whatever the function's existing
// complexity. Each new branch, error return, defer or panic is one unit, a
// goroutine or side effect two; the factor is full at BehaviourChangeFullUnits.
const (
	// WeightBehaviourChange is part of the 100 the weights in risk.go sum to
	WeightBehaviourChange    = 15.0
	BehaviourChangeFullUnits = 10.0
	// behaviourReasonMinUnits keeps a single new if out of the reasons
	behaviourReasonMinUnits = 2
)

// behaviourChange counts what the new lines add
type behaviourChange struct {
	branches, errorReturns, goroutines, defers, panics int
	sideEffects                                        []string // distinct patterns, in first-seen order
}

func (change behaviourChange) units() int {
	return change.branches + change.errorReturns + change.defers + change.panics +
		2*change.goroutines + 2*len(change.sideEffects)
}

// countBehaviourChange inspects added lines (comments removed) that are not
// rewrites of a removed line
func countBehaviourChange(lines []string, syntax *languageSyntax) behaviourChange {
	if syntax == nil {
		syntax = defaultSyntax
	}
	var change behaviourChange
	for _, line := range lines {
		change.addLine(line, syntax)
	}
	return change
}

func (change *behaviourChange) addLine(line string, syntax *languageSyntax) {
	tokens := tokenizeLine(line, syntax)
	for index, token := range tokens {
		if token.kind != tokenKeyword {
			continue
		}
		switch word := token.value; {
		case syntax.branches[word]:
			change.branches++
		case syntax.goroutines[word]:
			change.goroutines++
		case syntax.defers[word]:
			change.defers++
		case syntax.panics[word]:
			change.panics++
		case syntax.throws[word]:
			change.errorReturns++
		case word == "return" && returnsError(tokens[index+1:]):
			change.errorReturns++
		}
	}
	if pattern := firstSideEffect(line); pattern != "" && !slices.Contains(change.sideEffects, pattern) {
		change.sideEffects = append(change.sideEffects, pattern)
	}
}

// returnsError reports whether one of a return statement's operands is an
// error value: err, nil, err, fmt.Errorf(...), errors.New(...), wrapErr(...),
// &ParseError{...}. Comparisons (err == nil) and counts (errorCount) are not.
func returnsError(operands []lineToken) bool {
	for _, operand := range splitOperands(operands) {
		if isErrorValue(operand) {
			return true
		}
	}
	return false
}

// splitOperands splits a return statement's tokens at top-level commas, stopping
// at a closing bracket or ";" that ends the statement (if x { return err })
func splitOperands(tokens []lineToken) [][]lineToken {
	var operands [][]lineToken
	depth, start := 0, 0
	end := len(tokens)
	for index, token := range tokens {
		switch token.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		if depth < 0 || (depth == 0 && token.text == ";") {
			end = index
			break
		}
		if depth == 0 && token.text == "," {
			operands = append(operands, tokens[start:index])
			start = index + 1
		}
	}
	if start < end {
		operands = append(operands, tokens[start:end])
	}
	return operands
}

// isErrorValue reports whether an operand is an error variable, an error
// constructor call, or an error composite literal, with nothing after it
func isErrorValue(operand []lineToken) bool {
	for len(operand) > 0 && pointerOperators[operand[0].text] {
		operand = operand[1:]
	}
	if len(operand) == 0 || operand[0].kind != tokenIdentifier {
		return false
	}
	name := operand[0].value
	if len(operand) == 1 {
		return namesError(name)
	}
	opener := operand[1].text
	// the call or literal must be the whole operand, or continue on the next line
	closing := closingIndex(operand, 1)
	if (opener != "(" && opener != "{") || (closing != len(operand)-1 && closing != -1) {
		return false
	}
	if opener == "{" {
		return namesError(name)
	}
	return isErrorConstructor(name, len(operand) > 2 && operand[2].text == ")")
}

// closingIndex returns the index of the bracket closing the one at open, or -1
func closingIndex(tokens []lineToken, open int) int {
	depth := 0
	for index := open; index < len(tokens); index++ {
		switch tokens[index].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// errorPackages build errors with whatever function they call
var errorPackages = wordSet("errors", "xerrors", "pkgerrors")

// errorConstructors are function names (the last part of a qualified name) that
// return an error: fmt.Errorf, status.Error(code, msg), Rust Err(...), Swift .failure(...)
var errorConstructors = wordSet("Errorf", "Error", "NewError", "Err", "failure")

// isErrorConstructor reports whether calling name returns an error. err.Error()
// returns the message string, so a qualified Error with no arguments does not count.
func isErrorConstructor(name string, noArguments bool) bool {
	parts := strings.Split(name, ".")
	last := parts[len(parts)-1]
	if len(parts) > 1 && errorPackages[parts[0]] {
		return last != "Is" && last != "As"
	}
	if last == "Error" && len(parts) > 1 && noArguments {
		return false
	}
	return errorConstructors[last] || namesError(last)
}

// namesError reports whether an identifier, as written, names an error value:
// err, error, parseErr, ErrClosed, errNotFound, read_error, ValueError. It is
// case-sensitive so errorCount and errors do not count.
func namesError(identifier string) bool {
	name := identifier[strings.LastIndex(identifier, ".")+1:]
	if predicatePrefixes[firstWord(name)] && firstWord(name) != name {
		return false // isError, hasErr: a yes-or-no answer about an error
	}
	switch {
	case name == "err" || name == "error" || name == "Err" || name == "Error":
		return true
	case strings.HasSuffix(name, "Err") || strings.HasSuffix(name, "Error") ||
		strings.HasSuffix(name, "_err") || strings.HasSuffix(name, "_error"):
		return true
	case (strings.HasPrefix(name, "err") || strings.HasPrefix(name, "Err")) && len(name) > 3:
		next := name[3]
		return next >= 'A' && next <= 'Z' || next == '_' || next >= '0' && next <= '9'
	}
	return false
}

func firstSideEffect(line string) string {
	code := stripLiteralsAndComments(line)
	for _, pattern := range sideEffectPatterns {
		if strings.Contains(code, pattern) {
			return pattern
		}
	}
	return ""
}

// behaviourChangeFactor scores new control flow and side effects
func behaviourChangeFactor(edit functionEdit) riskFactor {
	change := countBehaviourChange(edit.unpairedAdded, edit.languageSyntax())
	units := change.units()
	factor := riskFactor{contribution: WeightBehaviourChange * ramp(float64(units), 0, BehaviourChangeFullUnits)}
	if units >= behaviourReasonMinUnits || len(change.sideEffects) > 0 {
		factor.reason = "behaviour change: adds " + change.describe()
	}
	return factor
}

// describe names what changed: "4 branches, an error return and an exec.Command"
func (change behaviourChange) describe() string {
	var parts []string
	parts = appendCount(parts, change.branches, "branch", "branches")
	parts = appendCount(parts, change.errorReturns, "error return", "error returns")
	parts = appendCount(parts, change.goroutines, "goroutine", "goroutines")
	parts = appendCount(parts, change.defers, "defer", "defers")
	parts = appendCount(parts, change.panics, "panic", "panics")
	for _, pattern := range change.sideEffects {
		parts = append(parts, withArticle(pattern))
	}
	return joinWithAnd(parts)
}

func appendCount(parts []string, count int, singular, plural string) []string {
	switch {
	case count == 1:
		return append(parts, withArticle(singular))
	case count > 1:
		return append(parts, fmt.Sprintf("%d %s", count, plural))
	}
	return parts
}

func withArticle(noun string) string {
	if noun != "" && strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// Decision-logic factor. The subtlest bugs found in review were in code that
// decides what is skipped, hidden, withheld, trusted, cached or allowed, driven
// by input the PR author controls (a file name, a marker comment, a long line).
// Each changed line that makes such a decision is a signal; the factor is full
// at decisionFullSignals.
const (
	WeightDecisionLogic = 8.0
	decisionFullSignals = 2
)

// decisionWords are identifier words that name a skip, trust or allow decision
var decisionWords = wordSet("skip", "skipped", "skips", "ignore", "ignored", "ignores", "withhold",
	"withheld", "withholds", "hide", "hidden", "hides", "allow", "allowed", "allows", "deny", "denied",
	"trust", "trusted", "safe", "generated", "exclude", "excluded", "excludes", "filter", "filtered",
	"whitelist", "allowlist", "blacklist", "blocklist", "denylist", "bypass", "minified", "vendored")

// predicatePrefixes start the name of a function that answers yes or no
var predicatePrefixes = wordSet("is", "should", "can", "has", "may", "must", "needs", "allow", "allows")

// matchCalls compare a path or content against a pattern
var matchCalls = wordSet("hassuffix", "hasprefix", "contains", "containsany", "match", "matchstring",
	"fullmatch", "search", "endswith", "startswith", "includes", "test", "fnmatch", "glob")

// matchSubjects are words naming the path or content a match is applied to
var matchSubjects = wordSet("path", "file", "filename", "name", "base", "ext", "extension", "dir",
	"directory", "content", "contents", "line", "text", "body", "url", "host", "pattern")

// decisionContext is what the factor knows about the function as a whole
type decisionContext struct {
	syntax         *languageSyntax
	nameDecides    bool // the function's name contains a decision word
	isPredicate    bool // ... and asks a yes-or-no question (isGenerated, shouldSkip)
	booleanReturns bool // the changed lines return true or false
}

// decisionLogicFactor fires when changed lines decide what is skipped, hidden,
// trusted or allowed: assignments to or returns of decision-named values,
// returns in decision-named functions, and pattern matches on paths or content
// that feed a yes-or-no answer
func decisionLogicFactor(function *FunctionRisk, edit functionEdit) riskFactor {
	context := decisionContext{syntax: edit.languageSyntax(), nameDecides: hasDecisionWord(function.Name)}
	context.isPredicate = context.nameDecides && predicatePrefixes[firstWord(function.Name)]
	tokenized := make([][]lineToken, len(edit.unpairedAdded))
	for index, line := range edit.unpairedAdded {
		tokenized[index] = tokenizeLine(line, context.syntax)
		context.booleanReturns = context.booleanReturns || returnsBoolean(tokenized[index])
	}
	signals, label := 0, ""
	for _, tokens := range tokenized {
		found := context.decisionConstruct(tokens)
		if found == "" {
			continue
		}
		signals++
		if label == "" {
			label = found
		}
	}
	if signals == 0 {
		return riskFactor{}
	}
	if context.nameDecides {
		label = function.Name
	}
	return riskFactor{
		contribution: WeightDecisionLogic * ramp(float64(signals), 0, decisionFullSignals),
		reason:       fmt.Sprintf("decides what is skipped or trusted (%s): check it can't be gamed by PR content", label),
	}
}

// decisionConstruct returns a label for the decision a line makes, or ""
func (context decisionContext) decisionConstruct(tokens []lineToken) string {
	returnIndex := slices.IndexFunc(tokens, func(token lineToken) bool {
		return token.kind == tokenKeyword && token.value == "return"
	})
	if name := decisionAssignment(tokens); name != "" {
		return name
	}
	if returnIndex >= 0 {
		returned := tokens[returnIndex+1:]
		for _, token := range returned {
			if token.kind == tokenIdentifier && hasDecisionWord(token.value) {
				return token.value
			}
		}
		if context.nameDecides && (context.isPredicate || isPlainVerdict(returned)) {
			return "return"
		}
	}
	return context.patternMatch(tokens, returnIndex >= 0)
}

// decisionAssignment returns the decision-named target of an assignment
// (skip := true, file.Generated = true, allowed[name] = struct{}{}), or ""
func decisionAssignment(tokens []lineToken) string {
	assignment := slices.IndexFunc(tokens, func(token lineToken) bool { return assignmentOperators[token.text] })
	if assignment < 0 {
		return ""
	}
	for _, token := range tokens[:assignment] {
		if token.kind == tokenIdentifier && hasDecisionWord(token.value) {
			return token.value
		}
	}
	return ""
}

// patternMatch returns the match call in a line that tests a path or content and
// feeds a yes-or-no answer: in a return, or in a condition of a function whose
// changed lines return true or false
func (context decisionContext) patternMatch(tokens []lineToken, isReturn bool) string {
	isCondition := slices.ContainsFunc(tokens, func(token lineToken) bool {
		return token.kind == tokenKeyword && context.syntax.branches[token.value]
	})
	feedsAnswer := isReturn || (isCondition && context.booleanReturns)
	if !feedsAnswer {
		return ""
	}
	call := ""
	for index, token := range tokens {
		if token.kind == tokenIdentifier && index+1 < len(tokens) && tokens[index+1].text == "(" &&
			matchCalls[token.text[strings.LastIndex(token.text, ".")+1:]] {
			call = token.value
			break
		}
	}
	if call == "" || !hasMatchSubject(tokens) {
		return ""
	}
	return call
}

// hasMatchSubject reports whether a line matches against a literal pattern or a
// path- or content-named value
func hasMatchSubject(tokens []lineToken) bool {
	for _, token := range tokens {
		if token.kind == tokenString {
			return true
		}
		if token.kind != tokenIdentifier {
			continue
		}
		for _, word := range splitIdentifierWords(token.value) {
			if matchSubjects[word] {
				return true
			}
		}
	}
	return false
}

// returnsBoolean reports whether a line returns a bare true or false
func returnsBoolean(tokens []lineToken) bool {
	for index, token := range tokens {
		if token.kind != tokenKeyword || token.value != "return" {
			continue
		}
		operands := splitOperands(tokens[index+1:])
		if len(operands) != 1 || len(operands[0]) != 1 {
			return false
		}
		answer := operands[0][0]
		return answer.kind == tokenKeyword && (answer.text == "true" || answer.text == "false")
	}
	return false
}

// isPlainVerdict reports whether returned tokens are a single constant answer:
// true, false, nil, None, null or "" (an empty reason meaning "keep it")
func isPlainVerdict(returned []lineToken) bool {
	operands := splitOperands(returned)
	if len(operands) != 1 || len(operands[0]) != 1 {
		return false
	}
	token := operands[0][0]
	switch token.kind {
	case tokenKeyword:
		return token.text == "true" || token.text == "false" || token.text == "nil" || token.text == "none" || token.text == "null"
	case tokenString:
		return token.value == ""
	}
	return false
}

func hasDecisionWord(identifier string) bool {
	for _, word := range splitIdentifierWords(identifier) {
		if decisionWords[word] {
			return true
		}
	}
	return false
}

func firstWord(identifier string) string {
	words := splitIdentifierWords(identifier)
	if len(words) == 0 {
		return ""
	}
	return words[0]
}
