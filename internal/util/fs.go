// Package util provides shared utility functions used across the codebase.
package util

import (
	"path/filepath"
	"strings"
)

// matchPattern checks if a single string matches a given pattern
func matchPattern(pattern, str string) bool {
	matched, _ := filepath.Match(pattern, str)
	return matched
}

// matchesFullPath checks if the full path matches the pattern
func matchesFullPath(pattern, normalizedPath string) bool {
	return matchPattern(pattern, normalizedPath)
}

// matchesFileName checks if the filename matches the pattern
func matchesFileName(pattern, name string) bool {
	return name != "" && matchPattern(pattern, name)
}

// matchesPathParts checks if any path component matches the pattern
func matchesPathParts(pattern string, parts []string) bool {
	for _, part := range parts {
		if matchPattern(pattern, part) {
			return true
		}
	}
	return false
}

// IsExcluded checks if a path matches any exclude pattern
// It can check against both the full path and just the name (filename) portion
func IsExcluded(path, name string, exclude []string) bool {
	if len(exclude) == 0 {
		return false
	}

	normalizedPath := filepath.ToSlash(path)
	parts := strings.Split(normalizedPath, "/")

	for _, pattern := range exclude {
		if matchesFullPath(pattern, normalizedPath) ||
			matchesFileName(pattern, name) ||
			matchesPathParts(pattern, parts) {
			return true
		}
	}
	return false
}
