package job

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/mock"
	"os"
	"testing"

	"github.com/nickalie/nship/internal/core/target"
)

func TestNewServiceWithOptions(t *testing.T) {
	mockClientFactory := &MockClientFactory{}
	mockHashStorage := &MockHashStorage{}

	// Create service with options
	service := NewService(mockClientFactory,
		WithHashStorage(mockHashStorage),
		WithSkipUnchanged(false))

	// Verify options were applied
	if service.hashStorage != mockHashStorage {
		t.Error("HashStorage option was not applied")
	}

	if service.skipUnchanged != false {
		t.Error("SkipUnchanged option was not applied")
	}
}

func TestShouldExecuteStep(t *testing.T) {
	// Create step for testing
	step := &Step{Run: "echo test"}
	defaultHash := "hashed_step_value"

	// Create mock filesystem
	mockFS := &MockFileSystemForHashing{
		StatFunc: func(name string) (os.FileInfo, error) {
			return &MockFileInfoForHashing{}, nil
		},
	}

	tests := []struct {
		name           string
		forceExecute   bool
		skipUnchanged  bool
		hashStorage    HashStorage
		expectedResult bool
	}{
		{
			name:           "force execute",
			forceExecute:   true,
			skipUnchanged:  true,
			hashStorage:    nil,
			expectedResult: true,
		},
		{
			name:           "skip unchanged disabled",
			forceExecute:   false,
			skipUnchanged:  false,
			hashStorage:    nil,
			expectedResult: true,
		},
		{
			name:           "no hash storage",
			forceExecute:   false,
			skipUnchanged:  true,
			hashStorage:    nil,
			expectedResult: true,
		},
		{
			name:          "stored hash matches",
			forceExecute:  false,
			skipUnchanged: true,
			hashStorage: &MockHashStorage{
				GetHashFunc: func(targetName, jobName string, stepIndex int) (string, error) {
					return defaultHash, nil
				},
			},
			expectedResult: false, // Should skip due to matching hash
		},
		{
			name:          "stored hash differs",
			forceExecute:  false,
			skipUnchanged: true,
			hashStorage: &MockHashStorage{
				GetHashFunc: func(targetName, jobName string, stepIndex int) (string, error) {
					return "different_hash", nil
				},
			},
			expectedResult: true, // Should execute due to different hash
		},
		{
			name:          "no stored hash",
			forceExecute:  false,
			skipUnchanged: true,
			hashStorage: &MockHashStorage{
				GetHashFunc: func(targetName, jobName string, stepIndex int) (string, error) {
					return "", nil
				},
			},
			expectedResult: true, // Should execute when no hash is stored
		},
		{
			name:          "hash storage error",
			forceExecute:  false,
			skipUnchanged: true,
			hashStorage: &MockHashStorage{
				GetHashFunc: func(targetName, jobName string, stepIndex int) (string, error) {
					return "", errors.New("hash storage error")
				},
			},
			expectedResult: true, // Should execute on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service with test configuration
			service := &Service{
				stepHasher:    NewStepHasher(),
				skipUnchanged: tt.skipUnchanged,
				hashStorage:   tt.hashStorage,
				fileSystem:    mockFS,
			}

			// Create target and job for testing
			tgt := &target.Target{Name: "test-target"}
			job := &Job{Name: "test-job"}

			// Call the function
			result, err := service.shouldExecuteStep(tgt, job, 0, step, tt.forceExecute)

			// Check result
			if result != tt.expectedResult {
				t.Errorf("Expected shouldExecuteStep to return %v, got %v", tt.expectedResult, result)
			}

			// If an error is expected, verify it was returned
			if tt.hashStorage != nil && tt.name == "hash storage error" && err == nil {
				t.Error("Expected error but got nil")
			}
		})
	}
}

func TestStepSkipping(t *testing.T) {
	// Create a target and job with steps
	tgt := &target.Target{
		Name:     "test-target",
		Host:     "localhost",
		User:     "user",
		Password: "password",
	}

	job := &Job{
		Name: "test-job",
		Steps: []*Step{
			{Run: "echo step1"},
			{Run: "echo step2"},
			{Run: "echo step3"},
		},
	}

	// Create a mock filesystem for hashing
	mockFS := &MockFileSystemForHashing{
		StatFunc: func(name string) (os.FileInfo, error) {
			return &MockFileInfoForHashing{}, nil
		},
	}

	executedSteps := make(map[int]bool)
	mockClient := &MockClient{}
	mockClient.On("ExecuteStep", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			stepNum := args.Get(1).(int)
			executedSteps[stepNum-1] = true
		}).
		Return(nil)

	// Create a client factory that returns our mock client
	mockClientFactory := &MockClientFactory{}
	mockClientFactory.On("NewClient", mock.Anything).Return(mockClient, nil)

	// Create a mock hash storage that returns specific hashes
	hashStore := make(map[string]string)
	mockHashStorage := &MockHashStorage{
		SaveHashFunc: func(targetName, jobName string, stepIndex int, hash string) error {
			key := fmt.Sprintf("%s:%s:%d", targetName, jobName, stepIndex)
			hashStore[key] = hash
			return nil
		},
		GetHashFunc: func(targetName, jobName string, stepIndex int) (string, error) {
			key := fmt.Sprintf("%s:%s:%d", targetName, jobName, stepIndex)
			return hashStore[key], nil
		},
	}

	// Test cases
	tests := []struct {
		name          string
		setupHashes   func()
		expectedSteps []int
		skipUnchanged bool
	}{
		{
			name: "execute all steps when no hashes stored",
			setupHashes: func() {
				hashStore = make(map[string]string)
			},
			expectedSteps: []int{0, 1, 2},
			skipUnchanged: true,
		},
		{
			name: "execute all steps when skipping disabled",
			setupHashes: func() {
				// Set up stored hashes for all steps
				hashStore = make(map[string]string)
				for i, step := range job.Steps {
					hash, _ := (&StepHasher{}).ComputeHash(step, mockFS)
					key := fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, i)
					hashStore[key] = hash
				}
			},
			expectedSteps: []int{0, 1, 2},
			skipUnchanged: false,
		},
		{
			name: "skip all steps when hashes match",
			setupHashes: func() {
				// Set up stored hashes for all steps
				hashStore = make(map[string]string)
				for i, step := range job.Steps {
					hash, _ := (&StepHasher{}).ComputeHash(step, mockFS)
					key := fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, i)
					hashStore[key] = hash
				}
			},
			expectedSteps: []int{},
			skipUnchanged: true,
		},
		{
			name: "execute from changed step onwards",
			setupHashes: func() {
				// Set up stored hashes for steps 0 and 2, with step 1 being changed
				hashStore = make(map[string]string)

				// Step 0 has matching hash
				hash, _ := (&StepHasher{}).ComputeHash(job.Steps[0], mockFS)
				key := fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, 0)
				hashStore[key] = hash

				// Step 1 has different hash
				key = fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, 1)
				hashStore[key] = "different_hash"

				// Step 2 has matching hash, but we expect it to run due to step 1 change
				hash, _ = (&StepHasher{}).ComputeHash(job.Steps[2], mockFS)
				key = fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, 2)
				hashStore[key] = hash
			},
			expectedSteps: []int{1, 2},
			skipUnchanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up hash state
			tt.setupHashes()

			// Reset tracking
			executedSteps = make(map[int]bool)

			// Create service with test configuration
			service := NewService(mockClientFactory,
				WithHashStorage(mockHashStorage),
				WithSkipUnchanged(tt.skipUnchanged))

			// Execute the job
			err := service.ExecuteJob(tgt, job)

			// Check for errors
			if err != nil {
				t.Fatalf("ExecuteJob returned error: %v", err)
			}

			// Check which steps were executed
			for i := 0; i < len(job.Steps); i++ {
				expectedExecution := false
				for _, expectedStep := range tt.expectedSteps {
					if i == expectedStep {
						expectedExecution = true
						break
					}
				}

				if executedSteps[i] != expectedExecution {
					t.Errorf("Step %d execution: got %v, expected %v",
						i, executedSteps[i], expectedExecution)
				}
			}
		})
	}
}

func TestClearHashes(t *testing.T) {
	mockClientFactory := &MockClientFactory{}

	t.Run("with hash storage", func(t *testing.T) {
		cleared := false
		mockHashStorage := &MockHashStorage{
			ClearFunc: func() error {
				cleared = true
				return nil
			},
		}

		service := NewService(mockClientFactory, WithHashStorage(mockHashStorage))

		err := service.ClearHashes()
		if err != nil {
			t.Errorf("ClearHashes returned error: %v", err)
		}

		if !cleared {
			t.Error("Hash storage Clear() was not called")
		}
	})

	t.Run("without hash storage", func(t *testing.T) {
		service := NewService(mockClientFactory)

		err := service.ClearHashes()
		if err != nil {
			t.Errorf("ClearHashes returned error: %v", err)
		}
	})

	t.Run("with error", func(t *testing.T) {
		mockHashStorage := &MockHashStorage{
			ClearFunc: func() error {
				return errors.New("clear error")
			},
		}

		service := NewService(mockClientFactory, WithHashStorage(mockHashStorage))

		err := service.ClearHashes()
		if err == nil {
			t.Error("Expected error but got nil")
		}
	})
}
