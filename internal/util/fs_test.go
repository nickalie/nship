package util

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		filename    string
		exclude     []string
		expected    bool
		description string
	}{
		{
			name:        "no exclude patterns",
			path:        "dir/file.txt",
			filename:    "file.txt",
			exclude:     []string{},
			expected:    false,
			description: "With no exclude patterns, nothing should be excluded",
		},
		{
			name:        "match filename",
			path:        "dir/file.txt",
			filename:    "file.txt",
			exclude:     []string{"*.txt"},
			expected:    true,
			description: "Should exclude if the filename matches the pattern",
		},
		{
			name:        "match full path",
			path:        "dir/sub/file.txt",
			filename:    "file.txt",
			exclude:     []string{"dir/sub/*"},
			expected:    true,
			description: "Should exclude if the full path matches the pattern",
		},
		{
			name:        "match directory name",
			path:        "dir/node_modules/file.js",
			filename:    "file.js",
			exclude:     []string{"node_modules"},
			expected:    true,
			description: "Should exclude if a directory name matches exactly",
		},
		{
			name:        "no match",
			path:        "dir/file.txt",
			filename:    "file.txt",
			exclude:     []string{"*.log", "*.tmp"},
			expected:    false,
			description: "Should not exclude if no patterns match",
		},
		{
			name:        "empty filename",
			path:        "dir/file.txt",
			filename:    "",
			exclude:     []string{"*.txt"},
			expected:    true,
			description: "Should still match on path even with empty filename",
		},
		{
			name:        "cross-platform path separators",
			path:        filepath.Join("dir", "sub", "file.txt"), // Uses OS-specific separator
			filename:    "file.txt",
			exclude:     []string{"dir/sub/*"}, // Forward slash pattern
			expected:    true,
			description: "Should handle different path separators correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExcluded(tt.path, tt.exclude)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestIsExcluded_PlatformSpecific(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows-specific path test
		excluded := IsExcluded(`C:\Users\test\file.txt`, []string{"C:/Users/*/file.txt"})
		assert.True(t, excluded, "Should match Windows path with forward-slash pattern")
	} else {
		// Unix-specific path test
		excluded := IsExcluded("/home/user/file.txt", []string{"/home/*/file.txt"})
		assert.True(t, excluded, "Should match Unix path with exact pattern")
	}
}

// TestIsExcluded_GlobPatterns tests the glob pattern matching capabilities
func TestIsExcluded_GlobPatterns(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		patterns    []string
		shouldMatch bool
		description string
	}{
		{
			name:        "double-star idea directory pattern - deep path",
			path:        "foo/.idea/bar/bla",
			patterns:    []string{"**/.idea/**"},
			shouldMatch: true,
			description: "Should match path with .idea directory in the middle",
		},
		{
			name:        "double-star idea directory pattern - root path",
			path:        ".idea/bla/foo",
			patterns:    []string{"**/.idea/**"},
			shouldMatch: true,
			description: "Should match path with .idea directory at the beginning",
		},
		{
			name:        "double-star pattern - no match",
			path:        "foo/bar/bla/something",
			patterns:    []string{"**/.idea/**"},
			shouldMatch: false,
			description: "Should not match path without .idea directory",
		},
		{
			name:        "simple wildcard pattern",
			path:        "foo/file.log",
			patterns:    []string{"*.log"},
			shouldMatch: true,
			description: "Should match file with .log extension using simple wildcard",
		},
		{
			name:        "prefix wildcard pattern",
			path:        "node_modules/package/file.js",
			patterns:    []string{"node_modules/**"},
			shouldMatch: true,
			description: "Should match any file under node_modules directory",
		},
		{
			name:        "suffix wildcard pattern",
			path:        "src/components/Button.tsx",
			patterns:    []string{"**/Button.tsx"},
			shouldMatch: true,
			description: "Should match Button.tsx file anywhere in the path",
		},
		{
			name:        "middle wildcard pattern",
			path:        "src/components/forms/Button.tsx",
			patterns:    []string{"src/**/Button.tsx"},
			shouldMatch: true,
			description: "Should match Button.tsx file in src directory with any subdirectories",
		},
		{
			name:        "cross-platform separators",
			path:        filepath.FromSlash("foo/bar/.idea/workspace.xml"),
			patterns:    []string{"**/.idea/**"},
			shouldMatch: true,
			description: "Should handle cross-platform path separators correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExcluded(tt.path, tt.patterns)
			assert.Equal(t, tt.shouldMatch, result, tt.description)
		})
	}
}
