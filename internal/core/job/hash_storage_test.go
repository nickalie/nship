package job

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// MockFileSystemForHashing implements FileSystemInterface for testing
type MockFileSystemForHashing struct {
	StatFunc    func(name string) (os.FileInfo, error)
	ReadDirFunc func(name string) ([]os.DirEntry, error)
}

func (m *MockFileSystemForHashing) Stat(name string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, fmt.Errorf("Stat not implemented")
}

func (m *MockFileSystemForHashing) ReadDir(name string) ([]os.DirEntry, error) {
	if m.ReadDirFunc != nil {
		return m.ReadDirFunc(name)
	}
	return nil, fmt.Errorf("ReadDir not implemented")
}

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

func TestStepHasher_ComputeHash(t *testing.T) {
	hasher := NewStepHasher()

	// Create a nil FileSystem for basic tests
	var nilFS FileSystemInterface = nil

	// Test that identical steps have the same hash
	t.Run("identical steps have same hash", func(t *testing.T) {
		step1 := &Step{Run: "echo hello"}
		step2 := &Step{Run: "echo hello"}

		hash1, err := hasher.ComputeHash(step1, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(step2, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		if hash1 != hash2 {
			t.Errorf("Identical steps should have the same hash, got %s and %s", hash1, hash2)
		}
	})

	// Test that different steps have different hashes
	t.Run("different steps have different hashes", func(t *testing.T) {
		step1 := &Step{Run: "echo hello"}
		step2 := &Step{Run: "echo world"}

		hash1, err := hasher.ComputeHash(step1, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(step2, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		if hash1 == hash2 {
			t.Errorf("Different steps should have different hashes, both got %s", hash1)
		}
	})

	// Test steps with different types
	t.Run("different step types have different hashes", func(t *testing.T) {
		runStep := &Step{Run: "echo hello"}
		copyStep := &Step{Copy: &CopyStep{Src: "src", Dst: "dst"}}
		dockerStep := &Step{Docker: &DockerStep{Image: "nginx", Name: "web"}}

		hashRun, err := hasher.ComputeHash(runStep, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hashCopy, err := hasher.ComputeHash(copyStep, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hashDocker, err := hasher.ComputeHash(dockerStep, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		if hashRun == hashCopy || hashRun == hashDocker || hashCopy == hashDocker {
			t.Errorf("Different step types should have different hashes")
		}
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
				Commands: []string{"nginx", "-g", "daemon off;"},
				Restart:  "always",
			},
		}

		hash, err := hasher.ComputeHash(complexStep, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash for complex step: %v", err)
		}

		if hash == "" {
			t.Error("Hash for complex step should not be empty")
		}
	})

	// Test that shell changes affect the hash
	t.Run("different shell produces different hash", func(t *testing.T) {
		step1 := &Step{Run: "echo hello", Shell: "sh"}
		step2 := &Step{Run: "echo hello", Shell: "bash"}

		hash1, err := hasher.ComputeHash(step1, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(step2, nilFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		if hash1 == hash2 {
			t.Errorf("Steps with different shells should have different hashes, both got %s", hash1)
		}
	})

	// Test file-based hashing for CopyStep
	t.Run("file content affects hash for CopyStep", func(t *testing.T) {
		// Create a mock filesystem
		fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		mockFS := &MockFileSystemForHashing{
			StatFunc: func(name string) (os.FileInfo, error) {
				// Return different file info depending on the path
				return &MockFileInfoForHashing{
					ModTimeFunc: func() time.Time { return fixedTime },
					SizeFunc:    func() int64 { return 100 },
					IsDirFunc:   func() bool { return false },
				}, nil
			},
		}

		// Create two identical copy steps
		copyStep1 := &Step{Copy: &CopyStep{Src: "src/file.txt", Dst: "dst/file.txt"}}
		copyStep2 := &Step{Copy: &CopyStep{Src: "src/file.txt", Dst: "dst/file.txt"}}

		// Compute hashes
		hash1, err := hasher.ComputeHash(copyStep1, mockFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(copyStep2, mockFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		// Hashes should be the same for identical files
		if hash1 != hash2 {
			t.Errorf("Identical copy steps should have the same hash, got %s and %s", hash1, hash2)
		}

		// Now create a mock with different file content
		mockFSModified := &MockFileSystemForHashing{
			StatFunc: func(name string) (os.FileInfo, error) {
				return &MockFileInfoForHashing{
					ModTimeFunc: func() time.Time { return fixedTime.Add(time.Hour) }, // Different time
					SizeFunc:    func() int64 { return 200 },                          // Different size
					IsDirFunc:   func() bool { return false },
				}, nil
			},
		}

		// Compute hash with different file content
		hash3, err := hasher.ComputeHash(copyStep1, mockFSModified)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		// Hashes should be different
		if hash1 == hash3 {
			t.Errorf("Copy steps with different file content should have different hashes")
		}
	})

	// Test directory-based hashing for CopyStep
	t.Run("directory content affects hash for CopyStep", func(t *testing.T) {
		fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a mock filesystem with a directory
		mockFS := &MockFileSystemForHashing{
			StatFunc: func(name string) (os.FileInfo, error) {
				if name == "src" {
					return &MockFileInfoForHashing{
						IsDirFunc: func() bool { return true },
					}, nil
				}
				return &MockFileInfoForHashing{
					ModTimeFunc: func() time.Time { return fixedTime },
					SizeFunc:    func() int64 { return 100 },
					IsDirFunc:   func() bool { return false },
				}, nil
			},
			ReadDirFunc: func(name string) ([]os.DirEntry, error) {
				return []os.DirEntry{
					&MockDirEntryForHashing{
						NameFunc:  func() string { return "file1.txt" },
						IsDirFunc: func() bool { return false },
					},
					&MockDirEntryForHashing{
						NameFunc:  func() string { return "file2.txt" },
						IsDirFunc: func() bool { return false },
					},
				}, nil
			},
		}

		// Create a copy step for a directory
		dirCopyStep := &Step{Copy: &CopyStep{Src: "src", Dst: "dst"}}

		// Compute hash
		hash1, err := hasher.ComputeHash(dirCopyStep, mockFS)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		// Create a mock with different directory content
		mockFSModified := &MockFileSystemForHashing{
			StatFunc: func(name string) (os.FileInfo, error) {
				if name == "src" {
					return &MockFileInfoForHashing{
						IsDirFunc: func() bool { return true },
					}, nil
				}
				return &MockFileInfoForHashing{
					ModTimeFunc: func() time.Time { return fixedTime },
					SizeFunc:    func() int64 { return 100 },
					IsDirFunc:   func() bool { return false },
				}, nil
			},
			ReadDirFunc: func(name string) ([]os.DirEntry, error) {
				return []os.DirEntry{
					&MockDirEntryForHashing{
						NameFunc:  func() string { return "file1.txt" },
						IsDirFunc: func() bool { return false },
					},
					&MockDirEntryForHashing{
						NameFunc:  func() string { return "file3.txt" }, // Different file
						IsDirFunc: func() bool { return false },
					},
				}, nil
			},
		}

		// Compute hash with different directory content
		hash2, err := hasher.ComputeHash(dirCopyStep, mockFSModified)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		// Hashes should be different
		if hash1 == hash2 {
			t.Errorf("Copy steps with different directory content should have different hashes")
		}
	})
}
