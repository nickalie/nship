package fs

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err, "Failed to save hash")

	// Get the hash
	hash, err := storage.GetHash("target1", "job1", 0)
	assert.NoError(t, err, "Failed to get hash")
	assert.Equal(t, "hash1", hash, "Retrieved hash doesn't match expected value")

	// Try to get a non-existent hash
	hash, err = storage.GetHash("target1", "job1", 1)
	assert.NoError(t, err, "Getting non-existent hash should not error")
	assert.Empty(t, hash, "Expected empty hash for non-existent entry")

	// Verify the data was written to file
	expectedPath := filepath.Join(".test/hashes", "step_hashes.json")
	assert.Contains(t, mockHashData, expectedPath, "Hash file was not written")

	// Verify the file content
	jsonData := mockHashData[expectedPath]
	var hashList []StepHash
	err = json.Unmarshal(jsonData, &hashList)
	assert.NoError(t, err, "Failed to parse hash file JSON")

	assert.Len(t, hashList, 1, "Expected 1 hash in file")
	if len(hashList) > 0 {
		assert.Equal(t, "hash1", hashList[0].Hash, "Stored hash doesn't match expected value")
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
	assert.NoError(t, err, "Failed to clear hashes")
	assert.True(t, removed, "RemoveAll was not called during Clear()")
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
	assert.NoError(t, err, "Failed to get hash")
	assert.Equal(t, "hash1", hash, "Retrieved hash doesn't match expected value")

	// Get another hash to ensure all were loaded
	hash, err = storage.GetHash("target2", "job2", 1)
	assert.NoError(t, err, "Failed to get hash")
	assert.Equal(t, "hash2", hash, "Retrieved hash doesn't match expected value")
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
	assert.Error(t, err, "Expected error when reading hash file fails")

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
	assert.Error(t, err, "Expected error when writing hash file fails")

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
	assert.Error(t, err, "Expected error when creating directory fails")
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
		assert.Equal(t, tt.expected, key,
			"makeHashKey(%s, %s, %d) returned unexpected result",
			tt.targetName, tt.jobName, tt.stepIndex)
	}
}
