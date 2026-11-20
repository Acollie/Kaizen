package engine

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review"
)

// Engine selection names accepted by Select
const (
	SelectAuto = "auto"
	SelectCLI  = "cli"
	SelectAPI  = "api"
)

// SelectOptions configures the engine Select returns
type SelectOptions struct {
	CLI CLIEngine // settings for the CLI engine; ClaudePath is filled in from lookPath when empty
	API APIEngine // settings for the API engine

	// LookPath and Getenv default to exec.LookPath and os.Getenv; tests replace them
	LookPath func(file string) (string, error)
	Getenv   func(key string) string
}

const setupHelp = `To run an AI review, set up one of:
  - Claude Code (recommended): install the claude CLI (https://claude.com/claude-code) and log in, so "claude" is on your PATH. Uses your Claude subscription or Console account.
  - Anthropic API: export ANTHROPIC_API_KEY=<key from https://platform.claude.com/settings/keys>.`

// Select returns the engine named by name: "cli", "api", or "auto" (or empty),
// which prefers the claude CLI when it is on PATH and falls back to the API
// when ANTHROPIC_API_KEY is set
func Select(name string, options SelectOptions) (review.Engine, error) {
	lookPath := options.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	getenv := options.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", SelectAuto:
		return selectAuto(options, lookPath, getenv)
	case SelectCLI:
		return selectCLI(options.CLI, lookPath)
	case SelectAPI:
		return selectAPI(options.API, getenv)
	default:
		return nil, fmt.Errorf("unknown review engine %q: use auto, cli or api", name)
	}
}

func selectAuto(options SelectOptions, lookPath func(string) (string, error), getenv func(string) string) (review.Engine, error) {
	if engine, err := selectCLI(options.CLI, lookPath); err == nil {
		return engine, nil
	}
	if engine, err := selectAPI(options.API, getenv); err == nil {
		return engine, nil
	}
	return nil, fmt.Errorf("no AI review engine available: the claude CLI is not on PATH and ANTHROPIC_API_KEY is not set.\n%s", setupHelp)
}

func selectCLI(config CLIEngine, lookPath func(string) (string, error)) (review.Engine, error) {
	if config.ClaudePath == "" {
		path, err := lookPath("claude")
		if err != nil {
			return nil, fmt.Errorf("the claude CLI is not on PATH.\n%s", setupHelp)
		}
		config.ClaudePath = path
	}
	return &config, nil
}

func selectAPI(config APIEngine, getenv func(string) string) (review.Engine, error) {
	if strings.TrimSpace(getenv("ANTHROPIC_API_KEY")) == "" && len(config.ClientOptions) == 0 {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set.\n%s", setupHelp)
	}
	return &config, nil
}
