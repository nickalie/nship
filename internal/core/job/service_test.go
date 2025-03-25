package job

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStepHasher implements StepHasherInterface for testing
type MockStepHasher struct {
	ComputeHashFunc func(step *Step, tgt *target.Target) (string, error)
}

// ComputeHash implements the hashing functionality
func (m *MockStepHasher) ComputeHash(step *Step, tgt *target.Target) (string, error) {
	if m.ComputeHashFunc != nil {
		return m.ComputeHashFunc(step, tgt)
	}
	return "", errors.New("ComputeHash not implemented")
}

func TestNewServiceWithOptions(t *testing.T) {
	mockClientFactory := &MockClientFactory{}
	mockHashStorage := &MockHashStorage{}

	// Create service with options
	service := NewService(mockClientFactory,
		WithHashStorage(mockHashStorage),
		WithSkipUnchanged(false))

	// Verify options were applied
	assert.Equal(t, mockHashStorage, service.hashStorage, "HashStorage option was not applied")
	assert.Equal(t, false, service.skipUnchanged, "SkipUnchanged option was not applied")
}

func TestShouldExecuteStep(t *testing.T) {
	// Create step for testing
	step := &Step{Run: "echo test"}
	defaultHash := "hashed_step_value"

	// Create a mock StepHasher that always returns defaultHash
	mockStepHasher := &MockStepHasher{
		ComputeHashFunc: func(step *Step, tgt *target.Target) (string, error) {
			return defaultHash, nil
		},
	}

	tests := []struct {
		name           string
		forceExecute   bool
		skipUnchanged  bool
		hashStorage    HashStorage
		expectedResult bool
		expectErr      bool
	}{
		{
			name:           "force execute",
			forceExecute:   true,
			skipUnchanged:  true,
			hashStorage:    nil,
			expectedResult: true,
			expectErr:      false,
		},
		{
			name:           "skip unchanged disabled",
			forceExecute:   false,
			skipUnchanged:  false,
			hashStorage:    nil,
			expectedResult: true,
			expectErr:      false,
		},
		{
			name:           "no hash storage",
			forceExecute:   false,
			skipUnchanged:  true,
			hashStorage:    nil,
			expectedResult: true,
			expectErr:      false,
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
			expectErr:      false,
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
			expectErr:      false,
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
			expectErr:      false,
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
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service with test configuration
			service := &Service{
				stepHasher:    mockStepHasher,
				skipUnchanged: tt.skipUnchanged,
				hashStorage:   tt.hashStorage,
			}

			// Create target and job for testing
			tgt := &target.Target{Name: "test-target"}
			job := &Job{Name: "test-job"}

			// Call the function
			result, err := service.shouldExecuteStep(tgt, job, 0, step, tt.forceExecute)

			// Check result
			assert.Equal(t, tt.expectedResult, result,
				"shouldExecuteStep returned unexpected result")

			// Check error status
			if tt.expectErr {
				assert.Error(t, err, "Expected error but got nil")
			} else {
				assert.NoError(t, err, "Unexpected error")
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

	executedSteps := make(map[int]bool)
	mockClient := &MockClient{}
	mockClient.On("ExecuteStep", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			stepNum := args.Get(1).(int)
			executedSteps[stepNum-1] = true
		}).
		Return(nil)
	mockClient.On("Close").Return()

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
					hash, _ := (&StepHasher{}).ComputeHash(step, tgt)
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
					hash, _ := (&StepHasher{}).ComputeHash(step, tgt)
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
				hash, _ := (&StepHasher{}).ComputeHash(job.Steps[0], tgt)
				key := fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, 0)
				hashStore[key] = hash

				// Step 1 has different hash
				key = fmt.Sprintf("%s:%s:%d", tgt.GetName(), job.Name, 1)
				hashStore[key] = "different_hash"

				// Step 2 has matching hash, but we expect it to run due to step 1 change
				hash, _ = (&StepHasher{}).ComputeHash(job.Steps[2], tgt)
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
			assert.NoError(t, err, "ExecuteJob returned error")

			// Check which steps were executed
			for i := 0; i < len(job.Steps); i++ {
				expectedExecution := false
				for _, expectedStep := range tt.expectedSteps {
					if i == expectedStep {
						expectedExecution = true
						break
					}
				}

				assert.Equal(t, expectedExecution, executedSteps[i],
					"Step %d execution unexpected", i)
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
		assert.NoError(t, err, "ClearHashes should not return an error")
		assert.True(t, cleared, "Hash storage Clear() was not called")
	})

	t.Run("without hash storage", func(t *testing.T) {
		service := NewService(mockClientFactory)

		err := service.ClearHashes()
		assert.NoError(t, err, "ClearHashes should not return an error")
	})

	t.Run("with error", func(t *testing.T) {
		mockHashStorage := &MockHashStorage{
			ClearFunc: func() error {
				return errors.New("clear error")
			},
		}

		service := NewService(mockClientFactory, WithHashStorage(mockHashStorage))

		err := service.ClearHashes()
		assert.Error(t, err, "Expected error but got nil")
	})
}
