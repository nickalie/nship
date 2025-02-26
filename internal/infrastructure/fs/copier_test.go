package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockFileSystem implements FileSystem for testing
type MockFileSystem struct {
	StatFunc    func(name string) (os.FileInfo, error)
	OpenFunc    func(name string) (io.ReadCloser, error)
	ReadDirFunc func(name string) ([]os.DirEntry, error)
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, errors.New("Stat not implemented")
}

func (m *MockFileSystem) Open(name string) (io.ReadCloser, error) {
	if m.OpenFunc != nil {
		return m.OpenFunc(name)
	}
	return nil, errors.New("Open not implemented")
}

func (m *MockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	if m.ReadDirFunc != nil {
		return m.ReadDirFunc(name)
	}
	return nil, errors.New("ReadDir not implemented")
}

func (m *MockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return errors.New("WriteFile not implemented")
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return errors.New("MkdirAll not implemented")
}

func (m *MockFileSystem) RemoveAll(path string) error {
	return errors.New("RemoveAll not implemented")
}

// MockFileInfo implements os.FileInfo for testing
type MockFileInfo struct {
	NameFunc    func() string
	SizeFunc    func() int64
	ModeFunc    func() os.FileMode
	ModTimeFunc func() time.Time
	IsDirFunc   func() bool
	SysFunc     func() interface{}
}

func (m *MockFileInfo) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-file"
}

func (m *MockFileInfo) Size() int64 {
	if m.SizeFunc != nil {
		return m.SizeFunc()
	}
	return 100
}

func (m *MockFileInfo) Mode() os.FileMode {
	if m.ModeFunc != nil {
		return m.ModeFunc()
	}
	return 0644
}

func (m *MockFileInfo) ModTime() time.Time {
	if m.ModTimeFunc != nil {
		return m.ModTimeFunc()
	}
	return time.Now()
}

func (m *MockFileInfo) IsDir() bool {
	if m.IsDirFunc != nil {
		return m.IsDirFunc()
	}
	return false
}

func (m *MockFileInfo) Sys() interface{} {
	if m.SysFunc != nil {
		return m.SysFunc()
	}
	return nil
}

// MockDirEntry implements os.DirEntry for testing
type MockDirEntry struct {
	NameFunc  func() string
	IsDirFunc func() bool
	TypeFunc  func() os.FileMode
	InfoFunc  func() (os.FileInfo, error)
}

func (m *MockDirEntry) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-entry"
}

func (m *MockDirEntry) IsDir() bool {
	if m.IsDirFunc != nil {
		return m.IsDirFunc()
	}
	return false
}

func (m *MockDirEntry) Type() os.FileMode {
	if m.TypeFunc != nil {
		return m.TypeFunc()
	}
	return 0
}

func (m *MockDirEntry) Info() (os.FileInfo, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc()
	}
	return &MockFileInfo{}, nil
}

// MockReadCloser implements io.ReadCloser for testing
type MockReadCloser struct {
	ReadFunc  func(p []byte) (n int, err error)
	CloseFunc func() error
}

func (m *MockReadCloser) Read(p []byte) (n int, err error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(p)
	}
	return 0, io.EOF
}

func (m *MockReadCloser) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockWriteCloser implements io.WriteCloser for testing
type MockWriteCloser struct {
	WriteFunc func(p []byte) (n int, err error)
	CloseFunc func() error
}

func (m *MockWriteCloser) Write(p []byte) (n int, err error) {
	if m.WriteFunc != nil {
		return m.WriteFunc(p)
	}
	return len(p), nil
}

func (m *MockWriteCloser) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockSFTPClient implements SFTPClient for testing
type MockSFTPClient struct {
	CreateFunc   func(path string) (io.WriteCloser, error)
	MkdirAllFunc func(path string) error
	ChmodFunc    func(path string, mode os.FileMode) error
	StatFunc     func(path string) (os.FileInfo, error)
}

func (m *MockSFTPClient) Create(path string) (io.WriteCloser, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(path)
	}
	return &MockWriteCloser{}, nil
}

func (m *MockSFTPClient) MkdirAll(path string) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path)
	}
	return nil
}

func (m *MockSFTPClient) Chmod(path string, mode os.FileMode) error {
	if m.ChmodFunc != nil {
		return m.ChmodFunc(path, mode)
	}
	return nil
}

func (m *MockSFTPClient) Stat(path string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(path)
	}
	return &MockFileInfo{}, nil
}

func TestCopyFile(t *testing.T) {
	mockContent := []byte("test file content")
	fileCreated := false
	fileWritten := false

	mockFS := &MockFileSystem{
		StatFunc: func(name string) (os.FileInfo, error) {
			return &MockFileInfo{
				SizeFunc: func() int64 {
					return int64(len(mockContent))
				},
				IsDirFunc: func() bool {
					return false
				},
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
			fileCreated = true
			return &MockWriteCloser{
				WriteFunc: func(p []byte) (n int, err error) {
					if string(p) == string(mockContent) {
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

	if err != nil {
		t.Errorf("CopyFile returned error: %v", err)
	}

	if !fileCreated {
		t.Error("Destination file was not created")
	}

	if !fileWritten {
		t.Error("Content was not correctly written to destination")
	}
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

	if err != nil {
		t.Errorf("CopyDir returned error: %v", err)
	}

	// Check directories were created
	expectedDirs := []string{"dst", "dst/subdir"}
	for _, dir := range expectedDirs {
		if !createdDirs[dir] {
			t.Errorf("Directory not created: %s", dir)
		}
	}

	// Check files were created
	expectedFiles := []string{"dst/file1.txt", "dst/subdir/file2.txt"}
	for _, file := range expectedFiles {
		if !createdFiles[file] {
			t.Errorf("File not created: %s", file)
		}
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
			if result != tt.expected {
				t.Errorf("isExcluded(%q, %v) = %v, want %v", tt.path, tt.exclude, result, tt.expected)
			}
		})
	}
}
