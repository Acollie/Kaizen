package review

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// Risk weights. Each factor is scaled to 0-1 against absolute thresholds (not
// relative to the other functions in the PR) and multiplied by its weight; the
// weights, together with WeightBehaviourChange and WeightDecisionLogic
// (behaviour.go), sum to 100, so a function maxing every factor scores 100 and a
// trivial one-line change to a simple function scores close to 0. The shape
// weights (complexity, nesting, size, MI) were cut when the diff-based factors
// were added, since those say more about what a change does.
const (
	// WeightCyclomatic rewards branchy code: 0 at CC<=5, full at CC>=20
	WeightCyclomatic = 14.0
	// WeightCognitive rewards hard-to-read code: 0 at <=8, full at >=30
	WeightCognitive = 7.0
	// WeightNesting rewards deep nesting: 0 at depth<=2, full at >=6
	WeightNesting = 5.0
	// WeightFanIn rewards blast radius: 0 at fan-in<=1, full at the critical threshold (15)
	WeightFanIn = 17.0
	// WeightChurn rewards unstable code: 0 at <=2 commits, full at >=20
	WeightChurn = 9.0
	// WeightLinesChanged rewards large edits: 0 at <=5 lines, full at >=80
	WeightLinesChanged = 7.0
	// WeightLowMaintainability rewards poor MI: 0 at MI>=40, full at MI<=10
	WeightLowMaintainability = 5.0
	// WeightNewComplex rewards new functions that are already complex: 0 at CC<10, full at CC>=20
	WeightNewComplex = 3.0
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
	filesByPath := make(map[string]*FileDiff, len(files))
	for index := range files {
		filesByPath[files[index].Path] = &files[index]
	}
	for index := range functions {
		function := &functions[index]
		edit := analyzeFunctionEdit(filesByPath[function.File], function.StartLine, function.EndLine)
		factors := collectRiskFactors(function, edit)
		function.RiskScore, function.RiskReasons = combineFactors(factors)
		if IsTestFile(function.File) {
			function.RiskScore = math.Round(function.RiskScore*TestCodeMultiplier*10) / 10
			function.RiskReasons = append(function.RiskReasons, "test code or fixture: weighted down so production changes get reviewed first")
		}
	}
	assignFocus(functions, focusCount)
}

// TestCodeMultiplier scales the score of functions in test files. Tests still show up
// in the ranking, but a large new test should not outrank the production code it tests.
const TestCodeMultiplier = 0.35

// testDirectorySegments are directory names that hold tests or test inputs (fixtures,
// golden files, snapshots) rather than production code
var testDirectorySegments = map[string]bool{
	"test": true, "tests": true, "__tests__": true,
	"testdata": true, "fixtures": true, "__fixtures__": true, "__snapshots__": true,
}

// IsTestFile reports whether a repo-relative path follows a test-file naming convention
// for one of the supported languages, or lives under a test or fixture directory.
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
		if testDirectorySegments[segment] {
			return true
		}
	}
	return false
}

// collectRiskFactors evaluates every factor for one function
func collectRiskFactors(function *FunctionRisk, edit functionEdit) []riskFactor {
	factors := []riskFactor{
		fanInFactor(function),
		churnFactor(function),
		securityFactor(function, edit.added),
		behaviourChangeFactor(edit),
		decisionLogicFactor(function, edit),
	}
	shape := []riskFactor{linesChangedFactor(function)}
	if function.Metrics != nil {
		shape = append(shape, intrinsicFactors(function)...)
		factors = append(factors, newComplexFactor(function.IsNew, function.Metrics.CyclomaticComplexity))
	}
	if edit.isMechanical() {
		shape = discountMechanicalEdit(shape, edit)
	}
	return append(factors, shape...)
}

// discountMechanicalEdit scales the size and shape factors of an edit that only
// rewrites existing statements, so existing complexity does not rank it high
func discountMechanicalEdit(factors []riskFactor, edit functionEdit) []riskFactor {
	for index := range factors {
		factors[index].contribution *= MechanicalEditDiscount
	}
	return append(factors, riskFactor{reason: edit.mechanicalReason()})
}

// intrinsicFactors scores the function's own shape (complexity, nesting, MI), scaled
// by how much of it the PR changed: a 3-line touch to a complex function is mostly
// a review of those 3 lines, while a rewrite of it is a review of all its complexity.
func intrinsicFactors(function *FunctionRisk) []riskFactor {
	metrics := function.Metrics
	factors := []riskFactor{
		cyclomaticFactor(metrics.CyclomaticComplexity),
		cognitiveFactor(metrics.CognitiveComplexity),
		nestingFactor(metrics.NestingDepth),
		maintainabilityFactor(metrics.MaintainabilityIndex, metrics.HalsteadVolume),
	}

	share, changed, length := changeShare(function)
	if share >= 1 {
		return factors
	}
	hasIntrinsicRisk := false
	for index := range factors {
		hasIntrinsicRisk = hasIntrinsicRisk || factors[index].contribution > 0
		factors[index].contribution *= share
	}
	if hasIntrinsicRisk {
		factors = append(factors, riskFactor{
			reason: fmt.Sprintf("small edit (%d of %d lines): complexity weighted down", changed, length),
		})
	}
	return factors
}

// Change-share scaling: the share of a function the PR changed is
// changed lines / length * ChangeShareGain, clamped to [MinChangeShare, 1], so
// editing half the function (or more) counts its complexity in full
const (
	ChangeShareGain = 2.0
	MinChangeShare  = 0.25
)

// changeShare returns the scale applied to intrinsic factors, with the changed
// line count and function length it was computed from. New functions, and
// functions whose change size or length is unknown, are not scaled.
func changeShare(function *FunctionRisk) (share float64, changed, length int) {
	changed = function.LinesAdded + function.LinesRemoved
	length = function.Metrics.Length
	if length <= 0 && function.EndLine >= function.StartLine && function.StartLine > 0 {
		length = function.EndLine - function.StartLine + 1
	}
	if function.IsNew || changed <= 0 || length <= 0 {
		return 1, changed, length
	}
	share = float64(changed) / float64(length) * ChangeShareGain
	return math.Max(MinChangeShare, math.Min(1, share)), changed, length
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
	subject := fanInSubject(function)
	if function.FanIn >= ReasonFanInCritical {
		factor.reason = subject + ": critical blast radius, many callers depend on this"
	} else {
		factor.reason = subject + ": changes ripple to many callers"
	}
	return factor
}

// fanInSubject describes fan-in as call sites and, when known, distinct callers:
// "fan-in 36 call sites from 11 callers (approximate)"
func fanInSubject(function *FunctionRisk) string {
	subject := fmt.Sprintf("fan-in %d", function.FanIn)
	if function.CallerCount > 0 {
		subject += fmt.Sprintf(" call sites from %d %s", function.CallerCount, pluralize(function.CallerCount, "caller", "callers"))
	}
	if function.FanInApproximate {
		subject += " (approximate)"
	}
	return subject
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
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

// sideEffectPatterns are code fragments that give a function a new effect on the
// outside world: running processes, network and file writes, and raw HTML
var sideEffectPatterns = []string{
	// process execution
	"exec.CommandContext", "exec.Command", "syscall.Exec", "subprocess", "os.system",
	"Runtime.getRuntime", "ProcessBuilder",
	// outbound effects and listeners
	"http.Post", "http.NewRequest", "net.Listen", "ListenAndServe",
	"os.WriteFile", "os.RemoveAll", "os.Setenv",
	// HTML injection
	"template.HTML", "dangerouslySetInnerHTML", "innerHTML",
}

// securityCodePatterns are code fragments that suggest security-sensitive code. They
// are matched against changed lines with comments and string literals removed, so a
// word like "secret" in a help text or a comment does not count.
var securityCodePatterns = append(slices.Clone(sideEffectPatterns),
	// data access, secrets and request handling
	".Query(", ".Exec(", "hashlib", "Authorization",
	"http.ResponseWriter", "http.Request", "HandleFunc", "ServeHTTP",
	"Unmarshal", "pickle.load", "yaml.load", "gob.NewDecoder", "Decodable",
)

// securityLiteralPatterns only ever appear inside string literals (import paths, SQL,
// command-line arguments), so they are matched against the raw changed line
var securityLiteralPatterns = []string{
	"os/exec", "database/sql", "crypto/", "text/template", "html/template",
	"SELECT ", "INSERT ", "UPDATE ", "DELETE FROM",
	"gh api", `"gh", "api"`, "--method POST", `"--method", "POST"`,
	"--permission-mode", "--dangerously-skip-permissions", "--allowedTools",
}

// securityCodeExpressions are patterns that need word boundaries or context, named
// by the label used in the risk reason
var securityCodeExpressions = []struct {
	label      string
	expression *regexp.Regexp
}{
	{"eval(", regexp.MustCompile(`\beval\(`)},
	{".Do( on an HTTP client", regexp.MustCompile(`(?i)client\)?\.Do\(`)},
}

func securityFactor(function *FunctionRisk, changedLines []string) riskFactor {
	if token := firstSecurityToken(function.Name, function.ReceiverType, function.File); token != "" {
		return riskFactor{
			contribution: WeightSecurity,
			reason:       fmt.Sprintf("security-sensitive: name or path mentions %q", token),
		}
	}
	for _, line := range changedLines {
		if pattern := firstSecurityPattern(line); pattern != "" {
			return riskFactor{
				contribution: WeightSecurity,
				reason:       fmt.Sprintf("security-sensitive: changed lines use %q", strings.TrimSpace(pattern)),
			}
		}
	}
	return riskFactor{}
}

// firstSecurityPattern returns the first security pattern a changed line uses, or ""
func firstSecurityPattern(line string) string {
	code := stripLiteralsAndComments(line)
	if word := firstCredentialIdentifier(code); word != "" {
		return word
	}
	for _, pattern := range securityCodePatterns {
		if strings.Contains(code, pattern) {
			return pattern
		}
	}
	for _, candidate := range securityCodeExpressions {
		if candidate.expression.MatchString(code) {
			return candidate.label
		}
	}
	for _, pattern := range securityLiteralPatterns {
		if strings.Contains(line, pattern) {
			return pattern
		}
	}
	return ""
}

// stripLiteralsAndComments blanks the contents of "...", '...' and `...` literals
// and drops // and /* */ comments from a single line. It is a per-line heuristic:
// literals and comments spanning lines are not tracked. The diff-based factors use
// commentScanner directly, with the file's language and state across lines.
func stripLiteralsAndComments(line string) string {
	return newCommentScanner(defaultSyntax, true).clean(line)
}

// sensitivePatterns returns every security and side-effect pattern a line of
// code uses, sorted, so two lines can be compared for them
func sensitivePatterns(line string) []string {
	code := stripLiteralsAndComments(line)
	var found []string
	for _, pattern := range securityCodePatterns {
		if strings.Contains(code, pattern) && !slices.Contains(found, pattern) {
			found = append(found, pattern)
		}
	}
	for _, candidate := range securityCodeExpressions {
		if candidate.expression.MatchString(code) {
			found = append(found, candidate.label)
		}
	}
	for _, pattern := range securityLiteralPatterns {
		if strings.Contains(line, pattern) {
			found = append(found, pattern)
		}
	}
	slices.Sort(found)
	return found
}

// skipLiteral returns the index of the quote closing the literal opened at start,
// honouring backslash escapes except in raw (backtick) strings
func skipLiteral(line string, start int) int {
	quote := line[start]
	for position := start + 1; position < len(line); position++ {
		if line[position] == '\\' && quote != '`' {
			position++
			continue
		}
		if line[position] == quote {
			return position
		}
	}
	return len(line)
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

// File-level risk for changes that no function captures. Scores use the same 0-100 scale
// as functions so files and functions can be ranked together.
const (
	// FileRiskWorkflow applies to CI definitions, which run with repository secrets and write tokens
	FileRiskWorkflow = 35.0
	// FileRiskScript applies to shell scripts, Makefiles and Dockerfiles, which execute commands
	FileRiskScript = 22.0
	// FileRiskDependencies applies to dependency manifests and lockfiles (supply chain)
	FileRiskDependencies = 18.0
	// FileRiskGitattributes applies to .gitattributes, which can hide changes from diffs
	FileRiskGitattributes = 30.0
	// FileRiskAgentInstructions applies to CLAUDE.md, AGENTS.md, .claude/ and similar
	// files, which AI tools (including the AI half of this review) follow as instructions
	FileRiskAgentInstructions = 30.0
	// WeightUnstructuredSize scales the size factor for files without analyzable functions:
	// 0 at <=10 changed lines, full at >=200
	WeightUnstructuredSize = 20.0
)

// ScoreFile returns the file's own risk and the reasons for it. hasFunctions reports
// whether any changed function was found in the file; when none was (YAML, shell,
// unsupported languages), the size of the change counts too, since nothing else will.
func ScoreFile(file FileDiff, hasFunctions bool) (float64, []string) {
	score, reasons := fileKindRisk(file.Path)
	if !hasFunctions && file.Status != FileDeleted {
		changed := file.Additions + file.Deletions
		sizeScore := WeightUnstructuredSize * ramp(float64(changed), 10, 200)
		if sizeScore > 0 {
			score += sizeScore
			if changed >= 50 {
				reasons = append(reasons, fmt.Sprintf("%d lines changed with no function-level analysis", changed))
			}
		}
	}
	return math.Round(math.Min(100, score)*10) / 10, reasons
}

// fileKindRisk scores a file by what kind of file it is
func fileKindRisk(path string) (float64, []string) {
	slashed := filepath.ToSlash(path)
	base := strings.ToLower(filepath.Base(slashed))
	extension := filepath.Ext(base)
	switch {
	case strings.Contains(slashed, ".github/workflows/") && (extension == ".yml" || extension == ".yaml"):
		return FileRiskWorkflow, []string{"CI workflow: runs with repository secrets and write tokens"}
	case base == "action.yml" || base == "action.yaml":
		return FileRiskWorkflow, []string{"GitHub Action definition: runs in other repositories' CI with their secrets"}
	case extension == ".sh" || extension == ".bash" || base == "makefile" || strings.HasPrefix(base, "dockerfile"):
		return FileRiskScript, []string{"script: executes shell commands"}
	case dependencyManifests[base]:
		return FileRiskDependencies, []string{"dependency manifest: supply-chain change"}
	case base == ".gitattributes":
		return FileRiskGitattributes, []string{"changes how git diffs and merges files: check for -diff, binary or merge attributes that hide changes"}
	case isAgentInstructionFile(slashed, base):
		return FileRiskAgentInstructions, []string{"AI agent instructions: can steer AI tools (including this review); read it yourself"}
	}
	return 0, nil
}

// agentInstructionFiles are lower-case file names AI coding tools load as instructions
var agentInstructionFiles = map[string]bool{
	"claude.md": true, "claude.local.md": true, "agents.md": true, ".cursorrules": true,
	"copilot-instructions.md": true, "gemini.md": true, ".windsurfrules": true,
}

// isAgentInstructionFile reports whether a file is read by AI tools as instructions:
// a known instruction file name, or anything under a .claude or .cursor directory
func isAgentInstructionFile(slashed, base string) bool {
	if agentInstructionFiles[base] {
		return true
	}
	for _, segment := range strings.Split(slashed, "/") {
		if segment == ".claude" || segment == ".cursor" {
			return true
		}
	}
	return false
}

// dependencyManifests are file names whose changes alter third-party code pulled into the build
var dependencyManifests = map[string]bool{
	"go.mod": true, "go.sum": true, "package.json": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "requirements.txt": true, "pyproject.toml": true, "poetry.lock": true,
	"cargo.toml": true, "cargo.lock": true, "gemfile": true, "gemfile.lock": true, "build.gradle": true,
	"build.gradle.kts": true, "package.swift": true, "podfile": true, "podfile.lock": true,
}

// credentialWords mark an identifier as holding a credential when they appear as a
// whole word in it (passwordHash, client_secret)
var credentialWords = map[string]bool{"password": true, "passwd": true, "secret": true, "credential": true, "credentials": true}

// authQualifiers turn a "token" identifier into a credential (accessToken, GH_TOKEN,
// bearer_token); without one, "token" is usually a lexer or parser token (lineToken)
var authQualifiers = map[string]bool{
	"access": true, "auth": true, "api": true, "bearer": true, "refresh": true, "session": true,
	"csrf": true, "jwt": true, "oauth": true, "id": true, "github": true, "gh": true, "personal": true, "secret": true,
}

// firstCredentialIdentifier returns the first identifier in code that names a
// credential, matched by whole words rather than substrings
func firstCredentialIdentifier(code string) string {
	for _, identifier := range identifierPattern.FindAllString(code, -1) {
		words := splitIdentifierWords(identifier)
		hasToken, hasQualifier := false, false
		for _, word := range words {
			if credentialWords[word] {
				return identifier
			}
			hasToken = hasToken || word == "token" || word == "tokens"
			hasQualifier = hasQualifier || authQualifiers[word]
		}
		if hasToken && hasQualifier {
			return identifier
		}
	}
	return ""
}

var identifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
