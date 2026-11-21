// Package config loads .kaizen.yaml and .kaizenignore settings.
package config

import (
	"path"
	"strings"
)

// IgnoreMatcher applies gitignore-style patterns to repo-relative paths
type IgnoreMatcher struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	glob     string
	negate   bool
	dirOnly  bool
	anchored bool
}

// Match reports whether relativePath is excluded; later patterns override earlier ones
func (matcher *IgnoreMatcher) Match(relativePath string, isDir bool) bool {
	if matcher == nil {
		return false
	}
	relativePath = strings.TrimPrefix(path.Clean("/"+relativePath), "/")
	ignored := false
	for _, pattern := range matcher.patterns {
		if pattern.dirOnly && !isDir {
			continue
		}
		candidate := relativePath
		if !pattern.anchored {
			candidate = path.Base(relativePath)
		}
		if matched, _ := path.Match(pattern.glob, candidate); matched {
			ignored = !pattern.negate
		}
	}
	return ignored
}
