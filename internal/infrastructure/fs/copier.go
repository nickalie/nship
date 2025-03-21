// Package fs provides functionality for file system operations and copying files.
package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nickalie/nship/internal/util"
)

// SFTPClient abstracts SFTP operations
type SFTPClient interface {
	Create(path string) (io.WriteCloser, error)
	MkdirAll(path string) error
	Chmod(path string, mode os.FileMode) error
	Stat(path string) (os.FileInfo, error)
}

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
func (c *Copier) CopyPath(local, remote string, exclude []string) error {
	localInfo, err := c.fs.Stat(local)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if localInfo.IsDir() {
		return c.CopyDir(local, remote, exclude)
	}
	return c.CopyFile(local, remote)
}

// CopyFile copies a single file
func (c *Copier) CopyFile(local, remote string) error {
	localFile, err := c.fs.Open(local)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer localFile.Close()

	remoteDir := filepath.ToSlash(filepath.Dir(remote))
	if err := c.client.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	remoteFile, err := c.client.Create(remote)
	if err != nil {
		return fmt.Errorf("create destination file: %s, %w", remote, err)
	}
	defer remoteFile.Close()

	if _, err := io.Copy(remoteFile, localFile); err != nil {
		return fmt.Errorf("copy file content: %w", err)
	}

	localInfo, err := c.fs.Stat(local)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	if err := c.client.Chmod(remote, localInfo.Mode()); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}

	return nil
}

// CopyDir copies a directory recursively
func (c *Copier) CopyDir(local, remote string, exclude []string) error {
	if err := c.client.MkdirAll(remote); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	entries, err := c.fs.ReadDir(local)
	if err != nil {
		return fmt.Errorf("read source directory: %w", err)
	}

	return c.processEntries(entries, local, remote, exclude)
}

func (c *Copier) processEntries(entries []os.DirEntry, local, remote string, exclude []string) error {
	for _, entry := range entries {
		if err := c.processEntry(entry, local, remote, exclude); err != nil {
			return err
		}
	}
	return nil
}

func (c *Copier) processEntry(entry os.DirEntry, local, remote string, exclude []string) error {
	localPath := filepath.Join(local, entry.Name())
	remotePath := filepath.ToSlash(filepath.Join(remote, entry.Name()))

	// Use the utility package's IsExcluded function
	if util.IsExcluded(localPath, entry.Name(), exclude) {
		return nil
	}

	ok, err := c.shouldTransferFile(localPath, remotePath)
	if err != nil {
		return fmt.Errorf("check file transfer: %w", err)
	}
	if !ok {
		return nil
	}

	if entry.IsDir() {
		return c.CopyDir(localPath, remotePath, exclude)
	}
	return c.CopyFile(localPath, remotePath)
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
