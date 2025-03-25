package job

import (
	"os"
	"testing"
	"time"

	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/assert"
)

// MockFileInfoForHashing implements os.FileInfo for testing
type MockFileInfoForHashing struct {
	NameFunc    func() string
	SizeFunc    func() int64
	ModeFunc    func() os.FileMode
	ModTimeFunc func() time.Time
	IsDirFunc   func() bool
	SysFunc     func() interface{}
}

func (m *MockFileInfoForHashing) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-file"
}

func (m *MockFileInfoForHashing) Size() int64 {
	if m.SizeFunc != nil {
		return m.SizeFunc()
	}
	return 100
}

func (m *MockFileInfoForHashing) Mode() os.FileMode {
	if m.ModeFunc != nil {
		return m.ModeFunc()
	}
	return 0644
}

func (m *MockFileInfoForHashing) ModTime() time.Time {
	if m.ModTimeFunc != nil {
		return m.ModTimeFunc()
	}
	return time.Now()
}

func (m *MockFileInfoForHashing) IsDir() bool {
	if m.IsDirFunc != nil {
		return m.IsDirFunc()
	}
	return false
}

func (m *MockFileInfoForHashing) Sys() interface{} {
	if m.SysFunc != nil {
		return m.SysFunc()
	}
	return nil
}

// MockDirEntryForHashing implements os.DirEntry for testing
type MockDirEntryForHashing struct {
	NameFunc  func() string
	IsDirFunc func() bool
	TypeFunc  func() os.FileMode
	InfoFunc  func() (os.FileInfo, error)
}

func (m *MockDirEntryForHashing) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-entry"
}

func (m *MockDirEntryForHashing) IsDir() bool {
	if m.IsDirFunc != nil {
		return m.IsDirFunc()
	}
	return false
}

func (m *MockDirEntryForHashing) Type() os.FileMode {
	if m.TypeFunc != nil {
		return m.TypeFunc()
	}
	return 0
}

func (m *MockDirEntryForHashing) Info() (os.FileInfo, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc()
	}
	return &MockFileInfoForHashing{}, nil
}

// createTestFileStructure creates real temporary files for testing
func createTestFileStructure(t *testing.T, content string) (string, func()) {
	tempDir, err := os.MkdirTemp("", "test-hash-*")
	assert.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Create a test file
	err = os.WriteFile(tempDir+"/test.txt", []byte(content), 0644)
	assert.NoError(t, err)

	// Create a subdirectory with a file
	err = os.MkdirAll(tempDir+"/subdir", 0755)
	assert.NoError(t, err)
	err = os.WriteFile(tempDir+"/subdir/subfile.txt", []byte(content), 0644)
	assert.NoError(t, err)

	return tempDir, cleanup
}

func TestStepHasher_ComputeHash(t *testing.T) {
	hasher := NewStepHasher()

	// Create a test target
	testTarget := &target.Target{
		Name:     "test-server",
		Host:     "10.0.0.1",
		User:     "admin",
		Password: "password123",
	}

	// Test that identical steps have the same hash
	t.Run("identical steps have same hash", func(t *testing.T) {
		step1 := &Step{Run: "echo hello"}
		step2 := &Step{Run: "echo hello"}

		hash1, err := hasher.ComputeHash(step1, testTarget)
		assert.NoError(t, err, "Failed to compute hash for step1")

		hash2, err := hasher.ComputeHash(step2, testTarget)
		assert.NoError(t, err, "Failed to compute hash for step2")

		assert.Equal(t, hash1, hash2, "Identical steps should have the same hash")
	})

	// Test that different steps have different hashes
	t.Run("different steps have different hashes", func(t *testing.T) {
		step1 := &Step{Run: "echo hello"}
		step2 := &Step{Run: "echo world"}

		hash1, err := hasher.ComputeHash(step1, testTarget)
		assert.NoError(t, err, "Failed to compute hash for step1")

		hash2, err := hasher.ComputeHash(step2, testTarget)
		assert.NoError(t, err, "Failed to compute hash for step2")

		assert.NotEqual(t, hash1, hash2, "Different steps should have different hashes")
	})

	// Test same steps but different targets have different hashes
	t.Run("same step with different targets has different hash", func(t *testing.T) {
		step := &Step{Run: "echo hello"}
		target1 := &target.Target{Name: "server1", Host: "10.0.0.1"}
		target2 := &target.Target{Name: "server2", Host: "10.0.0.2"}

		hash1, err := hasher.ComputeHash(step, target1)
		assert.NoError(t, err, "Failed to compute hash for step with target1")

		hash2, err := hasher.ComputeHash(step, target2)
		assert.NoError(t, err, "Failed to compute hash for step with target2")

		assert.NotEqual(t, hash1, hash2, "Same step with different targets should have different hashes")
	})

	// Test steps with different types
	t.Run("different step types have different hashes", func(t *testing.T) {
		tempDir, cleanup := createTestFileStructure(t, "test content")
		defer cleanup()

		runStep := &Step{Run: "echo hello"}
		copyStep := &Step{Copy: &CopyStep{Local: tempDir + "/test.txt", Remote: "remote/file.txt"}}
		dockerStep := &Step{Docker: &DockerStep{Image: "nginx", Name: "web"}}

		hashRun, err := hasher.ComputeHash(runStep, testTarget)
		assert.NoError(t, err, "Failed to compute hash for runStep")

		hashCopy, err := hasher.ComputeHash(copyStep, testTarget)
		assert.NoError(t, err, "Failed to compute hash for copyStep")

		hashDocker, err := hasher.ComputeHash(dockerStep, testTarget)
		assert.NoError(t, err, "Failed to compute hash for dockerStep")

		assert.NotEqual(t, hashRun, hashCopy, "Run and Copy steps should have different hashes")
		assert.NotEqual(t, hashRun, hashDocker, "Run and Docker steps should have different hashes")
		assert.NotEqual(t, hashCopy, hashDocker, "Copy and Docker steps should have different hashes")
	})

	// Test that complex steps can be hashed
	t.Run("complex step hashing", func(t *testing.T) {
		complexStep := &Step{
			Docker: &DockerStep{
				Image: "nginx",
				Name:  "web",
				Environment: map[string]string{
					"ENV1": "value1",
					"ENV2": "value2",
				},
				Ports: []string{"80:80", "443:443"},
				Volumes: []string{
					"vol1:/var/www",
					"vol2:/etc/nginx",
				},
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				Networks: []string{"net1", "net2"},
				Command:  []string{"nginx", "-g", "daemon off;"},
				Restart:  "always",
			},
		}

		hash, err := hasher.ComputeHash(complexStep, testTarget)
		assert.NoError(t, err, "Failed to compute hash for complex step")
		assert.NotEmpty(t, hash, "Hash for complex step should not be empty")
	})

	// Test that shell changes affect the hash
	t.Run("different shell produces different hash", func(t *testing.T) {
		step1 := &Step{Run: "echo hello", Shell: "sh"}
		step2 := &Step{Run: "echo hello", Shell: "bash"}

		hash1, err := hasher.ComputeHash(step1, testTarget)
		assert.NoError(t, err, "Failed to compute hash for step1")

		hash2, err := hasher.ComputeHash(step2, testTarget)
		assert.NoError(t, err, "Failed to compute hash for step2")

		assert.NotEqual(t, hash1, hash2, "Steps with different shells should have different hashes")
	})

	// Test file-based hashing for CopyStep
	t.Run("file content affects hash for CopyStep", func(t *testing.T) {
		tempDir1, cleanup1 := createTestFileStructure(t, "content1")
		defer cleanup1()
		tempDir2, cleanup2 := createTestFileStructure(t, "content2")
		defer cleanup2()

		// Create two identical copy steps pointing to different files
		copyStep1 := &Step{Copy: &CopyStep{Local: tempDir1 + "/test.txt", Remote: "remote/file.txt"}}
		copyStep2 := &Step{Copy: &CopyStep{Local: tempDir2 + "/test.txt", Remote: "remote/file.txt"}}

		// Compute hashes
		hash1, err := hasher.ComputeHash(copyStep1, testTarget)
		assert.NoError(t, err, "Failed to compute hash for copyStep1")

		hash2, err := hasher.ComputeHash(copyStep2, testTarget)
		assert.NoError(t, err, "Failed to compute hash for copyStep2")

		// Hashes should be different due to different file content
		assert.NotEqual(t, hash1, hash2, "Copy steps with different file content should have different hashes")
	})

	// Test directory-based hashing for CopyStep
	t.Run("directory content affects hash for CopyStep", func(t *testing.T) {
		tempDir1, cleanup1 := createTestFileStructure(t, "content1")
		defer cleanup1()
		tempDir2, cleanup2 := createTestFileStructure(t, "content2")
		defer cleanup2()

		// Create a copy step for a directory
		dirCopyStep1 := &Step{Copy: &CopyStep{Local: tempDir1, Remote: "remote"}}
		dirCopyStep2 := &Step{Copy: &CopyStep{Local: tempDir2, Remote: "remote"}}

		// Compute hash
		hash1, err := hasher.ComputeHash(dirCopyStep1, testTarget)
		assert.NoError(t, err, "Failed to compute hash for directory copy step")

		hash2, err := hasher.ComputeHash(dirCopyStep2, testTarget)
		assert.NoError(t, err, "Failed to compute hash for directory copy step with modified content")

		// Hashes should be different
		assert.NotEqual(t, hash1, hash2, "Copy steps with different directory content should have different hashes")
	})

	// Test exclude patterns affect the hash
	t.Run("exclude patterns affect hash for CopyStep", func(t *testing.T) {
		tempDir, cleanup := createTestFileStructure(t, "test content")
		defer cleanup()

		// Create two copy steps with different exclude patterns
		copyStep1 := &Step{Copy: &CopyStep{
			Local:   tempDir,
			Remote:  "remote",
			Exclude: []string{"*.log"},
		}}

		copyStep2 := &Step{Copy: &CopyStep{
			Local:   tempDir,
			Remote:  "remote",
			Exclude: []string{"*.log", "*.tmp"},
		}}

		// Compute hashes
		hash1, err := hasher.ComputeHash(copyStep1, testTarget)
		assert.NoError(t, err, "Failed to compute hash for copyStep1")

		hash2, err := hasher.ComputeHash(copyStep2, testTarget)
		assert.NoError(t, err, "Failed to compute hash for copyStep2")

		// Hashes should be different despite same files because exclude patterns differ
		assert.NotEqual(t, hash1, hash2, "Copy steps with different exclude patterns should have different hashes")

		// Create a copy step with same patterns but different order
		copyStep3 := &Step{Copy: &CopyStep{
			Local:   tempDir,
			Remote:  "remote",
			Exclude: []string{"*.tmp", "*.log"}, // Same patterns as copyStep2 but different order
		}}

		hash3, err := hasher.ComputeHash(copyStep3, testTarget)
		assert.NoError(t, err, "Failed to compute hash for copyStep3")

		// Hashes should be the same despite pattern order difference
		assert.Equal(t, hash2, hash3, "Copy steps with same exclude patterns in different order should have same hash")
	})
}
