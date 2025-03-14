// Package util provides shared utility functions used across the codebase.
package util

import (
	"path/filepath"
	"strings"
)

// IsExcluded checks if a path matches any exclude pattern
// It can check against both the full path and just the name (filename) portion
// Parameters:
// - path: Full path to check
// - name: Optional filename to check separately (can be empty)
// - exclude: List of glob patterns to match against
func IsExcluded(path, name string, exclude []string) bool {
	// Normalize path separators for cross-platform consistency
	normalizedPath := filepath.ToSlash(path)

	// Check only if we have exclusion patterns
	if len(exclude) == 0 {
		return false
	}

	// Check against each pattern
	for _, pattern := range exclude {
		// Match against full path
		if matched, _ := filepath.Match(pattern, normalizedPath); matched {
			return true
		}

		// Match against name if provided
		if name != "" {
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
		}

		// Match against directory components in the path
		// This handles cases where the exclude pattern is just a directory name
		// like "node_modules" that could appear anywhere in the path
		parts := strings.Split(normalizedPath, "/")
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}

	return false
}
