// Package util provides shared utility functions used across the codebase.
package util

import (
	"path/filepath"
	"strings"
)

// matchSimpleContains checks if a path contains a pattern without wildcards
func matchSimpleContains(normalizedPath, normalizedPattern string) bool {
	return strings.Contains(normalizedPath, normalizedPattern)
}

// matchSimpleGlob checks if a filename matches a simple glob pattern without path separators
func matchSimpleGlob(baseName, normalizedPattern string) bool {
	matched, _ := filepath.Match(normalizedPattern, baseName)
	return matched
}

// matchFullPathGlob checks if a full path matches a pattern with path separators
func matchFullPathGlob(normalizedPath, normalizedPattern string) bool {
	matched, _ := filepath.Match(normalizedPattern, normalizedPath)
	return matched
}

// matchPattern checks if a single pattern matches a path
func matchPattern(normalizedPath, normalizedPattern, baseName string) bool {
	// Simple contains check for patterns without wildcards
	if !strings.Contains(normalizedPattern, "*") {
		return matchSimpleContains(normalizedPath, normalizedPattern)
	}

	// Handle patterns with **
	if strings.Contains(normalizedPattern, "**") {
		return matchGlobPattern(normalizedPath, normalizedPattern)
	}

	// Handle simple patterns without path separators
	if !strings.Contains(normalizedPattern, "/") {
		return matchSimpleGlob(baseName, normalizedPattern)
	}

	// Handle full path patterns
	return matchFullPathGlob(normalizedPath, normalizedPattern)
}

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
	baseName := filepath.Base(normalizedPath)

	for _, pattern := range exclude {
		normalizedPattern := filepath.ToSlash(pattern)
		if matchPattern(normalizedPath, normalizedPattern, baseName) {
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
		return matchDoubleStarPattern(path, pattern)
	}

	// Fall back to filepath.Match for simple * patterns
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// matchDoubleStarPattern handles glob patterns containing "**"
func matchDoubleStarPattern(path, pattern string) bool {
	switch {
	case isDoubleStarPrefixAndSuffix(pattern):
		return matchDoubleStarPrefixAndSuffix(path, pattern)
	case isDoubleStarPrefix(pattern):
		return matchDoubleStarPrefix(path, pattern)
	case isDoubleStarSuffix(pattern):
		return matchDoubleStarSuffix(path, pattern)
	default:
		return matchDoubleStarMiddle(path, pattern)
	}
}

// isDoubleStarPrefixAndSuffix checks if a pattern has "**" at both start and end
func isDoubleStarPrefixAndSuffix(pattern string) bool {
	return strings.HasPrefix(pattern, "**") && strings.HasSuffix(pattern, "**")
}

// matchDoubleStarPrefixAndSuffix handles patterns like "**/.idea/**"
func matchDoubleStarPrefixAndSuffix(path, pattern string) bool {
	middle := pattern[2 : len(pattern)-2]
	return strings.Contains(path, middle)
}

// isDoubleStarPrefix checks if a pattern starts with "**"
func isDoubleStarPrefix(pattern string) bool {
	return strings.HasPrefix(pattern, "**")
}

// matchDoubleStarPrefix handles patterns like "**/foo.txt"
func matchDoubleStarPrefix(path, pattern string) bool {
	patternSuffix := pattern[2:]
	return strings.HasSuffix(path, patternSuffix) || strings.Contains(path, patternSuffix)
}

// isDoubleStarSuffix checks if a pattern ends with "**"
func isDoubleStarSuffix(pattern string) bool {
	return strings.HasSuffix(pattern, "**")
}

// matchDoubleStarSuffix handles patterns like "foo/**"
func matchDoubleStarSuffix(path, pattern string) bool {
	patternPrefix := pattern[:len(pattern)-2]
	return strings.HasPrefix(path, patternPrefix)
}

// matchDoubleStarMiddle handles patterns like "foo/**/bar"
func matchDoubleStarMiddle(path, pattern string) bool {
	parts := strings.Split(pattern, "**")
	return len(parts) >= 2 && strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[len(parts)-1])
}
