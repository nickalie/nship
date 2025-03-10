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

func TestCopyPath(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		dst         string
		exclude     []string
		isSourceDir bool
		setupMock   func() (*MockFileSystem, *MockSFTPClient)
		expectErr   bool
	}{
		{
			name:        "copy file",
			src:         "src/file.txt",
			dst:         "dst/file.txt",
			exclude:     nil,
			isSourceDir: false,
			setupMock: func() (*MockFileSystem, *MockSFTPClient) {
				mockContent := []byte("test file content")

				mockFS := setupMockFileSystem(mockContent, false)

				mockSFTP := &MockSFTPClient{
					CreateFunc: func(path string) (io.WriteCloser, error) {
						return &MockWriteCloser{}, nil
					},
					StatFunc: func(path string) (os.FileInfo, error) {
						return nil, os.ErrNotExist
					},
				}

				return mockFS, mockSFTP
			},
			expectErr: false,
		},
		{
			name:        "copy directory",
			src:         "src",
			dst:         "dst",
			exclude:     nil,
			isSourceDir: true,
			setupMock: func() (*MockFileSystem, *MockSFTPClient) {
				mockFS := &MockFileSystem{
					StatFunc: func(name string) (os.FileInfo, error) {
						return &MockFileInfo{
							IsDirFunc: func() bool {
								return true
							},
						}, nil
					},
					ReadDirFunc: func(name string) ([]os.DirEntry, error) {
						return []os.DirEntry{}, nil
					},
				}

				mockSFTP := &MockSFTPClient{
					MkdirAllFunc: func(path string) error {
						return nil
					},
				}

				return mockFS, mockSFTP
			},
			expectErr: false,
		},
		{
			name:        "source not found",
			src:         "nonexistent",
			dst:         "dst",
			exclude:     nil,
			isSourceDir: false,
			setupMock: func() (*MockFileSystem, *MockSFTPClient) {
				mockFS := &MockFileSystem{
					StatFunc: func(name string) (os.FileInfo, error) {
						return nil, os.ErrNotExist
					},
				}

				mockSFTP := &MockSFTPClient{}

				return mockFS, mockSFTP
			},
			expectErr: true,
		},
		{
			name:        "copy directory with exclusions",
			src:         "src",
			dst:         "dst",
			exclude:     []string{"*.log", "node_modules"},
			isSourceDir: true,
			setupMock: func() (*MockFileSystem, *MockSFTPClient) {
				mockFS := &MockFileSystem{
					StatFunc: func(name string) (os.FileInfo, error) {
						// Fix: Handle paths properly to avoid infinite recursion
						baseName := filepath.Base(name)

						// Explicitly handle paths we know about
						if name == "src" {
							return &MockFileInfo{
								IsDirFunc: func() bool { return true },
							}, nil
						}

						if baseName == "debug.log" || baseName == "node_modules" {
							return &MockFileInfo{
								IsDirFunc: func() bool { return baseName == "node_modules" },
								SizeFunc:  func() int64 { return 100 },
							}, nil
						}

						if baseName == "file.txt" {
							return &MockFileInfo{
								IsDirFunc: func() bool { return false },
								SizeFunc:  func() int64 { return 200 },
							}, nil
						}

						// For any other paths, default behavior
						return &MockFileInfo{
							IsDirFunc: func() bool { return false },
							SizeFunc:  func() int64 { return 0 },
						}, nil
					},
					ReadDirFunc: func(name string) ([]os.DirEntry, error) {
						// Only return entries for the root src directory
						// to prevent recursive scanning
						if name == "src" {
							return []os.DirEntry{
								&MockDirEntry{
									NameFunc:  func() string { return "file.txt" },
									IsDirFunc: func() bool { return false },
								},
								&MockDirEntry{
									NameFunc:  func() string { return "debug.log" },
									IsDirFunc: func() bool { return false },
								},
								&MockDirEntry{
									NameFunc:  func() string { return "node_modules" },
									IsDirFunc: func() bool { return true },
								},
							}, nil
						}

						// For node_modules dir, return empty to avoid recursion
						if filepath.Base(name) == "node_modules" {
							return []os.DirEntry{}, nil
						}

						return []os.DirEntry{}, nil
					},
					OpenFunc: func(name string) (io.ReadCloser, error) {
						return &MockReadCloser{
							ReadFunc: func(p []byte) (n int, err error) {
								// Return a small amount of content to avoid memory issues
								content := []byte("test")
								copy(p, content)
								return len(content), io.EOF
							},
							CloseFunc: func() error {
								return nil
							},
						}, nil
					},
				}

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

				return mockFS, mockSFTP
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS, mockSFTP := tt.setupMock()
			copier := NewCopier(mockFS, mockSFTP)

			err := copier.CopyPath(tt.src, tt.dst, tt.exclude)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestShouldTransferFile(t *testing.T) {
	tests := []struct {
		name       string
		localSize  int64
		remoteSize int64
		localErr   error
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
			name:      "local file not found should return error",
			localErr:  fmt.Errorf("stat error"),
			expected:  false,
			expectErr: true,
		},
		{
			name:      "remote file not found should transfer",
			remoteErr: os.ErrNotExist,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "remote stat error should return error",
			remoteErr: fmt.Errorf("remote stat error"),
			expected:  false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := &MockFileSystem{
				StatFunc: func(name string) (os.FileInfo, error) {
					if tt.localErr != nil {
						return nil, tt.localErr
					}
					return &MockFileInfo{
						SizeFunc: func() int64 {
							return tt.localSize
						},
					}, nil
				},
			}

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

			copier := NewCopier(mockFS, mockSFTP)
			result, err := copier.shouldTransferFile("local/path", "remote/path")

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
	t.Run("excluded file should be skipped", func(t *testing.T) {
		// Create a proper mock file system that won't try to access files
		mockFS := &MockFileSystem{
			StatFunc: func(name string) (os.FileInfo, error) {
				// We shouldn't reach this point if exclusion works correctly
				// but provide implementation just in case
				return &MockFileInfo{
					IsDirFunc: func() bool { return false },
				}, nil
			},
		}

		mockSFTP := &MockSFTPClient{}

		copier := NewCopier(mockFS, mockSFTP)
		entry := &MockDirEntry{
			NameFunc:  func() string { return "test.log" },
			IsDirFunc: func() bool { return false },
		}

		// Use a proper exclude pattern that will match test.log
		err := copier.processEntry(entry, "src", "dst", []string{"*.log"})

		assert.NoError(t, err)
	})

	t.Run("directory entry should copy directory", func(t *testing.T) {
		mockFS := &MockFileSystem{
			StatFunc: func(name string) (os.FileInfo, error) {
				return &MockFileInfo{
					IsDirFunc: func() bool { return true },
				}, nil
			},
			ReadDirFunc: func(name string) ([]os.DirEntry, error) {
				return []os.DirEntry{}, nil
			},
		}

		mockSFTP := &MockSFTPClient{
			MkdirAllFunc: func(path string) error {
				return nil
			},
		}

		copier := NewCopier(mockFS, mockSFTP)
		entry := &MockDirEntry{
			NameFunc:  func() string { return "testdir" },
			IsDirFunc: func() bool { return true },
		}

		err := copier.processEntry(entry, "src", "dst", nil)

		assert.NoError(t, err)
	})

	t.Run("file entry should copy file", func(t *testing.T) {
		mockContent := []byte("test content")
		mockFS := &MockFileSystem{
			StatFunc: func(name string) (os.FileInfo, error) {
				return &MockFileInfo{
					IsDirFunc: func() bool { return false },
				}, nil
			},
			OpenFunc: func(name string) (io.ReadCloser, error) {
				return &MockReadCloser{
					ReadFunc: func(p []byte) (n int, err error) {
						copy(p, mockContent)
						return len(mockContent), io.EOF
					},
				}, nil
			},
		}

		mockSFTP := &MockSFTPClient{
			CreateFunc: func(path string) (io.WriteCloser, error) {
				return &MockWriteCloser{}, nil
			},
			StatFunc: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
		}

		copier := NewCopier(mockFS, mockSFTP)
		entry := &MockDirEntry{
			NameFunc:  func() string { return "test.txt" },
			IsDirFunc: func() bool { return false },
		}

		err := copier.processEntry(entry, "src", "dst", nil)

		assert.NoError(t, err)
	})
}
