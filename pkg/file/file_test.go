package file

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

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
		{
			name: "copy file content error",
			src:  "/src/file.txt",
			dst:  "/dst/file.txt",
			srcFile: &mockFile{
				content: "test content",
				mode:    0644,
			},
			wantErr: true,
			setupMock: func(fs *mockFileSystem, sftp *mockSFTPClient) {
				sftp.createErr = errors.New("copy file content error")
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

func TestCopyPath(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		dst       string
		srcFile   *mockFile
		exclude   []string
		wantErr   bool
		setupMock func(*mockFileSystem, *mockSFTPClient)
	}{
		{
			name: "copy file successfully",
			src:  "/src/file.txt",
			dst:  "/dst/file.txt",
			srcFile: &mockFile{
				content: "test content",
				mode:    0644,
				size:    12,
				isDir:   false,
			},
			wantErr: false,
		},
		{
			name: "copy directory successfully",
			src:  "/src/dir",
			dst:  "/dst/dir",
			srcFile: &mockFile{
				mode:  0755,
				isDir: true,
			},
			wantErr: false,
		},
		{
			name:    "source not found",
			src:     "/src/nonexistent",
			dst:     "/dst/nonexistent",
			wantErr: true,
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
			err := copier.CopyPath(tt.src, tt.dst, tt.exclude)

			if (err != nil) != tt.wantErr {
				t.Errorf("CopyPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldTransferFile(t *testing.T) {
	tests := []struct {
		name       string
		local      string
		remote     string
		localFile  *mockFile
		remoteFile *mockFile
		want       bool
		wantErr    bool
		setupMock  func(*mockFileSystem, *mockSFTPClient)
	}{
		{
			name:   "different sizes",
			local:  "/src/file.txt",
			remote: "/dst/file.txt",
			localFile: &mockFile{
				content: "content1",
				size:    8,
			},
			remoteFile: &mockFile{
				content: "different",
				size:    9,
			},
			want:    true,
			wantErr: false,
		},
		{
			name:   "same sizes",
			local:  "/src/file.txt",
			remote: "/dst/file.txt",
			localFile: &mockFile{
				content: "content",
				size:    7,
			},
			remoteFile: &mockFile{
				content: "content",
				size:    7,
			},
			want:    false,
			wantErr: false,
		},
		{
			name:    "local stat error",
			local:   "/src/file.txt",
			remote:  "/dst/file.txt",
			wantErr: true,
			setupMock: func(fs *mockFileSystem, sftp *mockSFTPClient) {
				fs.statErr = errors.New("stat error")
			},
		},
		{
			name:   "remote stat error",
			local:  "/src/file.txt",
			remote: "/dst/file.txt",
			localFile: &mockFile{
				content: "content",
				size:    7,
			},
			wantErr: true,
			setupMock: func(fs *mockFileSystem, sftp *mockSFTPClient) {
				sftp.statErr = errors.New("stat error")
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

			if tt.localFile != nil {
				fs.files[tt.local] = tt.localFile
			}
			if tt.remoteFile != nil {
				sftp.files[tt.remote] = tt.remoteFile
			}

			if tt.setupMock != nil {
				tt.setupMock(fs, sftp)
			}

			copier := NewCopier(fs, sftp)
			got, err := copier.shouldTransferFile(tt.local, tt.remote)

			if (err != nil) != tt.wantErr {
				t.Errorf("shouldTransferFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got != tt.want {
				t.Errorf("shouldTransferFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopyDir(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		dst       string
		srcFiles  map[string]*mockFile
		wantErr   bool
		setupMock func(*mockFileSystem, *mockSFTPClient)
	}{
		{
			name: "copy directory successfully",
			src:  "/src/dir",
			dst:  "/dst/dir",
			srcFiles: map[string]*mockFile{
				"/src/dir/file1.txt": {content: "content1", mode: 0644, size: 8, isDir: false},
				"/src/dir/file2.txt": {content: "content2", mode: 0644, size: 8, isDir: false},
			},
			wantErr: false,
		},
		{
			name: "create destination directory error",
			src:  "/src/dir",
			dst:  "/dst/dir",
			srcFiles: map[string]*mockFile{
				"/src/dir/file1.txt": {content: "content1", mode: 0644, size: 8, isDir: false},
			},
			wantErr: true,
			setupMock: func(fs *mockFileSystem, sftp *mockSFTPClient) {
				sftp.mkdirErr = errors.New("mkdir error")
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

			for path, file := range tt.srcFiles {
				fs.files[path] = file
			}

			if tt.setupMock != nil {
				tt.setupMock(fs, sftp)
			}

			copier := NewCopier(fs, sftp)
			err := copier.CopyDir(tt.src, tt.dst, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("CopyDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessEntry(t *testing.T) {
	tests := []struct {
		name        string
		entry       *mockDirEntry
		src         string
		dst         string
		exclude     []string
		srcFiles    map[string]*mockFile
		remoteFiles map[string]*mockFile
		wantErr     bool
		setupMock   func(*mockFileSystem, *mockSFTPClient)
	}{
		{
			name: "process file entry successfully",
			entry: &mockDirEntry{
				name:  "test.txt",
				isDir: false,
			},
			src: "/src",
			dst: "/dst",
			srcFiles: map[string]*mockFile{
				"test.txt": { // Use relative path
					content: "test content",
					mode:    0644,
					size:    12,
					isDir:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "process directory entry successfully",
			entry: &mockDirEntry{
				name:  "testdir",
				isDir: true,
			},
			src: "/src",
			dst: "/dst",
			srcFiles: map[string]*mockFile{
				"testdir": { // Use relative path
					mode:  0755,
					isDir: true,
				},
			},
			wantErr: false,
		},
		{
			name: "excluded file",
			entry: &mockDirEntry{
				name:  "test.tmp",
				isDir: false,
			},
			src:     "/src",
			dst:     "/dst",
			exclude: []string{"test.tmp"},
			srcFiles: map[string]*mockFile{
				"test.tmp": { // Use relative path
					content: "temp content",
					mode:    0644,
					isDir:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "stat error",
			entry: &mockDirEntry{
				name:  "test.txt",
				isDir: false,
			},
			src:     "/src",
			dst:     "/dst",
			wantErr: true,
			setupMock: func(fs *mockFileSystem, sftp *mockSFTPClient) {
				fs.statErr = errors.New("stat error")
			},
		},
		{
			name: "same size file skip",
			entry: &mockDirEntry{
				name:  "test.txt",
				isDir: false,
			},
			src: "/src",
			dst: "/dst",
			srcFiles: map[string]*mockFile{
				"test.txt": { // Use relative path
					content: "content",
					size:    7,
					mode:    0644,
					isDir:   false,
				},
			},
			remoteFiles: map[string]*mockFile{
				"test.txt": { // Use relative path
					content: "content",
					size:    7,
					mode:    0644,
					isDir:   false,
				},
			},
			wantErr: false,
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

			// Set up source files
			for name, file := range tt.srcFiles {
				fullPath := filepath.Join(tt.src, name)
				fs.files[fullPath] = file
			}

			// Set up remote files
			for name, file := range tt.remoteFiles {
				fullPath := filepath.Join(tt.dst, name)
				sftp.files[fullPath] = file
			}

			if tt.setupMock != nil {
				tt.setupMock(fs, sftp)
			}

			copier := NewCopier(fs, sftp)
			err := copier.processEntry(tt.entry, tt.src, tt.dst, tt.exclude)

			if (err != nil) != tt.wantErr {
				t.Errorf("processEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// mockDirEntry implements os.DirEntry
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string {
	return m.name
}

func (m *mockDirEntry) IsDir() bool {
	return m.isDir
}

func (m *mockDirEntry) Type() os.FileMode {
	if m.isDir {
		return os.ModeDir
	}
	return 0
}

func (m *mockDirEntry) Info() (os.FileInfo, error) {
	return mockFileInfo{
		name:  m.name,
		isDir: m.isDir,
	}, nil
}
