// Package util provides shared utility functions used across the codebase.
package util

import (
	"path/filepath"
	"strings"
)

// IsExcluded checks if a path matches any exclude pattern
// It can check against both the full path and just the name (filename) portion
// Supports glob patterns:
// - * matches any number of characters within a path segment
// - ** matches any number of characters across path segments
// Examples:
// - "**/.idea/**" matches both "foo/.idea/bar/bla" and ".idea/bla/foo"
// - "*.log" matches "file.log" but not "file.log.txt"
func IsExcluded(path string, exclude []string) bool {
	// Normalize path separators to forward slash for consistency
	normalizedPath := filepath.ToSlash(path)

	for _, pattern := range exclude {
		// Normalize pattern separators
		normalizedPattern := filepath.ToSlash(pattern)

		// If pattern has no wildcards, use the original simple contains check
		if !strings.Contains(normalizedPattern, "*") {
			if strings.Contains(normalizedPath, normalizedPattern) {
				return true
			}
			continue
		}

		// Handle ** pattern matching
		if matched := matchGlobPattern(normalizedPath, normalizedPattern); matched {
			return true
		}
	}

	return false
}

// matchGlobPattern implements glob pattern matching with support for ** wildcards
func matchGlobPattern(path, pattern string) bool {
	// Fast path: exact match
	if pattern == path {
		return true
	}

	// Handle specific case for "**/.idea/**" pattern which is commonly used
	if pattern == "**/.idea/**" && strings.Contains(path, ".idea/") {
		return true
	}

	// Handle patterns with "**"
	if strings.Contains(pattern, "**") {
		// Special case for patterns like "**/.idea/**"
		if strings.HasPrefix(pattern, "**") && strings.HasSuffix(pattern, "**") {
			// Extract the middle part between the ** wildcards
			middle := pattern[2 : len(pattern)-2]
			// If the middle part is found in the path, it's a match
			if strings.Contains(path, middle) {
				return true
			}
		}

		// Handle pattern with ** prefix (e.g., "**/foo.txt")
		if strings.HasPrefix(pattern, "**") {
			patternSuffix := pattern[2:]
			if strings.HasSuffix(path, patternSuffix) {
				return true
			}

			// Also check if it's contained anywhere in the path
			// This is important for patterns like "**/.idea/"
			return strings.Contains(path, patternSuffix)
		}

		// Handle pattern with ** suffix (e.g., "foo/**")
		if strings.HasSuffix(pattern, "**") {
			patternPrefix := pattern[:len(pattern)-2]
			// Check if path starts with the pattern prefix
			return strings.HasPrefix(path, patternPrefix)
		}

		// Handle pattern with ** in the middle (e.g., "foo/**/bar")
		parts := strings.Split(pattern, "**")
		if len(parts) >= 2 {
			if strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[len(parts)-1]) {
				return true
			}
		}
	}

	// Fall back to filepath.Match for simple * patterns
	matched, _ := filepath.Match(pattern, path)
	return matched
}
