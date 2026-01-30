package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// BackendConfig specifies storage backend configuration
type BackendConfig struct {
	Type           string // "sqlite" or "json"
	Path           string // Path to database or JSON file
	KeepJSONBackup bool   // Also save JSON alongside database
}

// NewBackend creates a storage backend based on configuration
// If a kaizen.db exists at the default location, it will be used
// Otherwise, a new database will be created
func NewBackend(config BackendConfig) (StorageBackend, error) {
	switch config.Type {
	case "sqlite", "":
		return NewSQLiteBackend(config.Path)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", config.Type)
	}
}

// DetectOrCreateDatabase checks if kaizen.db exists in the given directory
// If it exists, returns the path; otherwise creates it in a .kaizen subdirectory
func DetectOrCreateDatabase(rootPath string) (string, error) {
	// Check for kaizen.db in root
	rootDBPath := filepath.Join(rootPath, "kaizen.db")
	if _, err := os.Stat(rootDBPath); err == nil {
		return rootDBPath, nil
	}

	// Use .kaizen subdirectory
	kaizenDir := filepath.Join(rootPath, ".kaizen")
	err := os.MkdirAll(kaizenDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create .kaizen directory: %w", err)
	}

	return filepath.Join(kaizenDir, "kaizen.db"), nil
}

// DefaultBackendConfig returns the default storage configuration
func DefaultBackendConfig(rootPath string) (BackendConfig, error) {
	dbPath, err := DetectOrCreateDatabase(rootPath)
	if err != nil {
		return BackendConfig{}, err
	}

	return BackendConfig{
		Type:           "sqlite",
		Path:           dbPath,
		KeepJSONBackup: true,
	}, nil
}
