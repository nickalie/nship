package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"ngdeploy/pkg/file"
)

type SyncStats struct {
	FilesTransferred int
	FilesSkipped     int
	DirsCreated      int
}

func SyncDirectory(client *sftp.Client, localDir, remoteDir string, exclude []string) (SyncStats, error) {
	stats := SyncStats{}

	_, err := os.Stat(localDir)
	if err != nil {
		return stats, fmt.Errorf("local directory does not exist: %w", err)
	}

	_, err = client.Stat(remoteDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = client.MkdirAll(remoteDir)
			if err != nil {
				return stats, fmt.Errorf("failed to create remote directory: %w", err)
			}
			stats.DirsCreated++
		} else {
			return stats, fmt.Errorf("failed to stat remote directory: %w", err)
		}
	}

	err = filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localDir, localPath)

		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		if isExcluded(relPath, exclude) {
			return nil
		}

		remotePath := filepath.Join(remoteDir, relPath)
		remotePath = filepath.ToSlash(remotePath)

		if info.IsDir() {
			_, err := client.Stat(remotePath)
			if err != nil && os.IsNotExist(err) {
				err = client.MkdirAll(remotePath)
				if err != nil {
					return fmt.Errorf("failed to create remote directory '%s': %w", remotePath, err)
				}
				stats.DirsCreated++
				fmt.Printf("Created directory: %s\n", remotePath)
			}
		} else {
			needsTransfer, err := shouldTransferFile(client, localPath, remotePath, info)
			if err != nil {
				return fmt.Errorf("failed to check if file needs transfer: %w", err)
			}

			if needsTransfer {
				err = file.CopyFile(client, localPath, remotePath)
				if err != nil {
					return fmt.Errorf("failed to transfer file '%s' to '%s': %w", localPath, remotePath, err)
				}
				stats.FilesTransferred++
				fmt.Printf("Transferred: %s\n", relPath)
			} else {
				stats.FilesSkipped++
				fmt.Printf("Skipped (unchanged): %s\n", relPath)
			}
		}

		return nil
	})

	return stats, err
}

func isExcluded(path string, exclude []string) bool {
	for _, pattern := range exclude {
		matched, _ := filepath.Match(pattern, filepath.ToSlash(path))

		if matched {
			return true
		}
	}

	return false
}

func shouldTransferFile(client *sftp.Client, localPath, remotePath string, localInfo os.FileInfo) (bool, error) {
	remoteInfo, err := client.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("failed to stat remote file: %w", err)
	}

	if localInfo.Size() != remoteInfo.Size() {
		return true, nil
	}

	return false, err
}
