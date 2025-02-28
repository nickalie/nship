package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fix for line 467 that has "no new variables on left side of :=" error
func TestFileSystemIntegration(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fs := NewFileSystem()

	// 1. Create a directory
	dirPath := filepath.Join(tempDir, "integration-test")
	err := fs.MkdirAll(dirPath, 0755)
	require.NoError(t, err, "MkdirAll failed")

	// 2. Write a file
	filePath := filepath.Join(dirPath, "test-file.txt")
	content := []byte("integration test content")
	err = fs.WriteFile(filePath, content, 0644)
	require.NoError(t, err, "WriteFile failed")

	// 3. Get file stats
	fileInfo, err := fs.Stat(filePath)
	require.NoError(t, err, "Stat failed")

	assert.Equal(t, int64(len(content)), fileInfo.Size(), "File size doesn't match expected value")

	// 3b. Read file using ReadFile
	readContent, err := fs.ReadFile(filePath)
	require.NoError(t, err, "ReadFile failed")
	assert.True(t, bytes.Equal(readContent, content), "ReadFile did not return expected content")

	// 4. Read directory entries
	entries, err := fs.ReadDir(dirPath)
	require.NoError(t, err, "ReadDir failed")

	assert.Len(t, entries, 1, "Expected a single directory entry")
	assert.Equal(t, "test-file.txt", entries[0].Name(), "Directory entry name mismatch")

	// 5. Open and read file
	file, err := fs.Open(filePath)
	require.NoError(t, err, "Open failed")

	// Use a different variable name here to avoid the "no new variables" error
	fileContent, readErr := io.ReadAll(file)
	file.Close()
	require.NoError(t, readErr, "Reading file content failed")

	assert.Equal(t, content, fileContent, "File content does not match expected value")

	// 6. Clean up
	err = fs.RemoveAll(dirPath)
	require.NoError(t, err, "RemoveAll failed")

	// Verify directory is removed
	_, err = fs.Stat(dirPath)
	assert.True(t, os.IsNotExist(err), "Directory was not properly removed")
}
