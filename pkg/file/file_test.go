package file

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// Mock implementations
type mockFileSystem struct {
	files   map[string]*mockFile
	statErr error
	openErr error
	readErr error
	entries []os.DirEntry
}

type mockFile struct {
	content string
	mode    os.FileMode
	size    int64
	isDir   bool
}

type mockSFTPClient struct {
	files     map[string]*mockFile
	mkdirErr  error
	createErr error
	chmodErr  error
	statErr   error
}

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	isDir   bool
	modTime time.Time
}

type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

// DirEntry interface implementation
func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return m.isDir }
func (m mockDirEntry) Type() os.FileMode          { return 0 }
func (m mockDirEntry) Info() (os.FileInfo, error) { return nil, nil }

// MockFileSystem implementation
func (m *mockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	if file, exists := m.files[name]; exists {
		return mockFileInfo{name: name, size: file.size, mode: file.mode, isDir: file.isDir}, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockFileSystem) Open(name string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	if file, exists := m.files[name]; exists {
		return io.NopCloser(strings.NewReader(file.content)), nil
	}
	return nil, os.ErrNotExist
}

func (m *mockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	return m.entries, nil
}

// MockSFTPClient implementation
type mockWriteCloser struct {
	*strings.Builder
}

func (m mockWriteCloser) Close() error { return nil }

func (m *mockSFTPClient) Create(path string) (io.WriteCloser, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.files == nil {
		m.files = make(map[string]*mockFile)
	}
	m.files[path] = &mockFile{content: "", mode: 0644}
	return &mockWriteCloser{&strings.Builder{}}, nil
}

func (m *mockSFTPClient) MkdirAll(path string) error {
	if m.mkdirErr != nil {
		return m.mkdirErr
	}
	return nil
}

func (m *mockSFTPClient) Chmod(path string, mode os.FileMode) error {
	if m.chmodErr != nil {
		return m.chmodErr
	}
	if file, exists := m.files[path]; exists {
		file.mode = mode
	}
	return nil
}

func (m *mockSFTPClient) Stat(path string) (os.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	if file, exists := m.files[path]; exists {
		return mockFileInfo{name: path, size: file.size, mode: file.mode, isDir: file.isDir}, nil
	}
	return nil, os.ErrNotExist
}

func TestCopyFile(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		dst       string
		srcFile   *mockFile
		wantErr   bool
		setupMock func(*mockFileSystem, *mockSFTPClient)
	}{
		{
			name: "successful copy",
			src:  "/src/file.txt",
			dst:  "/dst/file.txt",
			srcFile: &mockFile{
				content: "test content",
				mode:    0644,
				size:    12,
			},
			wantErr: false,
		},
		{
			name:    "source file not found",
			src:     "/src/nonexistent.txt",
			dst:     "/dst/file.txt",
			wantErr: true,
		},
		{
			name: "create destination error",
			src:  "/src/file.txt",
			dst:  "/dst/file.txt",
			srcFile: &mockFile{
				content: "test content",
				mode:    0644,
			},
			wantErr: true,
			setupMock: func(fs *mockFileSystem, sftp *mockSFTPClient) {
				sftp.createErr = errors.New("create error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &mockFileSystem{
				files: make(map[string]*mockFile),
			}
			sftp := &mockSFTPClient{
				files: make(map[string]*mockFile),
			}

			if tt.srcFile != nil {
				fs.files[tt.src] = tt.srcFile
			}

			if tt.setupMock != nil {
				tt.setupMock(fs, sftp)
			}

			copier := NewCopier(fs, sftp)
			err := copier.CopyFile(tt.src, tt.dst)

			if (err != nil) != tt.wantErr {
				t.Errorf("CopyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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
			path:     "test.txt",
			exclude:  nil,
			expected: false,
		},
		{
			name:     "matching pattern",
			path:     "test.txt",
			exclude:  []string{"*.txt"},
			expected: true,
		},
		{
			name:     "non-matching pattern",
			path:     "test.txt",
			exclude:  []string{"*.jpg"},
			expected: false,
		},
		{
			name:     "multiple patterns",
			path:     "test.txt",
			exclude:  []string{"*.jpg", "*.txt", "*.png"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExcluded(tt.path, tt.exclude); got != tt.expected {
				t.Errorf("isExcluded() = %v, want %v", got, tt.expected)
			}
		})
	}
}
