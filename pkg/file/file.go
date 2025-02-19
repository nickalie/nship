package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
)

func CopyPath(client *sftp.Client, src, dst string, exclude []string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return CopyDir(client, src, dst, exclude)
	}

	return CopyFile(client, src, dst)
}

func CopyFile(client *sftp.Client, src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstDir := filepath.ToSlash(filepath.Dir(dst))
	err = client.MkdirAll(dstDir)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := client.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	err = client.Chmod(dst, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

func CopyDir(client *sftp.Client, src, dst string, exclude []string) error {
	err := client.MkdirAll(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.ToSlash(filepath.Join(dst, entry.Name()))

		if isExcluded(srcPath, exclude) {
			continue
		}

		if entry.IsDir() {
			err = CopyDir(client, srcPath, dstPath, exclude)
			if err != nil {
				return err
			}
		} else {
			err = CopyFile(client, srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func isExcluded(path string, exclude []string) bool {
	for _, pattern := range exclude {
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
	}
	return false
}
