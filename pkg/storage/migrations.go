package storage

import "database/sql"

// Migration represents a single schema migration
type migration struct {
	version int
	up      func(*sql.DB) error
}

// migrateV1 creates the initial schema
func migrateV1(database *sql.DB) error {
	schema := `
	-- analysis_snapshots: Full analysis records with metadata
	CREATE TABLE IF NOT EXISTS analysis_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		analyzed_at TIMESTAMP NOT NULL,
		git_commit_hash TEXT,
		git_branch TEXT,
		kaizen_version TEXT,
		config_hash TEXT,

		-- Denormalized summary for fast queries
		total_files INTEGER,
		total_functions INTEGER,
		total_lines INTEGER,
		total_code_lines INTEGER,
		avg_cyclomatic_complexity REAL,
		avg_cognitive_complexity REAL,
		avg_function_length REAL,
		avg_maintainability_index REAL,
		hotspot_count INTEGER,

		-- Score report
		overall_grade TEXT,
		overall_score REAL,
		complexity_score REAL,
		maintainability_score REAL,
		churn_score REAL,
		has_churn_data BOOLEAN,

		-- Full JSON blob (complete data preservation)
		full_data TEXT NOT NULL,

		UNIQUE(analyzed_at)
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_date ON analysis_snapshots(analyzed_at DESC);
	CREATE INDEX IF NOT EXISTS idx_snapshots_grade ON analysis_snapshots(overall_grade);

	-- metrics_timeseries: Denormalized for efficient trending
	CREATE TABLE IF NOT EXISTS metrics_timeseries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapshot_id INTEGER NOT NULL,
		analyzed_at TIMESTAMP NOT NULL,

		metric_name TEXT NOT NULL,
		scope TEXT NOT NULL,
		scope_path TEXT,
		value REAL NOT NULL,

		FOREIGN KEY (snapshot_id) REFERENCES analysis_snapshots(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics_timeseries(metric_name, analyzed_at DESC);
	CREATE INDEX IF NOT EXISTS idx_metrics_path ON metrics_timeseries(scope_path, metric_name, analyzed_at DESC);

	-- function_history: Track individual function evolution
	CREATE TABLE IF NOT EXISTS function_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapshot_id INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		function_name TEXT NOT NULL,

		length INTEGER,
		cyclomatic_complexity INTEGER,
		cognitive_complexity INTEGER,
		maintainability_index REAL,
		total_commits INTEGER,
		is_hotspot BOOLEAN,

		analyzed_at TIMESTAMP NOT NULL,

		FOREIGN KEY (snapshot_id) REFERENCES analysis_snapshots(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_function_file_name ON function_history(file_path, function_name, analyzed_at DESC);
	CREATE INDEX IF NOT EXISTS idx_function_hotspot ON function_history(is_hotspot, analyzed_at DESC);

	-- file_ownership: Maps files to owners based on CODEOWNERS
	CREATE TABLE IF NOT EXISTS file_ownership (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapshot_id INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		owner TEXT NOT NULL,
		pattern TEXT,

		analyzed_at TIMESTAMP NOT NULL,

		FOREIGN KEY (snapshot_id) REFERENCES analysis_snapshots(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_file_ownership_path ON file_ownership(file_path, owner);
	CREATE INDEX IF NOT EXISTS idx_file_ownership_owner ON file_ownership(owner, analyzed_at DESC);

	-- owner_metrics: Aggregated metrics by code owner
	CREATE TABLE IF NOT EXISTS owner_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapshot_id INTEGER NOT NULL,
		owner TEXT NOT NULL,

		file_count INTEGER,
		function_count INTEGER,
		total_lines INTEGER,
		avg_cyclomatic_complexity REAL,
		avg_cognitive_complexity REAL,
		avg_maintainability_index REAL,
		hotspot_count INTEGER,
		high_complexity_function_count INTEGER,
		overall_health_score REAL,

		analyzed_at TIMESTAMP NOT NULL,

		FOREIGN KEY (snapshot_id) REFERENCES analysis_snapshots(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_owner_metrics_owner ON owner_metrics(owner, analyzed_at DESC);
	CREATE INDEX IF NOT EXISTS idx_owner_metrics_score ON owner_metrics(overall_health_score DESC);

	-- schema_version: Tracks migration state
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	// Execute schema
	_, err := database.Exec(schema)
	return err
}

// runMigrations applies all pending migrations
func runMigrations(database *sql.DB) error {
	migrations := []migration{
		{version: 1, up: migrateV1},
	}

	// Get current schema version
	currentVersion := 0
	row := database.QueryRow("SELECT MAX(version) FROM schema_version")
	_ = row.Scan(&currentVersion) // Ignore error if table doesn't exist

	// Apply pending migrations
	for _, mig := range migrations {
		if mig.version > currentVersion {
			if err := mig.up(database); err != nil {
				return err
			}

			// Record migration
			_, err := database.Exec("INSERT OR IGNORE INTO schema_version (version) VALUES (?)", mig.version)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
