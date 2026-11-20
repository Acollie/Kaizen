package review

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Risk weights. Each factor is scaled to 0-1 against absolute thresholds (not
// relative to the other functions in the PR) and multiplied by its weight; the
// weights sum to 100, so a function maxing every factor scores 100 and a
// trivial one-line change to a simple function scores close to 0.
const (
	// WeightCyclomatic rewards branchy code: 0 at CC<=5, full at CC>=20
	WeightCyclomatic = 20.0
	// WeightCognitive rewards hard-to-read code: 0 at <=8, full at >=30
	WeightCognitive = 10.0
	// WeightNesting rewards deep nesting: 0 at depth<=2, full at >=6
	WeightNesting = 8.0
	// WeightFanIn rewards blast radius: 0 at fan-in<=1, full at the critical threshold (15)
	WeightFanIn = 20.0
	// WeightChurn rewards unstable code: 0 at <=2 commits, full at >=20
	WeightChurn = 10.0
	// WeightLinesChanged rewards large edits: 0 at <=5 lines, full at >=80
	WeightLinesChanged = 10.0
	// WeightLowMaintainability rewards poor MI: 0 at MI>=40, full at MI<=10
	WeightLowMaintainability = 7.0
	// WeightNewComplex rewards new functions that are already complex: 0 at CC<10, full at CC>=20
	WeightNewComplex = 5.0
	// WeightSecurity is added in full when the name, path or changed lines look security-sensitive
	WeightSecurity = 10.0
)

// Thresholds at which a factor is called out in RiskReasons
const (
	ReasonCyclomaticNotable  = 10
	ReasonCyclomaticHigh     = 20
	ReasonCognitive          = 15
	ReasonNesting            = 4
	ReasonFanInWarning       = 5  // matches check.ThresholdBlastRadiusWarning
	ReasonFanInCritical      = 15 // matches check.ThresholdBlastRadiusCritical
	ReasonChurn              = 5
	ReasonLinesChanged       = 30
	ReasonLowMaintainability = 20
)

// DefaultFocusCount is how many functions get Focus when the caller passes 0
const DefaultFocusCount = 5

// MinFocusScore is the minimum RiskScore for a function to be a focus function
const MinFocusScore = 15.0

// GuaranteedFocusCount top-ranked functions are focused even below MinFocusScore,
// as long as they reach GuaranteedFocusMinScore, so a PR of moderate changes still
// gives the review a few concrete starting points instead of one or none.
const (
	GuaranteedFocusCount    = 3
	GuaranteedFocusMinScore = 5.0
)

// riskFactor is one weighted contribution to a function's score
type riskFactor struct {
	contribution float64
	reason       string // empty when the factor is too small to mention
}

// ScoreFunctions sets RiskScore, RiskReasons and Focus on each function in
// place, using only the function's own name and path for the security heuristic.
// The slice order is not changed.
func ScoreFunctions(functions []FunctionRisk, focusCount int) {
	ScoreFunctionsWithDiff(functions, nil, focusCount)
}

// ScoreFunctionsWithDiff is ScoreFunctions with access to the display diff, so the
// security heuristic can also look at the lines the PR added inside each function.
func ScoreFunctionsWithDiff(functions []FunctionRisk, files []FileDiff, focusCount int) {
	addedLines := addedLinesByFile(files)
	for index := range functions {
		function := &functions[index]
		factors := collectRiskFactors(function, addedLinesInRange(addedLines[function.File], function.StartLine, function.EndLine))
		function.RiskScore, function.RiskReasons = combineFactors(factors)
		if IsTestFile(function.File) {
			function.RiskScore = math.Round(function.RiskScore*TestCodeMultiplier*10) / 10
			function.RiskReasons = append(function.RiskReasons, "test code: weighted down so production changes get reviewed first")
		}
	}
	assignFocus(functions, focusCount)
}

// TestCodeMultiplier scales the score of functions in test files. Tests still show up
// in the ranking, but a large new test should not outrank the production code it tests.
const TestCodeMultiplier = 0.35

// IsTestFile reports whether a repo-relative path follows a test-file naming convention
// for one of the supported languages, or lives under a test directory.
func IsTestFile(path string) bool {
	slashed := filepath.ToSlash(path)
	base := filepath.Base(slashed)
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return true
	case strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"):
		return true
	case strings.HasSuffix(base, "_test.py"):
		return true
	case strings.HasSuffix(base, "Test.kt"), strings.HasSuffix(base, "Tests.kt"):
		return true
	case strings.HasSuffix(base, "Tests.swift"), strings.HasSuffix(base, "Test.swift"):
		return true
	}
	for _, segment := range strings.Split(slashed, "/") {
		if segment == "test" || segment == "tests" || segment == "__tests__" {
			return true
		}
	}
	return false
}

// collectRiskFactors evaluates every factor for one function
func collectRiskFactors(function *FunctionRisk, changedLines []string) []riskFactor {
	factors := []riskFactor{
		fanInFactor(function),
		churnFactor(function),
		linesChangedFactor(function),
		securityFactor(function, changedLines),
	}
	if function.Metrics != nil {
		factors = append(factors,
			cyclomaticFactor(function.Metrics.CyclomaticComplexity),
			cognitiveFactor(function.Metrics.CognitiveComplexity),
			nestingFactor(function.Metrics.NestingDepth),
			maintainabilityFactor(function.Metrics.MaintainabilityIndex, function.Metrics.HalsteadVolume),
			newComplexFactor(function.IsNew, function.Metrics.CyclomaticComplexity),
		)
	}
	return factors
}

// combineFactors sums contributions into a 0-100 score and orders reasons by contribution
func combineFactors(factors []riskFactor) (float64, []string) {
	sort.SliceStable(factors, func(left, right int) bool {
		return factors[left].contribution > factors[right].contribution
	})

	total := 0.0
	reasons := []string{}
	for _, factor := range factors {
		total += factor.contribution
		if factor.reason != "" {
			reasons = append(reasons, factor.reason)
		}
	}

	score := math.Round(math.Min(100, math.Max(0, total))*10) / 10
	return score, reasons
}

// assignFocus marks the highest-scoring functions (ties broken by ID) as Focus
func assignFocus(functions []FunctionRisk, focusCount int) {
	if focusCount <= 0 {
		focusCount = DefaultFocusCount
	}

	order := make([]int, len(functions))
	for index := range functions {
		order[index] = index
		functions[index].Focus = false
	}
	sort.SliceStable(order, func(left, right int) bool {
		leftFunction, rightFunction := functions[order[left]], functions[order[right]]
		if leftFunction.RiskScore != rightFunction.RiskScore {
			return leftFunction.RiskScore > rightFunction.RiskScore
		}
		return leftFunction.ID < rightFunction.ID
	})

	for rank, index := range order {
		if rank >= focusCount {
			break
		}
		score := functions[index].RiskScore
		guaranteed := rank < GuaranteedFocusCount && score >= GuaranteedFocusMinScore
		if score < MinFocusScore && !guaranteed {
			break
		}
		functions[index].Focus = true
	}
}

// ramp scales value linearly from 0 at low to 1 at high, clamped
func ramp(value, low, high float64) float64 {
	if value <= low {
		return 0
	}
	if value >= high {
		return 1
	}
	return (value - low) / (high - low)
}

func cyclomaticFactor(complexity int) riskFactor {
	factor := riskFactor{contribution: WeightCyclomatic * ramp(float64(complexity), 5, 20)}
	switch {
	case complexity >= ReasonCyclomaticHigh:
		factor.reason = fmt.Sprintf("cyclomatic complexity %d (high)", complexity)
	case complexity >= ReasonCyclomaticNotable:
		factor.reason = fmt.Sprintf("cyclomatic complexity %d (notable)", complexity)
	}
	return factor
}

func cognitiveFactor(complexity int) riskFactor {
	factor := riskFactor{contribution: WeightCognitive * ramp(float64(complexity), 8, 30)}
	if complexity >= ReasonCognitive {
		factor.reason = fmt.Sprintf("cognitive complexity %d: hard to follow", complexity)
	}
	return factor
}

func nestingFactor(depth int) riskFactor {
	factor := riskFactor{contribution: WeightNesting * ramp(float64(depth), 2, 6)}
	if depth >= ReasonNesting {
		factor.reason = fmt.Sprintf("nesting depth %d: deeply nested logic", depth)
	}
	return factor
}

func fanInFactor(function *FunctionRisk) riskFactor {
	factor := riskFactor{contribution: WeightFanIn * ramp(float64(function.FanIn), 1, ReasonFanInCritical)}
	if function.FanIn < ReasonFanInWarning {
		return factor
	}
	approximate := ""
	if function.FanInApproximate {
		approximate = " (approximate)"
	}
	if function.FanIn >= ReasonFanInCritical {
		factor.reason = fmt.Sprintf("fan-in %d%s: critical blast radius, many callers depend on this", function.FanIn, approximate)
	} else {
		factor.reason = fmt.Sprintf("fan-in %d%s: changes ripple to many callers", function.FanIn, approximate)
	}
	return factor
}

func churnFactor(function *FunctionRisk) riskFactor {
	if function.Metrics == nil || function.Metrics.Churn == nil {
		return riskFactor{}
	}
	commits := function.Metrics.Churn.TotalCommits
	factor := riskFactor{contribution: WeightChurn * ramp(float64(commits), 2, 20)}
	if commits >= ReasonChurn {
		factor.reason = fmt.Sprintf("churn %d commits: frequently changed code", commits)
	}
	return factor
}

func linesChangedFactor(function *FunctionRisk) riskFactor {
	changed := function.LinesAdded + function.LinesRemoved
	factor := riskFactor{contribution: WeightLinesChanged * ramp(float64(changed), 5, 80)}
	if changed >= ReasonLinesChanged {
		factor.reason = fmt.Sprintf("%d lines changed (+%d/-%d): large edit", changed, function.LinesAdded, function.LinesRemoved)
	}
	return factor
}

// maintainabilityFactor ignores MI when the analyzer did not compute it (MI and volume both 0)
func maintainabilityFactor(maintainability, halsteadVolume float64) riskFactor {
	if maintainability <= 0 && halsteadVolume <= 0 {
		return riskFactor{}
	}
	factor := riskFactor{contribution: WeightLowMaintainability * ramp(40-maintainability, 0, 30)}
	if maintainability < ReasonLowMaintainability {
		factor.reason = fmt.Sprintf("maintainability index %.0f (low)", maintainability)
	}
	return factor
}

func newComplexFactor(isNew bool, complexity int) riskFactor {
	if !isNew || complexity < ReasonCyclomaticNotable {
		return riskFactor{}
	}
	return riskFactor{
		contribution: WeightNewComplex * ramp(float64(complexity), ReasonCyclomaticNotable-1, ReasonCyclomaticHigh),
		reason:       fmt.Sprintf("new function with cyclomatic complexity %d: no history, needs tests", complexity),
	}
}

// securityNameTokens are identifier/path words that suggest security-sensitive code
var securityNameTokens = map[string]bool{
	"auth": true, "authn": true, "authz": true, "authenticate": true, "authentication": true,
	"authorize": true, "authorization": true, "oauth": true, "login": true, "session": true,
	"token": true, "jwt": true, "password": true, "passwd": true, "secret": true, "credential": true,
	"credentials": true, "crypto": true, "encrypt": true, "decrypt": true, "cipher": true,
	"sql": true, "query": true, "exec": true, "command": true, "shell": true, "handler": true,
	"deserialize": true, "unmarshal": true, "template": true, "sanitize": true, "permission": true,
}

// securityLinePatterns are code fragments in added lines that suggest security-sensitive code
var securityLinePatterns = []string{
	"exec.Command", "os/exec", "subprocess", "os.system", "Runtime.getRuntime", "ProcessBuilder",
	"database/sql", ".Query(", ".Exec(", "SELECT ", "INSERT ", "UPDATE ", "DELETE FROM",
	"crypto/", "hashlib", "password", "secret", "token", "Authorization",
	"http.ResponseWriter", "http.Request", "HandleFunc", "ServeHTTP",
	"Unmarshal", "pickle.load", "yaml.load", "gob.NewDecoder", "Decodable",
	"template.HTML", "text/template", "html/template",
	"filepath.Join", "os.path.join", "path.Join",
}

func securityFactor(function *FunctionRisk, changedLines []string) riskFactor {
	if token := firstSecurityToken(function.Name, function.ReceiverType, function.File); token != "" {
		return riskFactor{
			contribution: WeightSecurity,
			reason:       fmt.Sprintf("security-sensitive: name or path mentions %q", token),
		}
	}
	for _, line := range changedLines {
		for _, pattern := range securityLinePatterns {
			if strings.Contains(line, pattern) {
				return riskFactor{
					contribution: WeightSecurity,
					reason:       fmt.Sprintf("security-sensitive: changed lines use %q", strings.TrimSpace(pattern)),
				}
			}
		}
	}
	return riskFactor{}
}

// firstSecurityToken splits identifiers and paths into words and returns the first sensitive one
func firstSecurityToken(texts ...string) string {
	for _, text := range texts {
		for _, word := range splitIdentifierWords(text) {
			if securityNameTokens[word] {
				return word
			}
		}
	}
	return ""
}

// splitIdentifierWords splits camelCase, snake_case and path text into lower-case words
func splitIdentifierWords(text string) []string {
	var words []string
	var current strings.Builder
	runes := []rune(text)

	flush := func() {
		if current.Len() > 0 {
			words = append(words, strings.ToLower(current.String()))
			current.Reset()
		}
	}

	for position, character := range runes {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			flush()
			continue
		}
		if unicode.IsUpper(character) && position > 0 && startsNewWord(runes, position) {
			flush()
		}
		current.WriteRune(character)
	}
	flush()
	return words
}

// startsNewWord reports whether an upper-case rune begins a new camelCase word
// ("parseJSON" splits before J, "JSONParser" splits before P)
func startsNewWord(runes []rune, position int) bool {
	previous := runes[position-1]
	if unicode.IsLower(previous) || unicode.IsDigit(previous) {
		return true
	}
	hasNext := position+1 < len(runes)
	return unicode.IsUpper(previous) && hasNext && unicode.IsLower(runes[position+1])
}

// addedLinesByFile indexes added head-side lines by file path and line number
func addedLinesByFile(files []FileDiff) map[string]map[int]string {
	index := make(map[string]map[int]string, len(files))
	for _, file := range files {
		lines := make(map[int]string)
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == LineAdded {
					lines[line.NewLine] = line.Text
				}
			}
		}
		index[file.Path] = lines
	}
	return index
}

// addedLinesInRange returns the added lines whose head-side number is within [start, end]
func addedLinesInRange(lines map[int]string, start, end int) []string {
	var result []string
	for lineNumber := start; lineNumber <= end; lineNumber++ {
		if text, exists := lines[lineNumber]; exists {
			result = append(result, text)
		}
	}
	return result
}
