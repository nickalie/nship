package fs

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// MockFileSystem for testing
type MockHashFileSystem struct {
	ReadFileFunc  func(string) ([]byte, error)
	WriteFileFunc func(string, []byte, os.FileMode) error
	MkdirAllFunc  func(string, os.FileMode) error
	RemoveAllFunc func(string) error
}

func (m *MockHashFileSystem) Stat(name string) (os.FileInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *MockHashFileSystem) Open(name string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (m *MockHashFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return nil, errors.New("not implemented")
}

func (m *MockHashFileSystem) ReadFile(name string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(name)
	}
	return nil, errors.New("ReadFile not implemented")
}

func (m *MockHashFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(name, data, perm)
	}
	return errors.New("WriteFile not implemented")
}

func (m *MockHashFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return errors.New("MkdirAll not implemented")
}

func (m *MockHashFileSystem) RemoveAll(path string) error {
	if m.RemoveAllFunc != nil {
		return m.RemoveAllFunc(path)
	}
	return errors.New("RemoveAll not implemented")
}

func TestFileHashStorage_SaveAndGetHash(t *testing.T) {
	// Mock data for testing
	mockHashData := make(map[string][]byte)
	mockFileSystem := &MockHashFileSystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			if data, ok := mockHashData[name]; ok {
				return data, nil
			}
			return nil, os.ErrNotExist
		},
		WriteFileFunc: func(name string, data []byte, perm os.FileMode) error {
			mockHashData[name] = data
			return nil
		},
		MkdirAllFunc: func(path string, perm os.FileMode) error {
			return nil
		},
	}

	// Create storage with mock filesystem
	storage := NewFileHashStorageWithPath(".test/hashes")
	storage.fileSystem = mockFileSystem

	// Save a hash
	err := storage.SaveHash("target1", "job1", 0, "hash1")
	if err != nil {
		t.Fatalf("Failed to save hash: %v", err)
	}

	// Get the hash
	hash, err := storage.GetHash("target1", "job1", 0)
	if err != nil {
		t.Fatalf("Failed to get hash: %v", err)
	}

	if hash != "hash1" {
		t.Errorf("Expected hash to be 'hash1', got '%s'", hash)
	}

	// Try to get a non-existent hash
	hash, err = storage.GetHash("target1", "job1", 1)
	if err != nil {
		t.Fatalf("Getting non-existent hash should not error: %v", err)
	}

	if hash != "" {
		t.Errorf("Expected empty hash for non-existent entry, got '%s'", hash)
	}

	// Verify the data was written to file
	expectedPath := filepath.Join(".test/hashes", "step_hashes.json")
	if _, ok := mockHashData[expectedPath]; !ok {
		t.Error("Hash file was not written")
	}

	// Verify the file content
	jsonData := mockHashData[expectedPath]
	var hashList []StepHash
	if err := json.Unmarshal(jsonData, &hashList); err != nil {
		t.Fatalf("Failed to parse hash file JSON: %v", err)
	}

	if len(hashList) != 1 {
		t.Errorf("Expected 1 hash in file, got %d", len(hashList))
	}

	if len(hashList) > 0 && hashList[0].Hash != "hash1" {
		t.Errorf("Expected stored hash to be 'hash1', got '%s'", hashList[0].Hash)
	}
}

func TestFileHashStorage_Clear(t *testing.T) {
	removed := false
	// Mock fileSystem where RemoveAll records that it was called
	mockFileSystem := &MockHashFileSystem{
		RemoveAllFunc: func(path string) error {
			removed = true
			return nil
		},
	}

	// Create storage with mock filesystem
	storage := NewFileHashStorageWithPath(".test/hashes")
	storage.fileSystem = mockFileSystem

	// Since we can't easily check in-memory state, we'll just verify that
	// RemoveAll was called
	err := storage.Clear()
	if err != nil {
		t.Fatalf("Failed to clear hashes: %v", err)
	}

	if !removed {
		t.Error("RemoveAll was not called during Clear()")
	}
}

func TestFileHashStorage_LoadFromFile(t *testing.T) {
	// Create sample hash data
	hashList := []StepHash{
		{TargetName: "target1", JobName: "job1", StepIndex: 0, Hash: "hash1"},
		{TargetName: "target2", JobName: "job2", StepIndex: 1, Hash: "hash2"},
	}

	hashData, _ := json.MarshalIndent(hashList, "", "  ")

	// Mock fileSystem that returns our sample data
	mockFileSystem := &MockHashFileSystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return hashData, nil
		},
	}

	// Create storage with mock filesystem
	storage := NewFileHashStorageWithPath(".test/hashes")
	storage.fileSystem = mockFileSystem

	// Get a hash to trigger loading from file
	hash, err := storage.GetHash("target1", "job1", 0)
	if err != nil {
		t.Fatalf("Failed to get hash: %v", err)
	}

	if hash != "hash1" {
		t.Errorf("Expected hash to be 'hash1', got '%s'", hash)
	}

	// Get another hash to ensure all were loaded
	hash, err = storage.GetHash("target2", "job2", 1)
	if err != nil {
		t.Fatalf("Failed to get hash: %v", err)
	}

	if hash != "hash2" {
		t.Errorf("Expected hash to be 'hash2', got '%s'", hash)
	}
}

func TestFileHashStorage_FileErrors(t *testing.T) {
	fileErr := errors.New("file error")

	// Mock fileSystem that returns errors
	mockFileSystem := &MockHashFileSystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, fileErr
		},
		WriteFileFunc: func(name string, data []byte, perm os.FileMode) error {
			return fileErr
		},
		MkdirAllFunc: func(path string, perm os.FileMode) error {
			return fileErr
		},
		RemoveAllFunc: func(path string) error {
			return fileErr
		},
	}

	// Create storage with mock filesystem
	storage := NewFileHashStorageWithPath(".test/hashes")
	storage.fileSystem = mockFileSystem

	// Test error handling for Read
	_, err := storage.GetHash("target1", "job1", 0)
	if err == nil {
		t.Error("Expected error when reading hash file fails, got nil")
	}

	// Reset loaded flag
	storage.loaded = false

	// Test error handling for Write
	// First mock the read to return not found so we can get to write
	storage.fileSystem = &MockHashFileSystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
		WriteFileFunc: func(name string, data []byte, perm os.FileMode) error {
			return fileErr
		},
		MkdirAllFunc: func(path string, perm os.FileMode) error {
			return nil
		},
	}

	err = storage.SaveHash("target1", "job1", 0, "hash1")
	if err == nil {
		t.Error("Expected error when writing hash file fails, got nil")
	}

	// Test error handling for MkdirAll
	storage.fileSystem = &MockHashFileSystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
		MkdirAllFunc: func(path string, perm os.FileMode) error {
			return fileErr
		},
	}

	err = storage.SaveHash("target1", "job1", 0, "hash1")
	if err == nil {
		t.Error("Expected error when creating directory fails, got nil")
	}
}

func TestMakeHashKey(t *testing.T) {
	tests := []struct {
		targetName string
		jobName    string
		stepIndex  int
		expected   string
	}{
		{"target1", "job1", 0, "target1:job1:0"},
		{"target2", "job2", 1, "target2:job2:1"},
		{"", "", 0, "::0"},
	}

	for _, tt := range tests {
		key := makeHashKey(tt.targetName, tt.jobName, tt.stepIndex)
		if key != tt.expected {
			t.Errorf("makeHashKey(%s, %s, %d) = %s, want %s",
				tt.targetName, tt.jobName, tt.stepIndex, key, tt.expected)
		}
	}
}
