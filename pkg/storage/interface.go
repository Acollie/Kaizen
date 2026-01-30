package storage

import (
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

// StorageBackend defines the interface for storing and retrieving analysis results
type StorageBackend interface {
	// Save stores a new analysis result with metadata
	Save(result *models.AnalysisResult, metadata SnapshotMetadata) (int64, error)

	// GetLatest retrieves the most recent analysis snapshot
	GetLatest() (*models.AnalysisResult, error)

	// GetLatestSummary retrieves the most recent snapshot summary without full data
	GetLatestSummary() (*SnapshotSummary, error)

	// GetByID retrieves a specific snapshot by ID
	GetByID(id int64) (*models.AnalysisResult, error)

	// GetByIDSummary retrieves a specific snapshot summary by ID
	GetByIDSummary(id int64) (*SnapshotSummary, error)

	// GetRange retrieves snapshots within a time range
	GetRange(start, end time.Time, limit int) ([]SnapshotSummary, error)

	// GetTimeSeries retrieves metric history for trending
	// metricName: 'overall_score', 'cyclomatic_complexity', 'maintainability_index', etc.
	// scopePath: "" for repository level, path for folder/file level
	GetTimeSeries(metricName, scopePath string, start, end time.Time) ([]TimeSeriesPoint, error)

	// Compare diffs two snapshots
	Compare(id1, id2 int64) (*ComparisonResult, error)

	// ListSnapshots lists all snapshots (most recent first)
	ListSnapshots(limit int) ([]SnapshotSummary, error)

	// Prune removes snapshots older than retentionDays
	Prune(retentionDays int) (int, error)

	// DeleteSnapshot removes a specific snapshot
	DeleteSnapshot(id int64) error

	// Close closes the storage backend
	Close() error

	// IsHealthy checks if the backend is accessible
	IsHealthy() error

	// SaveOwnershipData saves file ownership and owner metrics
	SaveOwnershipData(snapshotID int64, fileOwnership map[string][]string, ownerMetrics []OwnerMetric, analyzedAt time.Time) error

	// GetOwnerMetrics retrieves owner metrics for a snapshot
	GetOwnerMetrics(snapshotID int64) ([]OwnerMetric, error)

	// GetFileOwnership retrieves ownership map for a snapshot
	GetFileOwnership(snapshotID int64) (map[string][]string, error)
}
