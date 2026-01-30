package models

import "time"

// AnalysisResult represents the complete analysis of a codebase
type AnalysisResult struct {
	Repository  string                   `json:"repository"`
	AnalyzedAt  time.Time                `json:"analyzed_at"`
	TimeRange   TimeRange                `json:"time_range"`
	Files       []FileAnalysis           `json:"files"`
	FolderStats map[string]FolderMetrics `json:"folder_stats"`
	Summary     SummaryMetrics           `json:"summary"`
	ScoreReport *ScoreReport             `json:"score_report,omitempty"`
}

// TimeRange represents the time period analyzed for churn
type TimeRange struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

// FileAnalysis contains all metrics for a single file
type FileAnalysis struct {
	Path     string `json:"path"`
	Language string `json:"language"`

	// Lines of code breakdown
	TotalLines            int     `json:"total_lines"`
	CodeLines             int     `json:"code_lines"`
	CommentLines          int     `json:"comment_lines"`
	BlankLines            int     `json:"blank_lines"`
	CommentDensity        float64 `json:"comment_density"`
	DuplicatedLines       int     `json:"duplicated_lines"`
	DuplicationPercentage float64 `json:"duplication_percentage"`

	// Dependencies
	ImportCount int `json:"import_count"`

	// Churn metrics
	Churn *ChurnMetric `json:"churn,omitempty"`

	// Function and type analysis
	Functions []FunctionAnalysis `json:"functions"`
	Types     []TypeAnalysis     `json:"types"`
}

// FunctionAnalysis contains metrics for a single function
type FunctionAnalysis struct {
	Name      string `json:"name"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`

	// Size metrics
	Length             int `json:"length"`
	LogicalLines       int `json:"logical_lines"`
	ParameterCount     int `json:"parameter_count"`
	LocalVariableCount int `json:"local_variable_count"`
	ReturnCount        int `json:"return_count"`

	// Complexity metrics
	CyclomaticComplexity int     `json:"cyclomatic_complexity"`
	CognitiveComplexity  int     `json:"cognitive_complexity"`
	NestingDepth         int     `json:"nesting_depth"`
	HalsteadVolume       float64 `json:"halstead_volume"`
	HalsteadDifficulty   float64 `json:"halstead_difficulty"`
	ABCScore             float64 `json:"abc_score"`

	// Quality metrics
	FanIn  int `json:"fan_in"`
	FanOut int `json:"fan_out"`

	// Churn metrics
	Churn *ChurnMetric `json:"churn,omitempty"`

	// Composite scores
	MaintainabilityIndex float64 `json:"maintainability_index"`
	IsHotspot            bool    `json:"is_hotspot"`
}

// TypeAnalysis contains metrics for a class/struct/interface
type TypeAnalysis struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // struct, interface, class

	// Coupling metrics
	AfferentCoupling int     `json:"afferent_coupling"`
	EfferentCoupling int     `json:"efferent_coupling"`
	Instability      float64 `json:"instability"`

	// Cohesion
	LCOM float64 `json:"lcom"` // Lack of Cohesion of Methods

	// Inheritance (mainly for Kotlin)
	DepthOfInheritance int `json:"depth_of_inheritance"`
	NumberOfChildren   int `json:"number_of_children"`

	// Methods
	MethodCount             int `json:"method_count"`
	WeightedMethodsPerClass int `json:"weighted_methods_per_class"`
	PublicMethodCount       int `json:"public_method_count"`

	Functions []FunctionAnalysis `json:"functions"`
}

// ChurnMetric represents version control churn data
type ChurnMetric struct {
	TotalCommits   int       `json:"total_commits"`
	LinesAdded     int       `json:"lines_added"`
	LinesDeleted   int       `json:"lines_deleted"`
	TotalChanges   int       `json:"total_changes"`
	LastModified   time.Time `json:"last_modified"`
	Contributors   []string  `json:"contributors"`
	ChurnScore     float64   `json:"churn_score"`      // Normalized 0-100
	AuthorCount    int       `json:"author_count"`     // Truck factor
	AverageChurnBy float64   `json:"average_churn_by"` // Average days between changes
}

// HalsteadMetrics represents Halstead complexity metrics
type HalsteadMetrics struct {
	DistinctOperators int     `json:"distinct_operators"` // n1
	DistinctOperands  int     `json:"distinct_operands"`  // n2
	TotalOperators    int     `json:"total_operators"`    // N1
	TotalOperands     int     `json:"total_operands"`     // N2
	Vocabulary        int     `json:"vocabulary"`         // n1 + n2
	Length            int     `json:"length"`             // N1 + N2
	Volume            float64 `json:"volume"`             // Length * log2(Vocabulary)
	Difficulty        float64 `json:"difficulty"`         // (n1/2) * (N2/n2)
	Effort            float64 `json:"effort"`             // Volume * Difficulty
	TimeToUnderstand  float64 `json:"time_to_understand"` // Effort / 18 (seconds)
	BugsDelivered     float64 `json:"bugs_delivered"`     // Volume / 3000
}

// FolderMetrics aggregates metrics for all files in a folder
type FolderMetrics struct {
	Path           string  `json:"path"`
	TotalFiles     int     `json:"total_files"`
	TotalFunctions int     `json:"total_functions"`
	TotalLines     int     `json:"total_lines"`
	TotalCodeLines int     `json:"total_code_lines"`
	TotalChurn     int     `json:"total_churn"`

	// Average metrics
	AverageComplexity     float64 `json:"average_complexity"`
	AverageCognitive      float64 `json:"average_cognitive"`
	AverageLength         float64 `json:"average_length"`
	AverageChurn          float64 `json:"average_churn"`
	AverageMaintainability float64 `json:"average_maintainability"`

	// Normalized scores for visualization (0-100)
	ComplexityScore      float64 `json:"complexity_score"`
	ChurnScore           float64 `json:"churn_score"`
	LengthScore          float64 `json:"length_score"`
	MaintainabilityScore float64 `json:"maintainability_score"`
	HotspotScore         float64 `json:"hotspot_score"` // Combined churn + complexity

	// Hotspot count
	HotspotCount int `json:"hotspot_count"`
}

// SummaryMetrics provides high-level statistics
type SummaryMetrics struct {
	TotalFiles                int     `json:"total_files"`
	TotalFunctions            int     `json:"total_functions"`
	TotalTypes                int     `json:"total_types"`
	TotalLines                int     `json:"total_lines"`
	TotalCodeLines            int     `json:"total_code_lines"`
	AverageCyclomaticComplexity float64 `json:"average_cyclomatic_complexity"`
	AverageCognitiveComplexity  float64 `json:"average_cognitive_complexity"`
	AverageFunctionLength     float64 `json:"average_function_length"`
	AverageMaintainabilityIndex float64 `json:"average_maintainability_index"`
	HotspotCount              int     `json:"hotspot_count"`
	HighComplexityCount       int     `json:"high_complexity_count"`       // >10
	VeryHighComplexityCount   int     `json:"very_high_complexity_count"` // >20
	LongFunctionCount         int     `json:"long_function_count"`        // >50 lines
	VeryLongFunctionCount     int     `json:"very_long_function_count"`   // >100 lines
}

// ScoreReport represents the overall health assessment of a codebase
type ScoreReport struct {
	OverallGrade    string          `json:"overall_grade"`    // A, B, C, D, F
	OverallScore    float64         `json:"overall_score"`    // 0-100
	ComponentScores ComponentScores `json:"component_scores"`
	Concerns        []Concern       `json:"concerns"`
	HasChurnData    bool            `json:"has_churn_data"`
}

// ComponentScores breaks down health by category
type ComponentScores struct {
	Complexity      CategoryScore `json:"complexity"`
	Maintainability CategoryScore `json:"maintainability"`
	Churn           CategoryScore `json:"churn"`
	FunctionSize    CategoryScore `json:"function_size"`
	CodeStructure   CategoryScore `json:"code_structure"`
}

// CategoryScore represents a single component's score
type CategoryScore struct {
	Score    float64 `json:"score"`    // 0-100 (higher = better)
	Weight   float64 `json:"weight"`   // Weight in overall calculation
	Category string  `json:"category"` // "excellent", "good", "moderate", "poor", "critical"
}

// Concern represents an area needing attention
type Concern struct {
	Type          string         `json:"type"`
	Severity      string         `json:"severity"` // "critical", "warning", "info"
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	AffectedItems []AffectedItem `json:"affected_items"`
}

// AffectedItem references a specific file or function
type AffectedItem struct {
	FilePath     string             `json:"file_path"`
	FunctionName string             `json:"function_name,omitempty"`
	Line         int                `json:"line,omitempty"`
	Metrics      map[string]float64 `json:"metrics"`
}
