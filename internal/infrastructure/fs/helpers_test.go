package fs

import (
	"io"
	"os"
	"time"
)

// MockReadCloser implements io.ReadCloser for testing
type MockReadCloser struct {
	ReadFunc  func(p []byte) (n int, err error)
	CloseFunc func() error
}

// Read implements io.Reader for MockReadCloser
func (m *MockReadCloser) Read(p []byte) (n int, err error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(p)
	}
	return 0, io.EOF
}

// Close implements io.Closer for MockReadCloser
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

// Write implements io.Writer for MockWriteCloser
func (m *MockWriteCloser) Write(p []byte) (n int, err error) {
	if m.WriteFunc != nil {
		return m.WriteFunc(p)
	}
	return len(p), nil
}

// Close implements io.Closer for MockWriteCloser
func (m *MockWriteCloser) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
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

// Name implements os.FileInfo.Name
func (m *MockFileInfo) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-file"
}

// Size implements os.FileInfo.Size
func (m *MockFileInfo) Size() int64 {
	if m.SizeFunc != nil {
		return m.SizeFunc()
	}
	return 100
}

// Mode implements os.FileInfo.Mode
func (m *MockFileInfo) Mode() os.FileMode {
	if m.ModeFunc != nil {
		return m.ModeFunc()
	}
	return 0644
}

// ModTime implements os.FileInfo.ModTime
func (m *MockFileInfo) ModTime() time.Time {
	if m.ModTimeFunc != nil {
		return m.ModTimeFunc()
	}
	return time.Now()
}

// IsDir implements os.FileInfo.IsDir
func (m *MockFileInfo) IsDir() bool {
	if m.IsDirFunc != nil {
		return m.IsDirFunc()
	}
	return false
}

// Sys implements os.FileInfo.Sys
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

// Name implements os.DirEntry.Name
func (m *MockDirEntry) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-entry"
}

// IsDir implements os.DirEntry.IsDir
func (m *MockDirEntry) IsDir() bool {
	if m.IsDirFunc != nil {
		return m.IsDirFunc()
	}
	return false
}

// Type implements os.DirEntry.Type
func (m *MockDirEntry) Type() os.FileMode {
	if m.TypeFunc != nil {
		return m.TypeFunc()
	}
	return 0
}

// Info implements os.DirEntry.Info
func (m *MockDirEntry) Info() (os.FileInfo, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc()
	}
	return &MockFileInfo{}, nil
}

// MockSFTPClient implements SFTPClient for testing
