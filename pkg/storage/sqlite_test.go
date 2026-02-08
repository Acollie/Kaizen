package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/models"
)

// TestSQLiteBackendSaveAndRetrieve tests basic save and retrieve functionality
func TestSQLiteBackendSaveAndRetrieve(testingT *testing.T) {
	tempDir, err := os.MkdirTemp("", "kaizen-test-")
	require.NoError(testingT, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	dbPath := tempDir + "/test.db"

	// Create backend
	backend, err := NewSQLiteBackend(dbPath)
	require.NoError(testingT, err)
	defer func() { _ = backend.Close() }()

	// Create test result
	result := &models.AnalysisResult{
		Repository: "test",
		AnalyzedAt: time.Now(),
		TimeRange: models.TimeRange{
			Since: time.Now().AddDate(0, 0, -90),
			Until: time.Now(),
		},
		Files: []models.FileAnalysis{
			{
				Path:     "test.go",
				Language: "golang",
				Functions: []models.FunctionAnalysis{
					{
						Name:                     "TestFunc",
						Length:                   20,
						CyclomaticComplexity:     2,
						CognitiveComplexity:      2,
						MaintainabilityIndex:     85.0,
						IsHotspot:                false,
					},
				},
			},
		},
		FolderStats: make(map[string]models.FolderMetrics),
		Summary: models.SummaryMetrics{
			TotalFiles:                    1,
			TotalFunctions:                1,
			TotalLines:                    100,
			TotalCodeLines:                80,
			AverageCyclomaticComplexity:   2.0,
			AverageCognitiveComplexity:    2.0,
			AverageFunctionLength:         20.0,
			AverageMaintainabilityIndex:   85.0,
			HotspotCount:                  0,
		},
		ScoreReport: &models.ScoreReport{
			OverallGrade: "A",
			OverallScore: 90.0,
			HasChurnData: false,
		},
	}

	// Save result
	id, err := backend.Save(result, SnapshotMetadata{
		KaizenVersion: "1.0.0",
	})
	require.NoError(testingT, err)
	require.Greater(testingT, id, int64(0))

	// Retrieve latest
	retrieved, err := backend.GetLatest()
	require.NoError(testingT, err)
	require.NotNil(testingT, retrieved)
	assert.Equal(testingT, result.Repository, retrieved.Repository)

	// Get summary
	summary, err := backend.GetLatestSummary()
	require.NoError(testingT, err)
	require.NotNil(testingT, summary)
	assert.Equal(testingT, id, summary.ID)
	assert.Equal(testingT, 1, summary.TotalFunctions)

	// List snapshots
	snapshots, err := backend.ListSnapshots(10)
	require.NoError(testingT, err)
	assert.NotEmpty(testingT, snapshots)

	// Get time series
	points, err := backend.GetTimeSeries("overall_score", "", time.Now().AddDate(0, 0, -90), time.Now())
	require.NoError(testingT, err)
	assert.NotEmpty(testingT, points)
}

// TestSQLiteBackendMultipleSnapshots tests appending multiple snapshots
func TestSQLiteBackendMultipleSnapshots(testingT *testing.T) {
	tempDir, err := os.MkdirTemp("", "kaizen-test-")
	require.NoError(testingT, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	dbPath := tempDir + "/test-multi.db"

	backend, err := NewSQLiteBackend(dbPath)
	require.NoError(testingT, err)
	defer func() { _ = backend.Close() }()

	// Save first snapshot
	result1 := createTestResult("first", 1, 90.0)
	id1, err := backend.Save(result1, SnapshotMetadata{KaizenVersion: "1.0.0"})
	require.NoError(testingT, err)

	// Save second snapshot (slightly different)
	time.Sleep(100 * time.Millisecond) // Ensure different timestamp
	result2 := createTestResult("second", 2, 92.0)
	id2, err := backend.Save(result2, SnapshotMetadata{KaizenVersion: "1.0.0"})
	require.NoError(testingT, err)

	// Verify IDs are different
	assert.NotEqual(testingT, id1, id2, "Snapshot IDs should be unique")

	// Get range
	snapshots, err := backend.GetRange(
		time.Now().AddDate(0, 0, -1),
		time.Now(),
		10,
	)
	require.NoError(testingT, err)
	assert.Len(testingT, snapshots, 2, "Should have 2 snapshots")

	// Compare snapshots
	comparison, err := backend.Compare(id1, id2)
	require.NoError(testingT, err)
	require.NotNil(testingT, comparison)
	assert.Equal(testingT, id1, comparison.Snapshot1.ID)
	assert.Equal(testingT, id2, comparison.Snapshot2.ID)
}

// createTestResult creates a test AnalysisResult with given parameters
func createTestResult(name string, functionCount int, score float64) *models.AnalysisResult {
	functions := make([]models.FunctionAnalysis, functionCount)
	for i := 0; i < functionCount; i++ {
		functions[i] = models.FunctionAnalysis{
			Name:                   "Func",
			Length:                 20,
			CyclomaticComplexity:   2,
			CognitiveComplexity:    2,
			MaintainabilityIndex:   85.0,
			IsHotspot:              false,
		}
	}

	return &models.AnalysisResult{
		Repository: name,
		AnalyzedAt: time.Now(),
		TimeRange: models.TimeRange{
			Since: time.Now().AddDate(0, 0, -90),
			Until: time.Now(),
		},
		Files: []models.FileAnalysis{
			{
				Path:      "test.go",
				Language:  "golang",
				Functions: functions,
			},
		},
		FolderStats: make(map[string]models.FolderMetrics),
		Summary: models.SummaryMetrics{
			TotalFiles:                  1,
			TotalFunctions:              functionCount,
			TotalLines:                  100,
			TotalCodeLines:              80,
			AverageCyclomaticComplexity: 2.0,
			AverageCognitiveComplexity:  2.0,
			AverageFunctionLength:       20.0,
			AverageMaintainabilityIndex: 85.0,
			HotspotCount:                0,
		},
		ScoreReport: &models.ScoreReport{
			OverallGrade: "A",
			OverallScore: score,
			HasChurnData: false,
		},
	}
}
