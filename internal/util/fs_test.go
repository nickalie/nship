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
			result := IsExcluded(tt.path, tt.filename, tt.exclude)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestIsExcluded_PlatformSpecific(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows-specific path test
		excluded := IsExcluded(`C:\Users\test\file.txt`, "file.txt", []string{"C:/Users/*/file.txt"})
		assert.True(t, excluded, "Should match Windows path with forward-slash pattern")
	} else {
		// Unix-specific path test
		excluded := IsExcluded("/home/user/file.txt", "file.txt", []string{"/home/*/file.txt"})
		assert.True(t, excluded, "Should match Unix path with exact pattern")
	}
}
