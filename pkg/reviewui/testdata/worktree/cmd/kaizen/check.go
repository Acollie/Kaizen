package main

import (
	"fmt"
	"os"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/check"
	"github.com/spf13/cobra"
)

var checkBase string

// runCheck analyzes the working tree and fails when changed code breaches thresholds
func runCheck(cmd *cobra.Command, args []string) error {
	settings, err := config.Load(".")
	if err != nil {
		return err
	}

	pipeline := analyzer.NewPipeline(".", settings.Ignore, nil)
	result, err := pipeline.Analyze(cmd.Context())
	if err != nil {
		return fmt.Errorf("check analysis: %w", err)
	}

	report, err := check.Evaluate(result, checkBase, settings.Thresholds)
	if err != nil {
		return err
	}
	report.Print(os.Stdout)
	if report.Failed() {
		os.Exit(1)
	}
	return nil
}
