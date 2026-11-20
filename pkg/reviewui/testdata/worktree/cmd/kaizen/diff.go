package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/alexcollie/kaizen/pkg/churn"
	"github.com/spf13/cobra"
)

var diffPath string

// runDiff compares the latest snapshot with a fresh analysis and prints churn deltas
func runDiff(cmd *cobra.Command, args []string) error {
	snapshot, err := loadLatestSnapshot(diffPath)
	if err != nil {
		return err
	}

	churnAnalyzer := churn.NewAnalyzer(diffPath, snapshot.CreatedAt)
	for _, file := range snapshot.ChangedFiles() {
		metric, err := churnAnalyzer.GetFileChurn(cmd.Context(), filepath.Join(diffPath, file))
		if err != nil {
			return fmt.Errorf("churn for %s: %w", file, err)
		}
		if metric.TotalCommits == 0 {
			continue
		}
		fmt.Printf("%-50s %3d commits since %s\n", file, metric.TotalCommits,
			snapshot.CreatedAt.Format(time.DateOnly))
	}
	return nil
}
