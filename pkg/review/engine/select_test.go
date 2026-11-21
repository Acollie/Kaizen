package engine

import (
	"errors"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeLookPath(found bool) func(string) (string, error) {
	return func(file string) (string, error) {
		if found {
			return "/usr/local/bin/" + file, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
}

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestSelectAutoPrefersCLI(t *testing.T) {
	engine, err := Select("auto", SelectOptions{
		CLI:      CLIEngine{Model: "sonnet"},
		LookPath: fakeLookPath(true),
		Getenv:   fakeGetenv(map[string]string{"ANTHROPIC_API_KEY": "sk-test"}),
	})
	require.NoError(t, err)
	cli, ok := engine.(*CLIEngine)
	require.True(t, ok)
	assert.Equal(t, "/usr/local/bin/claude", cli.ClaudePath)
	assert.Equal(t, "sonnet", cli.Model)
}

func TestSelectAutoFallsBackToAPI(t *testing.T) {
	engine, err := Select("", SelectOptions{
		LookPath: fakeLookPath(false),
		Getenv:   fakeGetenv(map[string]string{"ANTHROPIC_API_KEY": "sk-test"}),
	})
	require.NoError(t, err)
	assert.Equal(t, APIEngineName, engine.Name())
}

func TestSelectAutoNothingAvailable(t *testing.T) {
	_, err := Select("auto", SelectOptions{LookPath: fakeLookPath(false), Getenv: fakeGetenv(nil)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude CLI is not on PATH")
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
	assert.Contains(t, err.Error(), "install the claude CLI")
}

func TestSelectExplicit(t *testing.T) {
	_, err := Select("cli", SelectOptions{LookPath: fakeLookPath(false), Getenv: fakeGetenv(nil)})
	assert.ErrorContains(t, err, "not on PATH")

	engine, err := Select("CLI", SelectOptions{CLI: CLIEngine{ClaudePath: "/opt/claude"}, LookPath: fakeLookPath(false)})
	require.NoError(t, err)
	assert.Equal(t, "/opt/claude", engine.(*CLIEngine).ClaudePath)

	_, err = Select("api", SelectOptions{LookPath: fakeLookPath(true), Getenv: fakeGetenv(nil)})
	assert.ErrorContains(t, err, "ANTHROPIC_API_KEY is not set")

	engine, err = Select("api", SelectOptions{API: APIEngine{ClientOptions: []option.RequestOption{option.WithAPIKey("k")}}, Getenv: fakeGetenv(nil)})
	require.NoError(t, err)
	assert.Equal(t, APIEngineName, engine.Name())

	_, err = Select("gpt", SelectOptions{})
	assert.ErrorContains(t, err, `unknown review engine "gpt"`)
}
