package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// setupTestEnvironment creates a temporary directory and returns its path
// along with a cleanup function that removes the directory
func setupTestEnvironment(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "fs-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return tempDir, func() {
		os.RemoveAll(tempDir)
	}
}

func TestNewFileSystem(t *testing.T) {
	fs := NewFileSystem()

	// Check if the returned object is of the correct type
	_, ok := fs.(*DefaultFileSystem)
	if !ok {
		t.Errorf("NewFileSystem() did not return a *DefaultFileSystem, got %T", fs)
	}
}

func TestStat(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a test file
	testFilePath := filepath.Join(tempDir, "stat-test.txt")
	testContent := []byte("test content")
	err := os.WriteFile(testFilePath, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := NewFileSystem()

	// Test successful case
	info, err := fs.Stat(testFilePath)
	if err != nil {
		t.Errorf("Stat() returned an error for existing file: %v", err)
	}

	if info.Name() != "stat-test.txt" {
		t.Errorf("Expected file name 'stat-test.txt', got '%s'", info.Name())
	}

	if info.Size() != int64(len(testContent)) {
		t.Errorf("Expected file size %d, got %d", len(testContent), info.Size())
	}

	// Test error case - non-existent file
	_, err = fs.Stat(filepath.Join(tempDir, "non-existent.txt"))
	if err == nil {
		t.Errorf("Stat() did not return an error for non-existent file")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

func TestOpen(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a test file
	testFilePath := filepath.Join(tempDir, "open-test.txt")
	testContent := []byte("test file content")
	err := os.WriteFile(testFilePath, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := NewFileSystem()

	// Test successful case
	file, err := fs.Open(testFilePath)
	if err != nil {
		t.Errorf("Open() returned an error for existing file: %v", err)
	}

	if file != nil {
		defer file.Close()

		// Read the content and verify
		content, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("Failed to read file content: %v", err)
		}

		if !bytes.Equal(content, testContent) {
			t.Errorf("Expected content '%s', got '%s'", testContent, content)
		}
	}

	// Test error case - non-existent file
	_, err = fs.Open(filepath.Join(tempDir, "non-existent.txt"))
	if err == nil {
		t.Errorf("Open() did not return an error for non-existent file")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

func TestReadDir(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test files
	filenames := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, filename := range filenames {
		err := os.WriteFile(filepath.Join(tempDir, filename), []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create a subdirectory
	subdirName := "subdir"
	err := os.Mkdir(filepath.Join(tempDir, subdirName), 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	fs := NewFileSystem()

	// Test successful case
	entries, err := fs.ReadDir(tempDir)
	if err != nil {
		t.Errorf("ReadDir() returned an error: %v", err)
	}

	// Check that all files and subdirectory are listed
	if len(entries) != len(filenames)+1 {
		t.Errorf("Expected %d entries, got %d", len(filenames)+1, len(entries))
	}

	// Verify that all expected files are in the entries
	foundFiles := make(map[string]bool)
	foundSubdir := false

	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == subdirName {
				foundSubdir = true
			}
		} else {
			foundFiles[entry.Name()] = true
		}
	}

	if !foundSubdir {
		t.Errorf("Subdirectory '%s' not found in directory entries", subdirName)
	}

	for _, filename := range filenames {
		if !foundFiles[filename] {
			t.Errorf("File '%s' not found in directory entries", filename)
		}
	}

	// Test error case - non-existent directory
	_, err = fs.ReadDir(filepath.Join(tempDir, "non-existent-dir"))
	if err == nil {
		t.Errorf("ReadDir() did not return an error for non-existent directory")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected 'directory not found' error, got: %v", err)
	}
}

func TestWriteFile(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testFilePath := filepath.Join(tempDir, "write-test.txt")
	testContent := []byte("test write content")

	fs := NewFileSystem()

	// Test creating a new file
	err := fs.WriteFile(testFilePath, testContent, 0644)
	if err != nil {
		t.Errorf("WriteFile() returned an error: %v", err)
	}

	// Verify file was created with correct content
	content, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Errorf("Failed to read test file: %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("Expected content '%s', got '%s'", testContent, content)
	}

	// Test overwriting existing file
	newContent := []byte("new content")
	err = fs.WriteFile(testFilePath, newContent, 0644)
	if err != nil {
		t.Errorf("WriteFile() returned an error when overwriting: %v", err)
	}

	// Verify file was overwritten with new content
	content, err = os.ReadFile(testFilePath)
	if err != nil {
		t.Errorf("Failed to read test file after overwrite: %v", err)
	}

	if !bytes.Equal(content, newContent) {
		t.Errorf("Expected new content '%s', got '%s'", newContent, content)
	}

	// Test writing to a non-existent directory
	err = fs.WriteFile(filepath.Join(tempDir, "non-existent-dir", "file.txt"), testContent, 0644)
	if err == nil {
		t.Errorf("WriteFile() did not return an error for non-existent directory")
	}
}

func TestMkdirAll(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test creating a simple directory
	dirPath := filepath.Join(tempDir, "test-dir")

	fs := NewFileSystem()

	err := fs.MkdirAll(dirPath, 0755)
	if err != nil {
		t.Errorf("MkdirAll() returned an error: %v", err)
	}

	// Check if directory was created
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Errorf("Failed to stat created directory: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("Created path is not a directory")
	}

	// Test creating nested directories
	nestedPath := filepath.Join(tempDir, "parent", "child", "grandchild")

	err = fs.MkdirAll(nestedPath, 0755)
	if err != nil {
		t.Errorf("MkdirAll() returned an error for nested directories: %v", err)
	}

	// Check if all directories were created
	info, err = os.Stat(nestedPath)
	if err != nil {
		t.Errorf("Failed to stat created nested directories: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("Created nested path is not a directory")
	}

	// Test idempotence (creating already existing directory)
	err = fs.MkdirAll(dirPath, 0755)
	if err != nil {
		t.Errorf("MkdirAll() returned an error when directory already exists: %v", err)
	}
}

func TestRemoveAll(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a directory with files and subdirectories
	testDir := filepath.Join(tempDir, "remove-test")

	// Create the directory structure
	err := os.MkdirAll(filepath.Join(testDir, "subdir"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory structure: %v", err)
	}

	// Create some files
	err = os.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(filepath.Join(testDir, "subdir", "file2.txt"), []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file in subdirectory: %v", err)
	}

	fs := NewFileSystem()

	// Test removing the directory
	err = fs.RemoveAll(testDir)
	if err != nil {
		t.Errorf("RemoveAll() returned an error: %v", err)
	}

	// Check that the directory was removed
	_, err = os.Stat(testDir)
	if !os.IsNotExist(err) {
		t.Errorf("Directory was not removed, still exists or got unexpected error: %v", err)
	}

	// Test idempotence (removing non-existent directory)
	err = fs.RemoveAll(testDir)
	if err != nil {
		t.Errorf("RemoveAll() returned an error for non-existent directory: %v", err)
	}

	// Test removing a single file
	singleFilePath := filepath.Join(tempDir, "single-file.txt")
	err = os.WriteFile(singleFilePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = fs.RemoveAll(singleFilePath)
	if err != nil {
		t.Errorf("RemoveAll() returned an error when removing a file: %v", err)
	}

	// Check that the file was removed
	_, err = os.Stat(singleFilePath)
	if !os.IsNotExist(err) {
		t.Errorf("File was not removed, still exists or got unexpected error: %v", err)
	}
}

// TestFileSystemIntegration performs an integration test of multiple FileSystem operations
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

	readContent, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Reading file content failed: %v", err)
	}

	if !bytes.Equal(readContent, content) {
		t.Errorf("File content does not match: expected '%s', got '%s'", content, readContent)
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
