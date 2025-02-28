package fs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSFTPClient implements SFTPClient for testing
type MockSFTPClient struct {
	CreateFunc   func(path string) (io.WriteCloser, error)
	MkdirAllFunc func(path string) error
	ChmodFunc    func(path string, mode os.FileMode) error
	StatFunc     func(path string) (os.FileInfo, error)
}

// Create implements SFTPClient.Create
func (m *MockSFTPClient) Create(path string) (io.WriteCloser, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(path)
	}
	return &MockWriteCloser{}, nil
}

// MkdirAll implements SFTPClient.MkdirAll
func (m *MockSFTPClient) MkdirAll(path string) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path)
	}
	return nil
}

// Chmod implements SFTPClient.Chmod
func (m *MockSFTPClient) Chmod(path string, mode os.FileMode) error {
	if m.ChmodFunc != nil {
		return m.ChmodFunc(path, mode)
	}
	return nil
}

// Stat implements SFTPClient.Stat
func (m *MockSFTPClient) Stat(path string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(path)
	}
	return &MockFileInfo{}, nil
}

// Update MockFileSystem for testing with needed methods
func setupMockFileSystem(content []byte, isDir bool) *MockFileSystem {
	return &MockFileSystem{
		StatFunc: func(name string) (os.FileInfo, error) {
			return &MockFileInfo{
				SizeFunc: func() int64 {
					return int64(len(content))
				},
				IsDirFunc: func() bool {
					return isDir
				},
			}, nil
		},
		OpenFunc: func(name string) (io.ReadCloser, error) {
			return &MockReadCloser{
				ReadFunc: func(p []byte) (n int, err error) {
					copy(p, content)
					return len(content), io.EOF
				},
			}, nil
		},
		ReadDirFunc: func(name string) ([]os.DirEntry, error) {
			// Return directory entries based on the testing needs
			if isDir {
				return []os.DirEntry{
					&MockDirEntry{
						NameFunc:  func() string { return "file1.txt" },
						IsDirFunc: func() bool { return false },
					},
				}, nil
			}
			return nil, errors.New("not a directory")
		},
		ReadFileFunc: func(name string) ([]byte, error) {
			return content, nil
		},
	}
}

func TestCopyFile(t *testing.T) {
	mockContent := []byte("test file content")
	fileCreated := false
	fileWritten := false

	mockFS := setupMockFileSystem(mockContent, false)

	mockSFTP := &MockSFTPClient{
		CreateFunc: func(path string) (io.WriteCloser, error) {
			fileCreated = true
			return &MockWriteCloser{
				WriteFunc: func(p []byte) (n int, err error) {
					if bytes.Equal(p, mockContent) {
						fileWritten = true
					}
					return len(p), nil
				},
			}, nil
		},
		MkdirAllFunc: func(path string) error {
			return nil
		},
		ChmodFunc: func(path string, mode os.FileMode) error {
			return nil
		},
	}

	copier := NewCopier(mockFS, mockSFTP)
	err := copier.CopyFile("src/file.txt", "dst/file.txt")

	assert.NoError(t, err, "CopyFile should not return an error")
	assert.True(t, fileCreated, "Destination file was not created")
	assert.True(t, fileWritten, "Content was not correctly written to destination")
}

func TestCopyDir(t *testing.T) {
	mockFS := &MockFileSystem{
		StatFunc: func(name string) (os.FileInfo, error) {
			name = filepath.ToSlash(name)
			if name == "src" || name == "src/subdir" {
				return &MockFileInfo{
					IsDirFunc: func() bool {
						return true
					},
				}, nil
			}
			return &MockFileInfo{
				IsDirFunc: func() bool {
					return false
				},
			}, nil
		},
		ReadDirFunc: func(name string) ([]os.DirEntry, error) {
			name = filepath.ToSlash(name)
			switch name {
			case "src":
				return []os.DirEntry{
					&MockDirEntry{
						NameFunc:  func() string { return "file1.txt" },
						IsDirFunc: func() bool { return false },
					},
					&MockDirEntry{
						NameFunc:  func() string { return "subdir" },
						IsDirFunc: func() bool { return true },
					},
				}, nil
			case "src/subdir":
				return []os.DirEntry{
					&MockDirEntry{
						NameFunc:  func() string { return "file2.txt" },
						IsDirFunc: func() bool { return false },
					},
				}, nil
			default:
				return nil, fmt.Errorf("directory not found: %s", name)
			}
		},
		OpenFunc: func(name string) (io.ReadCloser, error) {
			return &MockReadCloser{}, nil
		},
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte("test content"), nil
		},
	}

	createdDirs := make(map[string]bool)
	createdFiles := make(map[string]bool)

	mockSFTP := &MockSFTPClient{
		MkdirAllFunc: func(path string) error {
			createdDirs[path] = true
			return nil
		},
		CreateFunc: func(path string) (io.WriteCloser, error) {
			createdFiles[path] = true
			return &MockWriteCloser{}, nil
		},
		StatFunc: func(path string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
	}

	copier := NewCopier(mockFS, mockSFTP)
	err := copier.CopyDir("src", "dst", nil)

	assert.NoError(t, err, "CopyDir should not return an error")

	// Check directories were created
	expectedDirs := []string{"dst", "dst/subdir"}
	for _, dir := range expectedDirs {
		assert.True(t, createdDirs[dir], "Directory not created: %s", dir)
	}

	// Check files were created
	expectedFiles := []string{"dst/file1.txt", "dst/subdir/file2.txt"}
	for _, file := range expectedFiles {
		assert.True(t, createdFiles[file], "File not created: %s", file)
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		exclude  []string
		expected bool
	}{
		{
			name:     "no exclusions",
			path:     "file.txt",
			exclude:  nil,
			expected: false,
		},
		{
			name:     "exact match",
			path:     "node_modules",
			exclude:  []string{"node_modules"},
			expected: true,
		},
		{
			name:     "pattern match",
			path:     "file.tmp",
			exclude:  []string{"*.tmp"},
			expected: true,
		},
		{
			name:     "no match",
			path:     "important.txt",
			exclude:  []string{"*.tmp", "*.log", "node_modules"},
			expected: false,
		},
		{
			name:     "multiple patterns with match",
			path:     "debug.log",
			exclude:  []string{"*.tmp", "*.log", "node_modules"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExcluded(tt.path, tt.exclude)
			assert.Equal(t, tt.expected, result, "isExcluded() returned unexpected result")
		})
	}
}

// setupTestEnvironment creates a temporary directory for testing
func setupTestEnvironment(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "fs-test")
	require.NoError(t, err, "Failed to create temp dir")

	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	return tempDir, cleanup
}
