package github

import (
	"context"
	"strings"
	"sync"
)

// recordedCall is one command the fake runner received
type recordedCall struct {
	Dir   string
	Name  string
	Args  []string
	Input []byte
}

// commandLine renders "name arg1 arg2"
func (call recordedCall) commandLine() string {
	return strings.TrimSpace(call.Name + " " + strings.Join(call.Args, " "))
}

// cannedResponse is what the fake returns for commands starting with a prefix
type cannedResponse struct {
	prefix string
	stdout string
	err    error
}

// fakeRunner records calls and answers them from canned responses, matched by
// command-line prefix in registration order. Unmatched commands succeed with no output.
type fakeRunner struct {
	mutex     sync.Mutex
	calls     []recordedCall
	responses []cannedResponse
}

func (runner *fakeRunner) respond(prefix, stdout string, err error) *fakeRunner {
	runner.responses = append(runner.responses, cannedResponse{prefix: prefix, stdout: stdout, err: err})
	return runner
}

func (runner *fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return runner.RunWithInput(ctx, dir, nil, name, args...)
}

func (runner *fakeRunner) RunWithInput(_ context.Context, dir string, input []byte, name string, args ...string) ([]byte, error) {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()

	call := recordedCall{Dir: dir, Name: name, Args: args, Input: input}
	runner.calls = append(runner.calls, call)
	for _, response := range runner.responses {
		if strings.HasPrefix(call.commandLine(), response.prefix) {
			return []byte(response.stdout), response.err
		}
	}
	return nil, nil
}

func (runner *fakeRunner) commandLines() []string {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	lines := make([]string, 0, len(runner.calls))
	for _, call := range runner.calls {
		lines = append(lines, call.commandLine())
	}
	return lines
}

func (runner *fakeRunner) lastCall() recordedCall {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	return runner.calls[len(runner.calls)-1]
}
