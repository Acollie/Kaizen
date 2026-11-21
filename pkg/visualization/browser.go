package visualization

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OpenInBrowser opens a local file path or an http(s) URL in the default browser
func OpenInBrowser(target string) error {
	resolved, err := resolveBrowserTarget(target)
	if err != nil {
		return err
	}

	command, args := browserCommand(runtime.GOOS, resolved)
	return exec.Command(command, args...).Start()
}

// resolveBrowserTarget leaves URLs untouched and converts file paths to absolute paths
func resolveBrowserTarget(target string) (string, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target, nil
	}
	return filepath.Abs(target)
}

// browserCommand returns the platform-specific command used to open a target
func browserCommand(goos, target string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{target}
	case "windows":
		// The empty title argument stops `start` treating a quoted target as the window title
		return "cmd", []string{"/c", "start", "", target}
	default:
		return "xdg-open", []string{target}
	}
}
