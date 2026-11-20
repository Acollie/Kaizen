package main

import (
	"fmt"
	"time"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/churn"
	"github.com/spf13/cobra"
)

var (
	analyzePath      string
	analyzeSince     string
	analyzeSkipChurn bool
	analyzeOutput    string
)

// runAnalyze executes the analyze command
func runAnalyze(cmd *cobra.Command, args []string) error {
	settings, err := config.Load(analyzePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var churnAnalyzer *churn.Analyzer
	if !analyzeSkipChurn && !settings.Analysis.SkipChurn {
		since, err := time.ParseDuration(analyzeSince)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}
		churnAnalyzer = churn.NewAnalyzer(analyzePath, time.Now().Add(-since))
	}

	pipeline := analyzer.NewPipeline(analyzePath, settings.Ignore, churnAnalyzer)
	result, err := pipeline.Analyze(cmd.Context())
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	if err := writeResults(result, analyzeOutput); err != nil {
		return err
	}
	fmt.Printf("Analyzed %d files -> %s\n", len(result.Files), analyzeOutput)
	return nil
}
