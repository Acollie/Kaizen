package github

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

// untrustedGitEnv is added to every git command that touches the PR checkout:
// never download LFS objects the PR points at, and never prompt for credentials
// (a prompt would hang the review; on old gits credential.interactive is unknown)
var untrustedGitEnv = []string{"GIT_LFS_SKIP_SMUDGE=1", "GIT_TERMINAL_PROMPT=0"}

var (
	emptyHooksDirOnce sync.Once
	emptyHooksDir     string
)

// disabledHooksPath is a core.hooksPath that contains no hooks. On Unix it is
// /dev/null (not a directory, so git finds no hooks in it); on Windows, where
// /dev/null is not a path, it is a private empty temporary directory.
func disabledHooksPath() string {
	if runtime.GOOS != "windows" {
		return "/dev/null"
	}
	emptyHooksDirOnce.Do(func() {
		dir, err := os.MkdirTemp("", "kaizen-no-hooks-")
		if err != nil {
			emptyHooksDir = os.DevNull
			return
		}
		emptyHooksDir = dir
	})
	return emptyHooksDir
}

// hardenedGitArgs disable the user's hooks (core.hooksPath=.husky and the like) for
// commands run against the untrusted PR checkout: a post-checkout or
// reference-transaction hook would otherwise run with the PR's files in its cwd
func hardenedGitArgs() []string {
	return []string{"-c", "core.hooksPath=" + disabledHooksPath()}
}

// checkoutGit runs a git subcommand that touches the PR checkout (fetch, worktree
// add/remove, merge-base, diff) with hooks disabled, LFS smudging skipped and
// terminal prompts off
func (client *Client) checkoutGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	fullArgs := append(hardenedGitArgs(), args...)
	if envRunner, supportsEnv := client.runner().(EnvCommandRunner); supportsEnv {
		return envRunner.RunWithEnv(ctx, dir, untrustedGitEnv, "git", fullArgs...)
	}
	return client.runner().Run(ctx, dir, "git", fullArgs...)
}

// validateGitArg rejects a ref or SHA that git would parse as an option
func validateGitArg(kind, value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("refusing %s %q: it starts with '-'", kind, value)
	}
	return nil
}
