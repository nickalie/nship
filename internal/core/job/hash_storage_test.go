package job

import (
	"testing"
)

func TestStepHasher_ComputeHash(t *testing.T) {
	hasher := NewStepHasher()

	// Test that identical steps have the same hash
	t.Run("identical steps have same hash", func(t *testing.T) {
		step1 := &Step{Run: "echo hello"}
		step2 := &Step{Run: "echo hello"}

		hash1, err := hasher.ComputeHash(step1)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(step2)
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

		hash1, err := hasher.ComputeHash(step1)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(step2)
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

		hashRun, err := hasher.ComputeHash(runStep)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hashCopy, err := hasher.ComputeHash(copyStep)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hashDocker, err := hasher.ComputeHash(dockerStep)
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

		hash, err := hasher.ComputeHash(complexStep)
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

		hash1, err := hasher.ComputeHash(step1)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		hash2, err := hasher.ComputeHash(step2)
		if err != nil {
			t.Fatalf("Failed to compute hash: %v", err)
		}

		if hash1 == hash2 {
			t.Errorf("Steps with different shells should have different hashes, both got %s", hash1)
		}
	})
}
