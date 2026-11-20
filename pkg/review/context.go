package review

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/alexcollie/kaizen/pkg/check"
	"github.com/alexcollie/kaizen/pkg/churn"
	"github.com/alexcollie/kaizen/pkg/languages"
	"github.com/alexcollie/kaizen/pkg/languages/golang"
	"github.com/alexcollie/kaizen/pkg/models"
)

const (
	// maxRelatedFunctions caps the callers and callees listed per changed function
	maxRelatedFunctions = 25
	// maxGraphNodes caps the review graph so the UI stays readable
	maxGraphNodes = 150
	// defaultChurnWindow is how far back churn looks when BuildOptions.ChurnSince is zero
	defaultChurnWindow = 365 * 24 * time.Hour
)

// BuildOptions tunes BuildContext
type BuildOptions struct {
	FocusCount int       // how many functions get Focus; 0 means DefaultFocusCount
	SkipChurn  bool      // skip git history lookups (faster, no churn factor)
	ChurnSince time.Time // churn window start; zero means one year ago
}

// changedEntry pairs a function from the mapping diff with its review record
type changedEntry struct {
	source  check.ChangedFunction // FilePath is absolute under the worktree
	risk    FunctionRisk
	absPath string
}

// BuildContext assembles everything Kaizen knows about a PR before the AI runs.
// worktree is the PR head checkout. displayDiff is `git diff -M --unified=3`
// between the merge base and head; mappingDiff is the same diff with
// --unified=0, used to locate changed functions.
func BuildContext(ctx context.Context, pr PullRequest, worktree string, displayDiff string, mappingDiff string, opts BuildOptions) (*ReviewContext, error) {
	root, err := resolveWorktree(worktree)
	if err != nil {
		return nil, err
	}

	files, err := ParseUnifiedDiff(displayDiff)
	if err != nil {
		return nil, fmt.Errorf("parse display diff: %w", err)
	}
	quarantined := quarantineUnsafeFiles(files, root)
	for path := range markGeneratedFiles(files) {
		quarantined[path] = true
	}
	setLineCounts(files, root)

	entries, err := findChangedFunctions(mappingDiff, files, root, quarantined)
	if err != nil {
		return nil, err
	}
	attachMetrics(entries, files)

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	callGraph := buildGoCallGraph(entries, root)
	fanInResults := attachRelationships(entries, callGraph, root)

	if !opts.SkipChurn {
		if err := attachChurn(ctx, entries, root, opts.ChurnSince); err != nil {
			return nil, err
		}
	}

	functions := make([]FunctionRisk, len(entries))
	for index := range entries {
		functions[index] = entries[index].risk
	}
	ScoreFunctionsWithDiff(functions, files, opts.FocusCount)
	sortFunctionsByRisk(functions)
	applyFileRisk(files, functions)

	concerns := check.DetectBlastRadius(fanInResults)
	if concerns == nil {
		concerns = []models.Concern{}
	}

	pr.FileCount = len(files)
	return &ReviewContext{
		PR:           pr,
		Files:        files,
		Functions:    functions,
		Graph:        buildReviewGraph(functions, callGraph, root),
		Concerns:     concerns,
		WorktreePath: root,
		GeneratedAt:  time.Now(),
	}, nil
}

// resolveWorktree returns the absolute, symlink-free worktree path so paths
// from git, the call graph and the filesystem all share one prefix
func resolveWorktree(worktree string) (string, error) {
	absolute, err := filepath.Abs(worktree)
	if err != nil {
		return "", fmt.Errorf("resolve worktree %s: %w", worktree, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("worktree %s is not accessible: %w", worktree, err)
	}
	return resolved, nil
}

// setLineCounts records the head-side line count of every non-deleted file
func setLineCounts(files []FileDiff, root string) {
	for index := range files {
		if files[index].Status == FileDeleted || files[index].Binary {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(files[index].Path)))
		if err != nil {
			continue
		}
		files[index].LineCount = countLines(content)
	}
}

// countLines counts lines, including a final line without a trailing newline
func countLines(content []byte) int {
	count := bytes.Count(content, []byte("\n"))
	if len(content) > 0 && content[len(content)-1] != '\n' {
		count++
	}
	return count
}

// findChangedFunctions maps the mapping diff (plus pure deletions from the
// display diff, which unified=0 hunk parsing drops) to changed functions
func findChangedFunctions(mappingDiff string, files []FileDiff, root string, quarantined map[string]bool) ([]*changedEntry, error) {
	parsed, err := check.ParseDiffOutput(mappingDiff)
	if err != nil {
		return nil, fmt.Errorf("parse mapping diff: %w", err)
	}
	parsed = append(parsed, deletionAnchorHunks(files)...)
	hunks := parsed[:0]
	for _, hunk := range parsed {
		if !quarantined[hunk.FilePath] {
			hunks = append(hunks, hunk)
		}
	}

	changed, err := check.MapHunksToFunctions(hunks, root)
	if err != nil {
		return nil, fmt.Errorf("map hunks to functions: %w", err)
	}
	sort.SliceStable(changed, func(left, right int) bool {
		if changed[left].FilePath != changed[right].FilePath {
			return changed[left].FilePath < changed[right].FilePath
		}
		return changed[left].StartLine < changed[right].StartLine
	})

	entries := make([]*changedEntry, 0, len(changed))
	for _, function := range changed {
		relative := check.RepoRelativePath(function.FilePath, root)
		entries = append(entries, &changedEntry{
			source:  function,
			absPath: function.FilePath,
			risk: FunctionRisk{
				Name:      function.Name,
				File:      relative,
				Language:  function.Language,
				StartLine: function.StartLine,
				EndLine:   function.EndLine,
				Callers:   []FunctionRef{},
				Callees:   []FunctionRef{},
			},
		})
		if function.HasReceiver {
			entries[len(entries)-1].risk.ReceiverType = function.ReceiverType
		}
	}
	return entries, nil
}

// deletionAnchorHunks returns a one-line hunk at the head line preceding each
// run of removed lines, so functions that only lost lines are still found
func deletionAnchorHunks(files []FileDiff) []check.DiffHunk {
	var hunks []check.DiffHunk
	for _, file := range files {
		if file.Status == FileDeleted {
			continue
		}
		for _, hunk := range file.Hunks {
			for _, anchor := range removalAnchors(hunk) {
				if anchor > 1 {
					hunks = append(hunks, check.DiffHunk{FilePath: file.Path, NewStart: anchor - 1, NewCount: 1})
				}
			}
		}
	}
	return hunks
}

// removalAnchors returns, for each run of removed lines in a hunk, the head-side
// line number that follows the removal
func removalAnchors(hunk Hunk) []int {
	var anchors []int
	for _, point := range removalPoints(hunk) {
		if len(anchors) == 0 || anchors[len(anchors)-1] != point {
			anchors = append(anchors, point)
		}
	}
	return anchors
}

// attachMetrics analyzes each changed file once and fills Metrics, line deltas and IsNew
func attachMetrics(entries []*changedEntry, files []FileDiff) {
	registry := languages.NewRegistry()
	analysisByPath := make(map[string]*models.FileAnalysis)
	filesByPath := make(map[string]*FileDiff, len(files))
	for index := range files {
		filesByPath[files[index].Path] = &files[index]
	}

	for _, entry := range entries {
		analysis, analyzed := analysisByPath[entry.absPath]
		if !analyzed {
			analysis = analyzeFile(registry, entry.absPath)
			analysisByPath[entry.absPath] = analysis
		}
		entry.risk.Metrics = matchFunctionMetrics(analysis, entry.risk)

		if fileDiff, exists := filesByPath[entry.risk.File]; exists {
			entry.risk.LinesAdded, entry.risk.LinesRemoved, entry.risk.IsNew =
				countChangesInRange(fileDiff, entry.risk.StartLine, entry.risk.EndLine)
		}
	}
}

// analyzeFile runs the language analyzer, returning nil on any failure
func analyzeFile(registry *languages.Registry, absPath string) *models.FileAnalysis {
	analyzer, err := registry.GetAnalyzerForFile(absPath)
	if err != nil || analyzer.IsStub() {
		return nil
	}
	analysis, err := analyzer.AnalyzeFile(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not analyze %s: %v\n", absPath, err)
		return nil
	}
	return analysis
}

// matchFunctionMetrics finds the analyzer's record for a function: same name and
// start line first, then same name with the largest overlapping line range
func matchFunctionMetrics(analysis *models.FileAnalysis, function FunctionRisk) *models.FunctionAnalysis {
	if analysis == nil {
		return nil
	}

	var best *models.FunctionAnalysis
	bestOverlap := 0
	for index := range analysis.Functions {
		candidate := &analysis.Functions[index]
		if candidate.Name != function.Name {
			continue
		}
		if candidate.StartLine == function.StartLine {
			return copyMetrics(candidate)
		}
		overlap := rangeOverlap(candidate.StartLine, candidate.EndLine, function.StartLine, function.EndLine)
		if overlap > bestOverlap {
			best, bestOverlap = candidate, overlap
		}
	}
	if best == nil {
		return nil
	}
	return copyMetrics(best)
}

func copyMetrics(metrics *models.FunctionAnalysis) *models.FunctionAnalysis {
	duplicate := *metrics
	return &duplicate
}

// rangeOverlap returns how many lines two inclusive ranges share
func rangeOverlap(startA, endA, startB, endB int) int {
	overlap := min(endA, endB) - max(startA, startB) + 1
	return max(overlap, 0)
}

// countChangesInRange counts added lines inside [start, end], removed lines
// whose removal point falls inside the function, and whether every line is new
func countChangesInRange(file *FileDiff, start, end int) (int, int, bool) {
	added, removed := 0, 0
	for _, hunk := range file.Hunks {
		added += countAddedInRange(hunk, start, end)
		for _, anchor := range removalPoints(hunk) {
			if anchor > start && anchor <= end {
				removed++
			}
		}
	}
	isNew := end >= start && added == end-start+1
	return added, removed, isNew
}

func countAddedInRange(hunk Hunk, start, end int) int {
	count := 0
	for _, line := range hunk.Lines {
		if line.Kind == LineAdded && line.NewLine >= start && line.NewLine <= end {
			count++
		}
	}
	return count
}

// removalPoints returns the head-side line each removed line would have preceded
func removalPoints(hunk Hunk) []int {
	var points []int
	nextHeadLine := hunk.NewStart
	for _, line := range hunk.Lines {
		if line.Kind == LineRemoved {
			points = append(points, nextHeadLine)
			continue
		}
		nextHeadLine = line.NewLine + 1
	}
	return points
}

// buildGoCallGraph builds the Go call graph over the worktree when any changed function is Go
func buildGoCallGraph(entries []*changedEntry, root string) *models.CallGraph {
	for _, entry := range entries {
		if entry.risk.Language != "Go" {
			continue
		}
		graph, err := golang.NewCallGraphAnalyzer().AnalyzeDirectory(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not build Go call graph: %v\n", err)
			return nil
		}
		return graph
	}
	return nil
}

// attachRelationships sets IDs, callers, callees and fan-in, returning the
// fan-in results used for blast-radius concerns
func attachRelationships(entries []*changedEntry, callGraph *models.CallGraph, root string) []check.FanInResult {
	var results []check.FanInResult
	var nonGo []check.ChangedFunction
	nonGoEntries := make(map[string]*changedEntry)

	for _, entry := range entries {
		if entry.risk.Language == "Go" && callGraph != nil {
			if fullName := check.MatchCallGraphName(entry.source, callGraph, root); fullName != "" {
				attachGoRelationships(entry, callGraph, fullName, root)
				results = append(results, fanInResult(entry, entry.risk.FanInApproximate))
				continue
			}
		}
		entry.risk.ID = entry.risk.File + ":" + entry.risk.Name
		if entry.risk.Language != "Go" {
			nonGo = append(nonGo, entry.source)
			nonGoEntries[fanInKey(entry.source)] = entry
		}
	}

	uniquifyIDs(entries)
	return append(results, attachGrepFanIn(nonGo, nonGoEntries, root)...)
}

// attachGoRelationships fills ID, callers, callees and fan-in from the call graph
func attachGoRelationships(entry *changedEntry, callGraph *models.CallGraph, fullName, root string) {
	entry.risk.ID = fullName
	entry.risk.FanIn = callSiteCount(callGraph, fullName)
	callers := callGraph.Callers(fullName)
	entry.risk.CallerCount = len(callers) // edges are deduplicated per caller
	entry.risk.FanInApproximate = anyInferred(callers)

	for _, edge := range callers {
		if len(entry.risk.Callers) >= maxRelatedFunctions {
			break
		}
		entry.risk.Callers = append(entry.risk.Callers, FunctionRef{
			ID:   edge.From,
			Name: nodeName(callGraph, edge.From),
			File: check.RepoRelativePath(edge.File, root),
			Line: edge.Line,
		})
	}

	for _, edge := range callGraph.Callees(fullName) {
		if len(entry.risk.Callees) >= maxRelatedFunctions {
			break
		}
		entry.risk.Callees = append(entry.risk.Callees, calleeRef(callGraph, edge.To, root))
	}
}

// callSiteCount is the number of call sites targeting fullName. It is derived
// from the edges rather than CallNode.CallCount, which the Go call-graph
// builder resets when a callee is defined in a file walked after its callers.
func callSiteCount(callGraph *models.CallGraph, fullName string) int {
	count := 0
	for _, edge := range callGraph.Callers(fullName) {
		count += edge.Weight
	}
	return count
}

// anyInferred reports whether any edge was resolved by method name alone, which
// makes the fan-in built from them approximate
func anyInferred(edges []models.CallEdge) bool {
	for _, edge := range edges {
		if edge.Inferred {
			return true
		}
	}
	return false
}

// calleeRef describes a callee by its definition site
func calleeRef(callGraph *models.CallGraph, fullName, root string) FunctionRef {
	node, exists := callGraph.Nodes[fullName]
	if !exists {
		return FunctionRef{ID: fullName, Name: fullName, External: true}
	}
	return FunctionRef{
		ID:       fullName,
		Name:     node.Name,
		File:     check.RepoRelativePath(node.File, root),
		Line:     node.Line,
		External: node.IsExternal,
	}
}

func nodeName(callGraph *models.CallGraph, fullName string) string {
	if node, exists := callGraph.Nodes[fullName]; exists {
		return node.Name
	}
	return fullName
}

// attachGrepFanIn computes approximate fan-in for non-Go functions
func attachGrepFanIn(nonGo []check.ChangedFunction, entries map[string]*changedEntry, root string) []check.FanInResult {
	if len(nonGo) == 0 {
		return nil
	}
	grepResults, err := check.ComputeFanIn(nonGo, root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not compute fan-in: %v\n", err)
		return nil
	}

	var results []check.FanInResult
	for _, grepResult := range grepResults {
		entry, exists := entries[fanInKey(grepResult.Function)]
		if !exists {
			continue
		}
		entry.risk.FanIn = grepResult.FanIn
		entry.risk.FanInApproximate = true
		results = append(results, fanInResult(entry, true))
	}
	return results
}

func fanInKey(function check.ChangedFunction) string {
	return fmt.Sprintf("%s:%s:%d", function.FilePath, function.Name, function.StartLine)
}

// fanInResult converts an entry into a FanInResult with a repo-relative path
func fanInResult(entry *changedEntry, approximate bool) check.FanInResult {
	function := entry.source
	function.FilePath = entry.risk.File
	return check.FanInResult{Function: function, FanIn: entry.risk.FanIn, Approximate: approximate}
}

// uniquifyIDs appends the start line to IDs that collide (overloads, same-named methods)
func uniquifyIDs(entries []*changedEntry) {
	counts := make(map[string]int)
	for _, entry := range entries {
		counts[entry.risk.ID]++
	}
	for _, entry := range entries {
		if counts[entry.risk.ID] > 1 {
			entry.risk.ID = fmt.Sprintf("%s:%d", entry.risk.ID, entry.risk.StartLine)
		}
	}
}

// attachChurn looks up function-level git churn for Go functions; failures leave churn unset
func attachChurn(ctx context.Context, entries []*changedEntry, root string, since time.Time) error {
	if since.IsZero() {
		since = time.Now().Add(-defaultChurnWindow)
	}
	churnAnalyzer := churn.NewGitChurnAnalyzer(root)

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.risk.Language != "Go" || entry.risk.Metrics == nil {
			continue
		}
		metric, err := churnAnalyzer.GetFunctionChurn(entry.absPath, entry.risk.Name, since)
		if err != nil || metric == nil {
			continue
		}
		entry.risk.Metrics.Churn = metric
	}
	return nil
}

// sortFunctionsByRisk orders functions by RiskScore descending, then by ID
func sortFunctionsByRisk(functions []FunctionRisk) {
	sort.SliceStable(functions, func(left, right int) bool {
		if functions[left].RiskScore != functions[right].RiskScore {
			return functions[left].RiskScore > functions[right].RiskScore
		}
		return functions[left].ID < functions[right].ID
	})
}

// applyFileRisk sets each file's RiskScore to the larger of its riskiest function and
// the file's own risk (CI config, scripts, dependency manifests), then sorts files by it
func applyFileRisk(files []FileDiff, functions []FunctionRisk) {
	maxByFile := make(map[string]float64)
	hasFunctions := make(map[string]bool)
	for _, function := range functions {
		maxByFile[function.File] = max(maxByFile[function.File], function.RiskScore)
		hasFunctions[function.File] = true
	}
	for index := range files {
		file := &files[index]
		fileScore, reasons := ScoreFile(*file, hasFunctions[file.Path])
		file.RiskScore = max(file.RiskScore, maxByFile[file.Path], fileScore)
		file.RiskReasons = append(file.RiskReasons, reasons...)
	}
	sort.SliceStable(files, func(left, right int) bool {
		if files[left].RiskScore != files[right].RiskScore {
			return files[left].RiskScore > files[right].RiskScore
		}
		return files[left].Path < files[right].Path
	})
}

// buildReviewGraph includes every changed function plus one hop of callers and
// callees, adding neighbourhoods in risk order until maxGraphNodes is reached.
// functions must already be sorted by risk.
func buildReviewGraph(functions []FunctionRisk, callGraph *models.CallGraph, root string) ReviewGraph {
	builder := newGraphBuilder(callGraph, root)
	for index := range functions {
		builder.addChanged(&functions[index])
	}
	for index := range functions {
		builder.addNeighbours(&functions[index])
	}
	return ReviewGraph{Nodes: builder.nodes, Edges: builder.edges()}
}

// graphBuilder accumulates deduplicated review-graph nodes
type graphBuilder struct {
	callGraph *models.CallGraph
	root      string
	nodes     []GraphNode
	included  map[string]bool
}

func newGraphBuilder(callGraph *models.CallGraph, root string) *graphBuilder {
	return &graphBuilder{callGraph: callGraph, root: root, nodes: []GraphNode{}, included: make(map[string]bool)}
}

func (builder *graphBuilder) full() bool {
	return len(builder.nodes) >= maxGraphNodes
}

// addChanged adds a changed function as a node
func (builder *graphBuilder) addChanged(function *FunctionRisk) {
	if builder.full() || builder.included[function.ID] {
		return
	}
	complexity := 0
	if function.Metrics != nil {
		complexity = function.Metrics.CyclomaticComplexity
	}
	builder.included[function.ID] = true
	builder.nodes = append(builder.nodes, GraphNode{
		ID:         function.ID,
		Name:       function.Name,
		File:       function.File,
		Line:       function.StartLine,
		Kind:       NodeChanged,
		RiskScore:  function.RiskScore,
		Focus:      function.Focus,
		FanIn:      function.FanIn,
		Complexity: complexity,
	})
}

// addNeighbours adds a changed function's callers and callees
func (builder *graphBuilder) addNeighbours(function *FunctionRisk) {
	for _, caller := range function.Callers {
		builder.addRelated(caller, NodeCaller)
	}
	for _, callee := range function.Callees {
		kind := NodeCallee
		if callee.External {
			kind = NodeExternal
		}
		builder.addRelated(callee, kind)
	}
}

// addRelated adds a caller or callee node, preferring call-graph definition details
func (builder *graphBuilder) addRelated(ref FunctionRef, kind GraphNodeKind) {
	if builder.full() || builder.included[ref.ID] {
		return
	}
	node := GraphNode{ID: ref.ID, Name: ref.Name, File: ref.File, Line: ref.Line, Kind: kind}
	if builder.callGraph != nil {
		if callNode, exists := builder.callGraph.Nodes[ref.ID]; exists {
			node.File = check.RepoRelativePath(callNode.File, builder.root)
			node.Line = callNode.Line
			node.FanIn = callSiteCount(builder.callGraph, ref.ID)
			node.Complexity = callNode.Complexity
		}
	}
	builder.included[ref.ID] = true
	builder.nodes = append(builder.nodes, node)
}

// edges returns call-graph edges whose endpoints are both included
func (builder *graphBuilder) edges() []GraphEdge {
	edges := []GraphEdge{}
	if builder.callGraph == nil {
		return edges
	}
	for _, edge := range builder.callGraph.Edges {
		if builder.included[edge.From] && builder.included[edge.To] {
			edges = append(edges, GraphEdge{From: edge.From, To: edge.To, Weight: edge.Weight})
		}
	}
	return edges
}
