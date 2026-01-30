package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/glebarez/sqlite"

	"github.com/alexcollie/kaizen/pkg/models"
)

// SQLiteBackend implements StorageBackend using SQLite
type SQLiteBackend struct {
	database *sql.DB
	path     string
}

// NewSQLiteBackend creates or opens a SQLite database at the given path
func NewSQLiteBackend(path string) (*SQLiteBackend, error) {
	// Open or create database
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	_, err = database.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Run migrations
	err = runMigrations(database)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &SQLiteBackend{
		database: database,
		path:     path,
	}, nil
}

// Save stores a new analysis result
func (backend *SQLiteBackend) Save(result *models.AnalysisResult, metadata SnapshotMetadata) (int64, error) {
	// Serialize full result as JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Extract score report values
	var overallGrade, overallScore, complexityScore, maintainabilityScore, churnScore interface{}
	var hasChurnData bool
	if result.ScoreReport != nil {
		overallGrade = result.ScoreReport.OverallGrade
		overallScore = result.ScoreReport.OverallScore
		complexityScore = result.ScoreReport.ComponentScores.Complexity.Score
		maintainabilityScore = result.ScoreReport.ComponentScores.Maintainability.Score
		churnScore = result.ScoreReport.ComponentScores.Churn.Score
		hasChurnData = result.ScoreReport.HasChurnData
	}

	// Insert snapshot
	execResult, err := backend.database.Exec(`
		INSERT INTO analysis_snapshots (
			analyzed_at, git_commit_hash, git_branch, kaizen_version, config_hash,
			total_files, total_functions, total_lines, total_code_lines,
			avg_cyclomatic_complexity, avg_cognitive_complexity, avg_function_length,
			avg_maintainability_index, hotspot_count,
			overall_grade, overall_score, complexity_score, maintainability_score,
			churn_score, has_churn_data, full_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.AnalyzedAt,
		metadata.GitCommitHash,
		metadata.GitBranch,
		metadata.KaizenVersion,
		metadata.ConfigHash,
		result.Summary.TotalFiles,
		result.Summary.TotalFunctions,
		result.Summary.TotalLines,
		result.Summary.TotalCodeLines,
		result.Summary.AverageCyclomaticComplexity,
		result.Summary.AverageCognitiveComplexity,
		result.Summary.AverageFunctionLength,
		result.Summary.AverageMaintainabilityIndex,
		result.Summary.HotspotCount,
		overallGrade,
		overallScore,
		complexityScore,
		maintainabilityScore,
		churnScore,
		hasChurnData,
		jsonData,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	snapshotID, err := execResult.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get snapshot ID: %w", err)
	}

	// Insert time-series metrics (repository level)
	err = backend.insertRepositoryMetrics(snapshotID, result)
	if err != nil {
		return 0, fmt.Errorf("failed to insert repository metrics: %w", err)
	}

	// Insert folder-level metrics
	err = backend.insertFolderMetrics(snapshotID, result)
	if err != nil {
		return 0, fmt.Errorf("failed to insert folder metrics: %w", err)
	}

	// Insert function history
	err = backend.insertFunctionHistory(snapshotID, result)
	if err != nil {
		return 0, fmt.Errorf("failed to insert function history: %w", err)
	}

	return snapshotID, nil
}

// insertRepositoryMetrics inserts repository-level time-series metrics
func (backend *SQLiteBackend) insertRepositoryMetrics(snapshotID int64, result *models.AnalysisResult) error {
	stmt, err := backend.database.Prepare(`
		INSERT INTO metrics_timeseries (snapshot_id, analyzed_at, metric_name, scope, scope_path, value)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	metrics := map[string]float64{
		"overall_score":                 0,
		"avg_cyclomatic_complexity":     result.Summary.AverageCyclomaticComplexity,
		"avg_cognitive_complexity":      result.Summary.AverageCognitiveComplexity,
		"avg_function_length":           result.Summary.AverageFunctionLength,
		"avg_maintainability_index":     result.Summary.AverageMaintainabilityIndex,
		"hotspot_count":                 float64(result.Summary.HotspotCount),
	}

	// Add score report metrics if available
	if result.ScoreReport != nil {
		metrics["overall_score"] = result.ScoreReport.OverallScore
		metrics["complexity_score"] = result.ScoreReport.ComponentScores.Complexity.Score
		metrics["maintainability_score"] = result.ScoreReport.ComponentScores.Maintainability.Score
		metrics["churn_score"] = result.ScoreReport.ComponentScores.Churn.Score
	}

	for metricName, value := range metrics {
		_, err := stmt.Exec(snapshotID, result.AnalyzedAt, metricName, "repository", "", value)
		if err != nil {
			return err
		}
	}

	return nil
}

// insertFolderMetrics inserts folder-level time-series metrics
func (backend *SQLiteBackend) insertFolderMetrics(snapshotID int64, result *models.AnalysisResult) error {
	stmt, err := backend.database.Prepare(`
		INSERT INTO metrics_timeseries (snapshot_id, analyzed_at, metric_name, scope, scope_path, value)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	folderMetricNames := []string{
		"complexity_score",
		"churn_score",
		"maintainability_score",
		"hotspot_score",
		"hotspot_count",
	}

	for folderPath, folderMetrics := range result.FolderStats {
		for _, metricName := range folderMetricNames {
			var value float64

			switch metricName {
			case "complexity_score":
				value = folderMetrics.ComplexityScore
			case "churn_score":
				value = folderMetrics.ChurnScore
			case "maintainability_score":
				value = folderMetrics.MaintainabilityScore
			case "hotspot_score":
				value = folderMetrics.HotspotScore
			case "hotspot_count":
				value = float64(folderMetrics.HotspotCount)
			}

			_, err := stmt.Exec(snapshotID, result.AnalyzedAt, metricName, "folder", folderPath, value)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// insertFunctionHistory inserts function-level historical data
func (backend *SQLiteBackend) insertFunctionHistory(snapshotID int64, result *models.AnalysisResult) error {
	stmt, err := backend.database.Prepare(`
		INSERT INTO function_history (
			snapshot_id, file_path, function_name,
			length, cyclomatic_complexity, cognitive_complexity,
			maintainability_index, total_commits, is_hotspot, analyzed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, fileAnalysis := range result.Files {
		for _, funcAnalysis := range fileAnalysis.Functions {
			totalCommits := 0
			if funcAnalysis.Churn != nil {
				totalCommits = funcAnalysis.Churn.TotalCommits
			}

			_, err := stmt.Exec(
				snapshotID,
				fileAnalysis.Path,
				funcAnalysis.Name,
				funcAnalysis.Length,
				funcAnalysis.CyclomaticComplexity,
				funcAnalysis.CognitiveComplexity,
				funcAnalysis.MaintainabilityIndex,
				totalCommits,
				funcAnalysis.IsHotspot,
				result.AnalyzedAt,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetLatest retrieves the most recent analysis
func (backend *SQLiteBackend) GetLatest() (*models.AnalysisResult, error) {
	var jsonData string
	err := backend.database.QueryRow(`
		SELECT full_data FROM analysis_snapshots
		ORDER BY analyzed_at DESC LIMIT 1
	`).Scan(&jsonData)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no analysis snapshots found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	var result models.AnalysisResult
	err = json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &result, nil
}

// GetLatestSummary retrieves the most recent snapshot summary
func (backend *SQLiteBackend) GetLatestSummary() (*SnapshotSummary, error) {
	return backend.GetByIDSummary(0) // Use helper that handles 0 as "get latest"
}

// GetByID retrieves a specific snapshot by ID
func (backend *SQLiteBackend) GetByID(id int64) (*models.AnalysisResult, error) {
	var jsonData string
	err := backend.database.QueryRow(`
		SELECT full_data FROM analysis_snapshots WHERE id = ?
	`, id).Scan(&jsonData)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	var result models.AnalysisResult
	err = json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &result, nil
}

// GetByIDSummary retrieves a snapshot summary by ID (or latest if id=0)
func (backend *SQLiteBackend) GetByIDSummary(id int64) (*SnapshotSummary, error) {
	query := `
		SELECT
			id, analyzed_at, git_commit_hash, git_branch,
			total_files, total_functions,
			avg_cyclomatic_complexity, avg_maintainability_index,
			hotspot_count, overall_grade, overall_score,
			complexity_score, maintainability_score, churn_score
		FROM analysis_snapshots
	`

	var args []interface{}
	if id > 0 {
		query += " WHERE id = ?"
		args = append(args, id)
	} else {
		query += " ORDER BY analyzed_at DESC LIMIT 1"
	}

	summary := &SnapshotSummary{}
	err := backend.database.QueryRow(query, args...).Scan(
		&summary.ID, &summary.AnalyzedAt, &summary.GitCommitHash, &summary.GitBranch,
		&summary.TotalFiles, &summary.TotalFunctions,
		&summary.AvgCyclomaticComplexity, &summary.AvgMaintainabilityIndex,
		&summary.HotspotCount, &summary.OverallGrade, &summary.OverallScore,
		&summary.ComplexityScore, &summary.MaintainabilityScore, &summary.ChurnScore,
	)

	if err == sql.ErrNoRows {
		if id > 0 {
			return nil, fmt.Errorf("snapshot %d not found", id)
		}
		return nil, fmt.Errorf("no analysis snapshots found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	return summary, nil
}

// GetRange retrieves snapshots within a time range
func (backend *SQLiteBackend) GetRange(start, end time.Time, limit int) ([]SnapshotSummary, error) {
	query := `
		SELECT
			id, analyzed_at, git_commit_hash, git_branch,
			total_files, total_functions,
			avg_cyclomatic_complexity, avg_maintainability_index,
			hotspot_count, overall_grade, overall_score,
			complexity_score, maintainability_score, churn_score
		FROM analysis_snapshots
		WHERE analyzed_at BETWEEN ? AND ?
		ORDER BY analyzed_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := backend.database.Query(query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	var summaries []SnapshotSummary
	for rows.Next() {
		summary := SnapshotSummary{}
		err := rows.Scan(
			&summary.ID, &summary.AnalyzedAt, &summary.GitCommitHash, &summary.GitBranch,
			&summary.TotalFiles, &summary.TotalFunctions,
			&summary.AvgCyclomaticComplexity, &summary.AvgMaintainabilityIndex,
			&summary.HotspotCount, &summary.OverallGrade, &summary.OverallScore,
			&summary.ComplexityScore, &summary.MaintainabilityScore, &summary.ChurnScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshots: %w", err)
	}

	return summaries, nil
}

// GetTimeSeries retrieves metric history for trending
func (backend *SQLiteBackend) GetTimeSeries(metricName, scopePath string, start, end time.Time) ([]TimeSeriesPoint, error) {
	query := `
		SELECT analyzed_at, value
		FROM metrics_timeseries
		WHERE metric_name = ? AND analyzed_at BETWEEN ? AND ?
	`
	args := []interface{}{metricName, start, end}

	if scopePath != "" {
		query += " AND scope_path = ?"
		args = append(args, scopePath)
	} else {
		query += " AND scope = 'repository'"
	}

	query += " ORDER BY analyzed_at ASC"

	rows, err := backend.database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		point := TimeSeriesPoint{}
		err := rows.Scan(&point.Timestamp, &point.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		points = append(points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics: %w", err)
	}

	return points, nil
}

// Compare diffs two snapshots
func (backend *SQLiteBackend) Compare(id1, id2 int64) (*ComparisonResult, error) {
	snap1, err := backend.GetByIDSummary(id1)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot 1: %w", err)
	}

	snap2, err := backend.GetByIDSummary(id2)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot 2: %w", err)
	}

	result := &ComparisonResult{
		Snapshot1: *snap1,
		Snapshot2: *snap2,
		MetricDeltas: make(map[string]float64),
	}

	// Calculate metric deltas
	if snap2.OverallScore > 0 {
		result.MetricDeltas["overall_score"] = snap2.OverallScore - snap1.OverallScore
	}
	if snap2.ComplexityScore > 0 {
		result.MetricDeltas["complexity_score"] = snap2.ComplexityScore - snap1.ComplexityScore
	}
	if snap2.MaintainabilityScore > 0 {
		result.MetricDeltas["maintainability_score"] = snap2.MaintainabilityScore - snap1.MaintainabilityScore
	}
	if snap2.ChurnScore > 0 {
		result.MetricDeltas["churn_score"] = snap2.ChurnScore - snap1.ChurnScore
	}

	result.MetricDeltas["hotspot_count"] = float64(snap2.HotspotCount - snap1.HotspotCount)
	result.MetricDeltas["total_files"] = float64(snap2.TotalFiles - snap1.TotalFiles)
	result.MetricDeltas["total_functions"] = float64(snap2.TotalFunctions - snap1.TotalFunctions)

	return result, nil
}

// ListSnapshots lists all snapshots most recent first
func (backend *SQLiteBackend) ListSnapshots(limit int) ([]SnapshotSummary, error) {
	query := `
		SELECT
			id, analyzed_at, git_commit_hash, git_branch,
			total_files, total_functions,
			avg_cyclomatic_complexity, avg_maintainability_index,
			hotspot_count, overall_grade, overall_score,
			complexity_score, maintainability_score, churn_score
		FROM analysis_snapshots
		ORDER BY analyzed_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := backend.database.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	var summaries []SnapshotSummary
	for rows.Next() {
		summary := SnapshotSummary{}
		err := rows.Scan(
			&summary.ID, &summary.AnalyzedAt, &summary.GitCommitHash, &summary.GitBranch,
			&summary.TotalFiles, &summary.TotalFunctions,
			&summary.AvgCyclomaticComplexity, &summary.AvgMaintainabilityIndex,
			&summary.HotspotCount, &summary.OverallGrade, &summary.OverallScore,
			&summary.ComplexityScore, &summary.MaintainabilityScore, &summary.ChurnScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshots: %w", err)
	}

	return summaries, nil
}

// Prune removes snapshots older than retentionDays
func (backend *SQLiteBackend) Prune(retentionDays int) (int, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	result, err := backend.database.Exec(`
		DELETE FROM analysis_snapshots WHERE analyzed_at < ?
	`, cutoffDate)

	if err != nil {
		return 0, fmt.Errorf("failed to prune snapshots: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// DeleteSnapshot removes a specific snapshot
func (backend *SQLiteBackend) DeleteSnapshot(id int64) error {
	result, err := backend.database.Exec(`
		DELETE FROM analysis_snapshots WHERE id = ?
	`, id)

	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("snapshot %d not found", id)
	}

	return nil
}

// Close closes the database connection
func (backend *SQLiteBackend) Close() error {
	if backend.database != nil {
		return backend.database.Close()
	}
	return nil
}

// IsHealthy checks if the backend is accessible
func (backend *SQLiteBackend) IsHealthy() error {
	return backend.database.Ping()
}

// SaveOwnershipData saves file ownership and owner metrics
func (backend *SQLiteBackend) SaveOwnershipData(snapshotID int64, fileOwnership map[string][]string, ownerMetrics []OwnerMetric, analyzedAt time.Time) error {
	// Insert file ownership
	stmt, err := backend.database.Prepare(`
		INSERT INTO file_ownership (snapshot_id, file_path, owner, analyzed_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for filePath, owners := range fileOwnership {
		for _, owner := range owners {
			_, err := stmt.Exec(snapshotID, filePath, owner, analyzedAt)
			if err != nil {
				return err
			}
		}
	}

	// Insert owner metrics
	stmt, err = backend.database.Prepare(`
		INSERT INTO owner_metrics (
			snapshot_id, owner, file_count, function_count, total_lines,
			avg_cyclomatic_complexity, avg_cognitive_complexity, avg_maintainability_index,
			hotspot_count, high_complexity_function_count, overall_health_score, analyzed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, metric := range ownerMetrics {
		_, err := stmt.Exec(
			snapshotID, metric.Owner, metric.FileCount, metric.FunctionCount, metric.TotalLines,
			metric.AvgCyclomaticComplexity, metric.AvgCognitiveComplexity, metric.AvgMaintainabilityIndex,
			metric.HotspotCount, metric.HighComplexityFunctionCount, metric.OverallHealthScore, analyzedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetOwnerMetrics retrieves owner metrics for a snapshot
func (backend *SQLiteBackend) GetOwnerMetrics(snapshotID int64) ([]OwnerMetric, error) {
	query := `
		SELECT owner, file_count, function_count, total_lines,
		       avg_cyclomatic_complexity, avg_cognitive_complexity, avg_maintainability_index,
		       hotspot_count, high_complexity_function_count, overall_health_score
		FROM owner_metrics
		WHERE snapshot_id = ?
		ORDER BY overall_health_score DESC
	`

	rows, err := backend.database.Query(query, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []OwnerMetric
	for rows.Next() {
		m := OwnerMetric{}
		err := rows.Scan(
			&m.Owner, &m.FileCount, &m.FunctionCount, &m.TotalLines,
			&m.AvgCyclomaticComplexity, &m.AvgCognitiveComplexity, &m.AvgMaintainabilityIndex,
			&m.HotspotCount, &m.HighComplexityFunctionCount, &m.OverallHealthScore,
		)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// GetFileOwnership retrieves ownership map for a snapshot
func (backend *SQLiteBackend) GetFileOwnership(snapshotID int64) (map[string][]string, error) {
	query := `
		SELECT file_path, owner
		FROM file_ownership
		WHERE snapshot_id = ?
		ORDER BY file_path, owner
	`

	rows, err := backend.database.Query(query, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ownership := make(map[string][]string)
	for rows.Next() {
		var filePath, owner string
		err := rows.Scan(&filePath, &owner)
		if err != nil {
			return nil, err
		}
		ownership[filePath] = append(ownership[filePath], owner)
	}

	return ownership, rows.Err()
}
