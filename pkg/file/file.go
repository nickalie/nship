package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileSystem abstracts OS file operations
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	Open(name string) (io.ReadCloser, error)
	ReadDir(name string) ([]os.DirEntry, error)
}

// SFTPClient abstracts SFTP operations
type SFTPClient interface {
	Create(path string) (io.WriteCloser, error)
	MkdirAll(path string) error
	Chmod(path string, mode os.FileMode) error
	Stat(path string) (os.FileInfo, error)
}

// DefaultFileSystem implements FileSystem using OS calls
type DefaultFileSystem struct{}

func (fs *DefaultFileSystem) Stat(name string) (os.FileInfo, error)      { return os.Stat(name) }
func (fs *DefaultFileSystem) Open(name string) (io.ReadCloser, error)    { return os.Open(name) }
func (fs *DefaultFileSystem) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }

// Copier handles file copy operations
type Copier struct {
	fs     FileSystem
	client SFTPClient
}

// NewCopier creates a new Copier instance
func NewCopier(fs FileSystem, client SFTPClient) *Copier {
	return &Copier{fs: fs, client: client}
}

// CopyPath copies a file or directory
func (c *Copier) CopyPath(src, dst string, exclude []string) error {
	srcInfo, err := c.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return c.CopyDir(src, dst, exclude)
	}
	return c.CopyFile(src, dst)
}

// CopyFile copies a single file
func (c *Copier) CopyFile(src, dst string) error {
	srcFile, err := c.fs.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	dstDir := filepath.ToSlash(filepath.Dir(dst))
	if err := c.client.MkdirAll(dstDir); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	dstFile, err := c.client.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy file content: %w", err)
	}

	srcInfo, err := c.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	if err := c.client.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}

	return nil
}

// CopyDir copies a directory recursively
func (c *Copier) CopyDir(src, dst string, exclude []string) error {
	if err := c.client.MkdirAll(dst); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	entries, err := c.fs.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source directory: %w", err)
	}

	return c.processEntries(entries, src, dst, exclude)
}

func (c *Copier) processEntries(entries []os.DirEntry, src, dst string, exclude []string) error {
	for _, entry := range entries {
		if err := c.processEntry(entry, src, dst, exclude); err != nil {
			return err
		}
	}
	return nil
}

func (c *Copier) processEntry(entry os.DirEntry, src, dst string, exclude []string) error {
	srcPath := filepath.Join(src, entry.Name())
	dstPath := filepath.ToSlash(filepath.Join(dst, entry.Name()))

	if isExcluded(srcPath, exclude) {
		return nil
	}

	ok, err := c.shouldTransferFile(srcPath, dstPath)
	if err != nil {
		return fmt.Errorf("check file transfer: %w", err)
	}
	if !ok {
		return nil
	}

	if entry.IsDir() {
		return c.CopyDir(srcPath, dstPath, exclude)
	}
	return c.CopyFile(srcPath, dstPath)
}

func (c *Copier) shouldTransferFile(localPath, remotePath string) (bool, error) {
	localInfo, err := c.fs.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("stat local file: %w", err)
	}

	remoteInfo, err := c.client.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("stat remote file: %w", err)
	}

	return localInfo.Size() != remoteInfo.Size(), nil
}

// isExcluded checks if a path matches any exclude pattern
func isExcluded(path string, exclude []string) bool {
	for _, pattern := range exclude {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}
