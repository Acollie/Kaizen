package storage

import "time"

// SnapshotMetadata contains metadata about an analysis snapshot
type SnapshotMetadata struct {
	GitCommitHash string
	GitBranch     string
	KaizenVersion string
	ConfigHash    string
}

// SnapshotSummary provides quick access to snapshot info without loading full data
type SnapshotSummary struct {
	ID                     int64
	AnalyzedAt             time.Time
	GitCommitHash          string
	GitBranch              string
	TotalFiles             int
	TotalFunctions         int
	AvgCyclomaticComplexity float64
	AvgMaintainabilityIndex float64
	HotspotCount           int
	OverallGrade           string
	OverallScore           float64
	ComplexityScore        float64
	MaintainabilityScore   float64
	ChurnScore             float64
}

// TimeSeriesPoint represents a single data point in a time series
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}

// ComparisonResult represents differences between two snapshots
type ComparisonResult struct {
	Snapshot1 SnapshotSummary
	Snapshot2 SnapshotSummary
	MetricDeltas map[string]float64
	NewFunctionCount int
	RemovedFunctionCount int
	ChangedFunctions []FunctionChange
}

// FunctionChange represents changes to a specific function
type FunctionChange struct {
	FilePath string
	FunctionName string
	OldComplexity int
	NewComplexity int
	OldLength int
	NewLength int
	OldMaintainability float64
	NewMaintainability float64
}

// OwnerMetric represents aggregated metrics for a code owner
type OwnerMetric struct {
	Owner                           string
	FileCount                       int
	FunctionCount                   int
	TotalLines                      int
	AvgCyclomaticComplexity         float64
	AvgCognitiveComplexity          float64
	AvgMaintainabilityIndex         float64
	HotspotCount                    int
	HighComplexityFunctionCount     int
	OverallHealthScore              float64
}
