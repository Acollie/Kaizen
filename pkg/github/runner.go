// Package github wraps the gh and git command-line tools for pull request
// review: fetching PR metadata, checking out the PR head in a worktree,
// computing diffs, and posting reviews. Every command goes through a
// CommandRunner so tests can substitute a fake.
package github

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CommandRunner executes external commands
type CommandRunner interface {
	// Run executes name with args in dir and returns stdout
	Run(ctx context.Context, dir string, name string, args ...string) (stdout []byte, err error)
	// RunWithInput is Run with input piped to the command's stdin
	RunWithInput(ctx context.Context, dir string, input []byte, name string, args ...string) (stdout []byte, err error)
}

// CommandError reports a failed command together with its stderr
type CommandError struct {
	Name   string
	Args   []string
	Stderr string
	Err    error
}

// Error formats the command, the underlying error and stderr
func (commandError *CommandError) Error() string {
	message := fmt.Sprintf("%s %s: %v", commandError.Name, strings.Join(commandError.Args, " "), commandError.Err)
	if stderr := strings.TrimSpace(commandError.Stderr); stderr != "" {
		message += ": " + stderr
	}
	return message
}

// Unwrap exposes the underlying error (e.g. exec.ErrNotFound)
func (commandError *CommandError) Unwrap() error {
	return commandError.Err
}

// ExecRunner runs commands with os/exec
type ExecRunner struct{}

// Run executes a command and returns stdout; failures include stderr
func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return ExecRunner{}.RunWithInput(ctx, dir, nil, name, args...)
}

// RunWithInput executes a command with stdin and returns stdout; failures include stderr
func (ExecRunner) RunWithInput(ctx context.Context, dir string, input []byte, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	if input != nil {
		command.Stdin = bytes.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return stdout.Bytes(), &CommandError{Name: name, Args: args, Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}
