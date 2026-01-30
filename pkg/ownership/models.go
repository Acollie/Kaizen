package ownership

// OwnershipRule represents a single entry in CODEOWNERS file
type OwnershipRule struct {
	Pattern    string   `json:"pattern"`
	Owners     []string `json:"owners"`
	LineNumber int      `json:"line_number"`
}

// CodeOwners represents the parsed CODEOWNERS file
type CodeOwners struct {
	Rules []OwnershipRule `json:"rules"`
	Path  string          `json:"path"`
}

// FileOwnership maps a file to its owners
type FileOwnership struct {
	FilePath string   `json:"file_path"`
	Owners   []string `json:"owners"`
	Patterns []string `json:"matched_patterns"`
}

// OwnerMetrics aggregates metrics for a single owner
type OwnerMetrics struct {
	Owner                      string  `json:"owner"`
	FileCount                  int     `json:"file_count"`
	FunctionCount              int     `json:"function_count"`
	TotalLines                 int     `json:"total_lines"`
	AvgCyclomaticComplexity    float64 `json:"avg_cyclomatic_complexity"`
	AvgCognitiveComplexity     float64 `json:"avg_cognitive_complexity"`
	AvgMaintainabilityIndex    float64 `json:"avg_maintainability_index"`
	HotspotCount               int     `json:"hotspot_count"`
	HighComplexityFunctionCount int    `json:"high_complexity_function_count"`
	OverallHealthScore         float64 `json:"overall_health_score"`
}

// OwnerReport represents ownership report for a snapshot
type OwnerReport struct {
	SnapshotID      int64           `json:"snapshot_id"`
	AnalyzedAt      string          `json:"analyzed_at"`
	TotalOwners     int             `json:"total_owners"`
	OwnerMetrics    []OwnerMetrics  `json:"owner_metrics"`
	FileOwnershipMap map[string][]string `json:"file_ownership_map,omitempty"`
}
