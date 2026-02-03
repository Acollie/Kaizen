package check

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// LineRange represents a range of line numbers
type LineRange struct {
	Start int // inclusive, 1-based
	End   int // inclusive, 1-based
}

// ChangedFunction represents a function that was modified in a diff
type ChangedFunction struct {
	FilePath     string
	Name         string  // bare function name (e.g. "Analyze", not qualified)
	HasReceiver  bool    // true if this is a method
	ReceiverType string  // the receiver type name if HasReceiver is true
	StartLine    int
	EndLine      int
	Language     string  // "Go", "Python", "Kotlin", "Swift"
}

// MapHunksToFunctions determines which functions contain changed lines
func MapHunksToFunctions(hunks []DiffHunk, repoPath string) ([]ChangedFunction, error) {
	if len(hunks) == 0 {
		return nil, nil
	}

	// Group hunks by file
	lineRangesByFile := hunksToLineRanges(hunks)

	var allChangedFunctions []ChangedFunction

	for filePath, ranges := range lineRangesByFile {
		// Skip if file doesn't exist
		fullPath := filepath.Join(repoPath, filePath)
		if _, err := os.Stat(fullPath); err != nil {
			// Log to stderr and skip
			fmt.Fprintf(os.Stderr, "Warning: file not found on disk: %s\n", filePath)
			continue
		}

		// Dispatch by extension
		var functions []ChangedFunction
		var err error

		ext := filepath.Ext(filePath)
		switch ext {
		case ".go":
			functions, err = extractGoFunctions(fullPath, ranges)
		case ".py":
			functions, err = extractHeuristicFunctions(fullPath, ranges, "python")
		case ".kt":
			functions, err = extractHeuristicFunctions(fullPath, ranges, "kotlin")
		case ".swift":
			functions, err = extractHeuristicFunctions(fullPath, ranges, "swift")
		default:
			// Unsupported extension, skip
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to extract functions from %s: %v\n", filePath, err)
			continue
		}

		allChangedFunctions = append(allChangedFunctions, functions...)
	}

	return allChangedFunctions, nil
}

// hunksToLineRanges groups hunks by file and converts to line ranges
func hunksToLineRanges(hunks []DiffHunk) map[string][]LineRange {
	result := make(map[string][]LineRange)
	for _, hunk := range hunks {
		lineEnd := hunk.NewStart + hunk.NewCount - 1
		result[hunk.FilePath] = append(result[hunk.FilePath], LineRange{
			Start: hunk.NewStart,
			End:   lineEnd,
		})
	}
	return result
}

// extractGoFunctions parses a Go file and returns functions with lines in the given ranges
func extractGoFunctions(filePath string, ranges []LineRange) ([]ChangedFunction, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	var functions []ChangedFunction

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		startLine := fset.Position(funcDecl.Pos()).Line
		endLine := fset.Position(funcDecl.End()).Line

		// Check if function overlaps with any changed range
		if functionOverlapsRanges(startLine, endLine, ranges) {
			changedFunc := ChangedFunction{
				FilePath:  filePath,
				Name:      funcDecl.Name.Name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  "Go",
			}

			// Check if it's a method
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				changedFunc.HasReceiver = true
				changedFunc.ReceiverType = extractGoReceiverType(funcDecl.Recv.List[0].Type)
			}

			functions = append(functions, changedFunc)
		}
	}

	return functions, nil
}

// extractGoReceiverType extracts the receiver type from a receiver expression
func extractGoReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return extractGoReceiverType(t.X)
	default:
		return "Unknown"
	}
}

// extractHeuristicFunctions extracts functions using line-by-line heuristics for non-Go languages
func extractHeuristicFunctions(filePath string, ranges []LineRange, language string) ([]ChangedFunction, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var functions []ChangedFunction
	seen := make(map[string]bool) // Deduplicate by (name, startLine)

	keyword := ""
	switch language {
	case "python":
		keyword = "def "
	case "kotlin":
		keyword = "fun "
	case "swift":
		keyword = "func "
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	for lineNum := 1; lineNum <= len(lines); lineNum++ {
		line := lines[lineNum-1]
		trimmedLine := strings.TrimSpace(line)

		// Look for function definition keyword
		if !strings.HasPrefix(trimmedLine, keyword) {
			continue
		}

		// Extract function name from the line
		funcName := extractFunctionName(trimmedLine, keyword, language)
		if funcName == "" {
			continue
		}

		// Check if this line overlaps with a changed range
		if !lineInRanges(lineNum, ranges) {
			continue
		}

		// Deduplicate
		key := fmt.Sprintf("%s:%d", funcName, lineNum)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Estimate function end line based on indentation
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		endLine := estimateEndLine(lines, lineNum, indent, language)

		functions = append(functions, ChangedFunction{
			FilePath:  filePath,
			Name:      funcName,
			StartLine: lineNum,
			EndLine:   endLine,
			Language:  strings.Title(language), // Capitalize first letter
		})
	}

	return functions, nil
}

// extractFunctionName extracts the function name from a line
func extractFunctionName(line, keyword, language string) string {
	// Remove keyword
	afterKeyword := strings.TrimPrefix(line, keyword)
	afterKeyword = strings.TrimSpace(afterKeyword)

	// Find the name (before parentheses)
	if idx := strings.IndexByte(afterKeyword, '('); idx > 0 {
		return strings.TrimSpace(afterKeyword[:idx])
	}

	return ""
}

// estimateEndLine estimates where a function ends based on indentation
func estimateEndLine(lines []string, startLine int, baseIndent int, language string) int {
	endLine := startLine

	for i := startLine; i < len(lines); i++ {
		if i == startLine {
			continue // Skip the definition line itself
		}

		line := lines[i]

		// Empty lines are okay
		if strings.TrimSpace(line) == "" {
			endLine = i + 1
			continue
		}

		// Comments at function scope
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && language == "python" {
			endLine = i + 1
			continue
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		// If we encounter a line with <= base indent and it's not empty, function likely ended
		if currentIndent <= baseIndent && strings.TrimSpace(line) != "" {
			break
		}

		endLine = i + 1
	}

	return endLine
}

// functionOverlapsRanges checks if a function's line range overlaps with any changed range
func functionOverlapsRanges(startLine, endLine int, ranges []LineRange) bool {
	for _, r := range ranges {
		// Check for overlap: [startLine, endLine] overlaps with [r.Start, r.End]
		if startLine <= r.End && endLine >= r.Start {
			return true
		}
	}
	return false
}

// lineInRanges checks if a line number falls within any of the ranges
func lineInRanges(lineNum int, ranges []LineRange) bool {
	for _, r := range ranges {
		if lineNum >= r.Start && lineNum <= r.End {
			return true
		}
	}
	return false
}
