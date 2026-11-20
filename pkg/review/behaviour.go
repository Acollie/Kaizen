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
	// WeightBehaviourChange is added on top of the other weights (scores are clamped to 100)
	WeightBehaviourChange    = 15.0
	BehaviourChangeFullUnits = 10.0
	// behaviourReasonMinUnits keeps a single new if out of the reasons
	behaviourReasonMinUnits = 2
)

// branchKeywords open a new path through the code (Go, Python, Kotlin, Swift)
var branchKeywords = map[string]bool{
	"if": true, "elif": true, "for": true, "while": true, "switch": true, "case": true,
	"select": true, "when": true, "guard": true, "match": true, "except": true, "catch": true,
}

// panicKeywords abort the normal flow
var panicKeywords = map[string]bool{"panic": true, "raise": true, "throw": true}

// behaviourChange counts what the new lines add
type behaviourChange struct {
	branches, errorReturns, goroutines, defers, panics int
	sideEffects                                        []string // distinct patterns, in first-seen order
}

func (change behaviourChange) units() int {
	return change.branches + change.errorReturns + change.defers + change.panics +
		2*change.goroutines + 2*len(change.sideEffects)
}

// countBehaviourChange inspects added lines that are not rewrites of a removed line
func countBehaviourChange(lines []string) behaviourChange {
	var change behaviourChange
	for _, line := range lines {
		change.addLine(line)
	}
	return change
}

func (change *behaviourChange) addLine(line string) {
	tokens := tokenizeLine(line)
	for index, token := range tokens {
		if token.kind != tokenKeyword {
			continue
		}
		switch {
		case branchKeywords[token.text]:
			change.branches++
		case token.text == "go":
			change.goroutines++
		case token.text == "defer":
			change.defers++
		case panicKeywords[token.text]:
			change.panics++
		case token.text == "return" && returnsError(tokens[index+1:]):
			change.errorReturns++
		}
	}
	if pattern := firstSideEffect(line); pattern != "" && !slices.Contains(change.sideEffects, pattern) {
		change.sideEffects = append(change.sideEffects, pattern)
	}
}

// returnsError reports whether a return statement's operands mention an error value
func returnsError(operands []lineToken) bool {
	for _, token := range operands {
		if token.kind == tokenIdentifier && namesError(token.text) {
			return true
		}
	}
	return false
}

// namesError reports whether a (possibly qualified, lower-cased) identifier
// looks like an error value or constructor: err, parseErr, ErrClosed, fmt.Errorf
func namesError(identifier string) bool {
	for _, part := range strings.Split(identifier, ".") {
		if strings.HasPrefix(part, "err") || strings.HasSuffix(part, "err") || strings.HasSuffix(part, "error") {
			return true
		}
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
	change := countBehaviourChange(edit.unpairedAdded)
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
