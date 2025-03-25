package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileHashStorage_SaveAndGetHash(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "hash_storage_test")
	assert.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create storage with temp directory
	storage := NewFileHashStorageWithPath(tempDir)

	// Save a hash
	err = storage.SaveHash("target1", "job1", 0, "hash1")
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
	hashFilePath := filepath.Join(tempDir, "step_hashes.json")
	jsonData, err := os.ReadFile(hashFilePath)
	assert.NoError(t, err, "Failed to read hash file")

	var hashList []StepHash
	err = json.Unmarshal(jsonData, &hashList)
	assert.NoError(t, err, "Failed to parse hash file JSON")
	assert.Len(t, hashList, 1, "Expected 1 hash in file")
	if len(hashList) > 0 {
		assert.Equal(t, "hash1", hashList[0].Hash, "Stored hash doesn't match expected value")
	}
}

func TestFileHashStorage_Clear(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "hash_storage_test")
	assert.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create storage with temp directory
	storage := NewFileHashStorageWithPath(tempDir)

	// Save some test data
	err = storage.SaveHash("target1", "job1", 0, "hash1")
	assert.NoError(t, err, "Failed to save hash")

	// Clear the storage
	err = storage.Clear()
	assert.NoError(t, err, "Failed to clear hashes")

	// Verify directory is removed
	_, err = os.Stat(tempDir)
	assert.True(t, os.IsNotExist(err), "Storage directory should be removed")
}

func TestFileHashStorage_LoadFromFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "hash_storage_test")
	assert.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create storage with temp directory
	storage := NewFileHashStorageWithPath(tempDir)

	// Save some test data
	err = storage.SaveHash("target1", "job1", 0, "hash1")
	assert.NoError(t, err, "Failed to save first hash")
	err = storage.SaveHash("target2", "job2", 1, "hash2")
	assert.NoError(t, err, "Failed to save second hash")

	// Create a new storage instance to test loading from file
	newStorage := NewFileHashStorageWithPath(tempDir)

	// Get hashes to trigger loading from file
	hash, err := newStorage.GetHash("target1", "job1", 0)
	assert.NoError(t, err, "Failed to get first hash")
	assert.Equal(t, "hash1", hash, "Retrieved hash doesn't match expected value")

	hash, err = newStorage.GetHash("target2", "job2", 1)
	assert.NoError(t, err, "Failed to get second hash")
	assert.Equal(t, "hash2", hash, "Retrieved hash doesn't match expected value")
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
