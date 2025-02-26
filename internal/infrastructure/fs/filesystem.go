package fs

import (
	"io"
	"os"
)

// FileSystem abstracts OS file operations
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	Open(name string) (io.ReadCloser, error)
	ReadDir(name string) ([]os.DirEntry, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
}

// DefaultFileSystem implements FileSystem using OS calls
type DefaultFileSystem struct{}

// NewFileSystem creates a new default file system implementation
func NewFileSystem() FileSystem {
	return &DefaultFileSystem{}
}

// Stat implements the FileSystem interface by returning file info using os.Stat
func (fs *DefaultFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// Open implements the FileSystem interface by opening a file using os.Open
func (fs *DefaultFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// ReadDir implements the FileSystem interface by reading directory entries using os.ReadDir
func (fs *DefaultFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

// WriteFile implements the FileSystem interface by writing a file using os.WriteFile
func (fs *DefaultFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// MkdirAll implements the FileSystem interface by creating directories using os.MkdirAll
func (fs *DefaultFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// RemoveAll implements the FileSystem interface by removing files/directories using os.RemoveAll
func (fs *DefaultFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}
