package fs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nickalie/nship/internal/util"
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

func TestCopyFile(t *testing.T) {
	// Create temporary test directory
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create source file with test content
	mockContent := []byte("test file content")
	sourceFile := filepath.Join(tempDir, "file.txt")
	err := os.WriteFile(sourceFile, mockContent, 0644)
	require.NoError(t, err, "Failed to create test source file")

	fileCreated := false
	fileWritten := false

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

	copier := NewCopier(mockSFTP)
	err = copier.CopyFile(sourceFile, "remote/file.txt")

	assert.NoError(t, err, "CopyFile should not return an error")
	assert.True(t, fileCreated, "Destination file was not created")
	assert.True(t, fileWritten, "Content was not correctly written to destination")
}

func TestCopyDir(t *testing.T) {
	// Create temporary test directory
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test directory structure
	sourceDir := filepath.Join(tempDir, "local")
	subdirPath := filepath.Join(sourceDir, "subdir")
	require.NoError(t, os.MkdirAll(subdirPath, 0755), "Failed to create test subdirectory")

	// Create test files
	file1Path := filepath.Join(sourceDir, "file1.txt")
	file2Path := filepath.Join(subdirPath, "file2.txt")
	require.NoError(t, os.WriteFile(file1Path, []byte("file1 content"), 0644), "Failed to create file1.txt")
	require.NoError(t, os.WriteFile(file2Path, []byte("file2 content"), 0644), "Failed to create file2.txt")

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

	copier := NewCopier(mockSFTP)
	err := copier.CopyDir(sourceDir, "remote", nil)

	assert.NoError(t, err, "CopyDir should not return an error")

	// Check directories were created
	expectedDirs := []string{"remote", "remote/subdir"}
	for _, dir := range expectedDirs {
		assert.True(t, createdDirs[dir], "Directory not created: %s", dir)
	}

	// Check files were created
	expectedFiles := []string{"remote/file1.txt", "remote/subdir/file2.txt"}
	for _, file := range expectedFiles {
		assert.True(t, createdFiles[file], "File not created: %s", file)
	}
}

func TestExcludePatterns(t *testing.T) {
	// Renamed test to avoid duplicate TestIsExcluded
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
			// Use util.IsExcluded instead of isExcluded
			result := util.IsExcluded(tt.path, tt.exclude)
			assert.Equal(t, tt.expected, result, "IsExcluded() returned unexpected result")
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

func TestCopyPath(t *testing.T) {
	// Create temporary test directory
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test files and directories
	localDir := filepath.Join(tempDir, "local")
	require.NoError(t, os.MkdirAll(localDir, 0755), "Failed to create test local directory")

	// Create a test file
	testFilePath := filepath.Join(localDir, "file.txt")
	require.NoError(t, os.WriteFile(testFilePath, []byte("test content"), 0644), "Failed to create test file")

	tests := []struct {
		name        string
		local       string
		remote      string
		exclude     []string
		isSourceDir bool
		setupMock   func() *MockSFTPClient
		expectErr   bool
	}{
		{
			name:        "copy file",
			local:       filepath.Join(tempDir, "local/file.txt"),
			remote:      "remote/file.txt",
			exclude:     nil,
			isSourceDir: false,
			setupMock: func() *MockSFTPClient {
				mockSFTP := &MockSFTPClient{
					CreateFunc: func(path string) (io.WriteCloser, error) {
						return &MockWriteCloser{}, nil
					},
					StatFunc: func(path string) (os.FileInfo, error) {
						return nil, os.ErrNotExist
					},
				}
				return mockSFTP
			},
			expectErr: false,
		},
		{
			name:        "copy directory",
			local:       filepath.Join(tempDir, "local"),
			remote:      "remote",
			exclude:     nil,
			isSourceDir: true,
			setupMock: func() *MockSFTPClient {
				mockSFTP := &MockSFTPClient{
					MkdirAllFunc: func(path string) error {
						return nil
					},
				}
				return mockSFTP
			},
			expectErr: false,
		},
		{
			name:        "source not found",
			local:       filepath.Join(tempDir, "nonexistent"),
			remote:      "remote",
			exclude:     nil,
			isSourceDir: false,
			setupMock: func() *MockSFTPClient {
				mockSFTP := &MockSFTPClient{}
				return mockSFTP
			},
			expectErr: true,
		},
		{
			name:        "copy directory with exclusions",
			local:       filepath.Join(tempDir, "local"),
			remote:      "remote",
			exclude:     []string{"*.log", "node_modules"},
			isSourceDir: true,
			setupMock: func() *MockSFTPClient {
				createdFiles := make(map[string]bool)
				mockSFTP := &MockSFTPClient{
					MkdirAllFunc: func(path string) error {
						return nil
					},
					CreateFunc: func(path string) (io.WriteCloser, error) {
						createdFiles[path] = true
						return &MockWriteCloser{
							WriteFunc: func(p []byte) (n int, err error) {
								return len(p), nil
							},
							CloseFunc: func() error {
								return nil
							},
						}, nil
					},
					StatFunc: func(path string) (os.FileInfo, error) {
						// Always return not exist to ensure files are copied
						return nil, os.ErrNotExist
					},
				}
				return mockSFTP
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSFTP := tt.setupMock()
			copier := NewCopier(mockSFTP)

			err := copier.CopyPath(tt.local, tt.remote, tt.exclude)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestShouldTransferFile(t *testing.T) {
	// Create temporary test directory
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name       string
		localSize  int64
		remoteSize int64
		remoteErr  error
		expected   bool
		expectErr  bool
	}{
		{
			name:       "different sizes should transfer",
			localSize:  100,
			remoteSize: 50,
			expected:   true,
			expectErr:  false,
		},
		{
			name:       "same sizes should not transfer",
			localSize:  100,
			remoteSize: 100,
			expected:   false,
			expectErr:  false,
		},
		{
			name:      "remote file not found should transfer",
			localSize: 100,
			remoteErr: os.ErrNotExist,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "remote stat error should return error",
			localSize: 100,
			remoteErr: fmt.Errorf("remote stat error"),
			expected:  false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a local test file with the specified size
			localPath := filepath.Join(tempDir, "testfile")
			err := os.WriteFile(localPath, make([]byte, tt.localSize), 0644)
			require.NoError(t, err, "Failed to create test file")

			mockSFTP := &MockSFTPClient{
				StatFunc: func(path string) (os.FileInfo, error) {
					if tt.remoteErr != nil {
						return nil, tt.remoteErr
					}
					return &MockFileInfo{
						SizeFunc: func() int64 {
							return tt.remoteSize
						},
					}, nil
				},
			}

			copier := NewCopier(mockSFTP)
			result, err := copier.shouldTransferFile(localPath, "remote/path")

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestProcessEntry(t *testing.T) {
	// Create temporary test directory
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a local directory for testing
	localDir := filepath.Join(tempDir, "local")
	require.NoError(t, os.MkdirAll(localDir, 0755))

	t.Run("excluded file should be skipped", func(t *testing.T) {
		// Create the test.log file
		testLogFile := filepath.Join(localDir, "test.log")
		require.NoError(t, os.WriteFile(testLogFile, []byte("test content"), 0644))

		mockSFTP := &MockSFTPClient{}

		copier := NewCopier(mockSFTP)
		entry := &MockDirEntry{
			NameFunc:  func() string { return "test.log" },
			IsDirFunc: func() bool { return false },
		}

		// Use a proper exclude pattern that will match test.log
		err := copier.processEntry(entry, localDir, "remote", []string{"*.log"})

		assert.NoError(t, err)
	})

	t.Run("directory entry should copy directory", func(t *testing.T) {
		// Create test directory
		testDir := filepath.Join(localDir, "testdir")
		require.NoError(t, os.MkdirAll(testDir, 0755))

		mockSFTP := &MockSFTPClient{
			MkdirAllFunc: func(path string) error {
				return nil
			},
		}

		copier := NewCopier(mockSFTP)
		entry := &MockDirEntry{
			NameFunc:  func() string { return "testdir" },
			IsDirFunc: func() bool { return true },
		}

		err := copier.processEntry(entry, localDir, "remote", nil)

		assert.NoError(t, err)
	})

	t.Run("file entry should copy file", func(t *testing.T) {
		// Create test file
		testFile := filepath.Join(localDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

		mockSFTP := &MockSFTPClient{
			CreateFunc: func(path string) (io.WriteCloser, error) {
				return &MockWriteCloser{}, nil
			},
			StatFunc: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
		}

		copier := NewCopier(mockSFTP)
		entry := &MockDirEntry{
			NameFunc:  func() string { return "test.txt" },
			IsDirFunc: func() bool { return false },
		}

		err := copier.processEntry(entry, localDir, "remote", nil)

		assert.NoError(t, err)
	})
}
