package visualization

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBrowserTargetKeepsURLs(t *testing.T) {
	for _, url := range []string{"http://127.0.0.1:7777/?t=abc", "https://example.com"} {
		resolved, err := resolveBrowserTarget(url)
		require.NoError(t, err)
		assert.Equal(t, url, resolved)
	}
}

func TestResolveBrowserTargetMakesPathsAbsolute(t *testing.T) {
	resolved, err := resolveBrowserTarget("report.html")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(resolved))
}

func TestBrowserCommandPerPlatform(t *testing.T) {
	command, args := browserCommand("darwin", "x")
	assert.Equal(t, "open", command)
	assert.Equal(t, []string{"x"}, args)

	command, args = browserCommand("windows", "x")
	assert.Equal(t, "cmd", command)
	assert.Equal(t, []string{"/c", "start", "", "x"}, args)

	command, _ = browserCommand("linux", "x")
	assert.Equal(t, "xdg-open", command)
}
