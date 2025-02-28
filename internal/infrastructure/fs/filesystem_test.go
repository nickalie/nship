package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Fix for line 467 that has "no new variables on left side of :=" error
func TestFileSystemIntegration(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fs := NewFileSystem()

	// 1. Create a directory
	dirPath := filepath.Join(tempDir, "integration-test")
	err := fs.MkdirAll(dirPath, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// 2. Write a file
	filePath := filepath.Join(dirPath, "test-file.txt")
	content := []byte("integration test content")
	err = fs.WriteFile(filePath, content, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 3. Get file stats
	fileInfo, err := fs.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if fileInfo.Size() != int64(len(content)) {
		t.Errorf("Expected file size %d, got %d", len(content), fileInfo.Size())
	}

	// 3b. Read file using ReadFile
	readContent, err := fs.ReadFile(filePath)
	if err != nil || !bytes.Equal(readContent, content) {
		t.Errorf("ReadFile did not return expected content: %v", err)
	}

	// 4. Read directory entries
	entries, err := fs.ReadDir(dirPath)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != "test-file.txt" {
		t.Errorf("ReadDir did not return expected entries")
	}

	// 5. Open and read file
	file, err := fs.Open(filePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Use a different variable name here to avoid the "no new variables" error
	fileContent, readErr := io.ReadAll(file)
	file.Close()
	if readErr != nil {
		t.Fatalf("Reading file content failed: %v", readErr)
	}

	if !bytes.Equal(fileContent, content) {
		t.Errorf("File content does not match: expected '%s', got '%s'", content, fileContent)
	}

	// 6. Clean up
	err = fs.RemoveAll(dirPath)
	if err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	// Verify directory is removed
	_, err = fs.Stat(dirPath)
	if !os.IsNotExist(err) {
		t.Errorf("Directory was not properly removed")
	}
}
