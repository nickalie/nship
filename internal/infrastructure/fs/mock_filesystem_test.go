package fs

import (
	"io"
	"os"
)

// MockFileSystem implements FileSystem for testing
type MockFileSystem struct {
	StatFunc      func(name string) (os.FileInfo, error)
	OpenFunc      func(name string) (io.ReadCloser, error)
	ReadDirFunc   func(name string) ([]os.DirEntry, error)
	ReadFileFunc  func(name string) ([]byte, error)
	WriteFileFunc func(name string, data []byte, perm os.FileMode) error
	MkdirAllFunc  func(path string, perm os.FileMode) error
	RemoveAllFunc func(path string) error
}

// Stat mocks the Stat method of FileSystem interface
func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, os.ErrNotExist
}

// Open mocks the Open method of FileSystem interface
func (m *MockFileSystem) Open(name string) (io.ReadCloser, error) {
	if m.OpenFunc != nil {
		return m.OpenFunc(name)
	}
	return nil, os.ErrNotExist
}

// ReadDir mocks the ReadDir method of FileSystem interface
func (m *MockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	if m.ReadDirFunc != nil {
		return m.ReadDirFunc(name)
	}
	return nil, os.ErrNotExist
}

// ReadFile mocks the ReadFile method of FileSystem interface
func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(name)
	}
	return nil, os.ErrNotExist
}

// WriteFile mocks the WriteFile method of FileSystem interface
func (m *MockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(name, data, perm)
	}
	return os.ErrNotExist
}

// MkdirAll mocks the MkdirAll method of FileSystem interface
func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return os.ErrNotExist
}

// RemoveAll mocks the RemoveAll method of FileSystem interface
func (m *MockFileSystem) RemoveAll(path string) error {
	if m.RemoveAllFunc != nil {
		return m.RemoveAllFunc(path)
	}
	return os.ErrNotExist
}
