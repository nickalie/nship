package job

import (
	"fmt"

	"github.com/nickalie/nship/internal/core/target"
)

// Service manages job execution across targets.
type Service struct {
	clientFactory ClientFactory
	hashStorage   HashStorage
	fileSystem    FileSystemInterface
	stepHasher    StepHasherInterface
	skipUnchanged bool
}

// ServiceOption defines functional options for Service
type ServiceOption func(*Service)

// WithHashStorage sets the hash storage for the service
func WithHashStorage(storage HashStorage) ServiceOption {
	return func(s *Service) {
		s.hashStorage = storage
	}
}

// WithSkipUnchanged enables or disables skipping unchanged steps
func WithSkipUnchanged(skip bool) ServiceOption {
	return func(s *Service) {
		s.skipUnchanged = skip
	}
}

// NewService creates a new job service with the provided client factory.
func NewService(clientFactory ClientFactory, opts ...ServiceOption) *Service {
	service := &Service{
		clientFactory: clientFactory,
		stepHasher:    NewStepHasher(),
		// Default filesystem is nil, which disables source file hashing
		fileSystem:    nil,
		skipUnchanged: true, // Enable by default
	}

	// Apply options
	for _, opt := range opts {
		opt(service)
	}

	return service
}

// ExecuteJobs executes the specified jobs on all targets.
func (s *Service) ExecuteJobs(targets []*target.Target, jobs []*Job) error {
	for _, tgt := range targets {
		for _, job := range jobs {
			if err := s.ExecuteJob(tgt, job); err != nil {
				return fmt.Errorf("job '%s' failed on target '%s': %w", job.Name, tgt.GetName(), err)
			}
			fmt.Printf("Job '%s' completed successfully on target '%s'\n", job.Name, tgt.GetName())
		}
	}
	return nil
}

// ExecuteJob executes a single job on a target.
func (s *Service) ExecuteJob(tgt *target.Target, job *Job) error {
	fmt.Printf("Running job '%s' on target '%s'\n", job.Name, tgt.GetName())

	client, err := s.clientFactory.NewClient(tgt)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	var forceExecute bool

	for i, step := range job.Steps {
		if err := s.executeStep(client, tgt, job, i, step, &forceExecute); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) executeStep(client Client, tgt *target.Target, job *Job, stepIndex int, step *Step, forceExecute *bool) error {
	shouldExecute, err := s.shouldExecuteStep(tgt, job, stepIndex, step, *forceExecute)
	if err != nil {
		return fmt.Errorf("failed to check step hash: %w", err)
	}

	if !shouldExecute {
		fmt.Printf("Skipping step %d (unchanged)\n", stepIndex+1)
		return nil
	}

	if err := client.ExecuteStep(step, stepIndex+1, len(job.Steps)); err != nil {
		return fmt.Errorf("step %d failed: %w", stepIndex+1, err)
	}

	*forceExecute = true

	if s.hashStorage != nil {
		if err := s.storeStepHash(tgt, job, stepIndex, step); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	return nil
}

func (s *Service) storeStepHash(tgt *target.Target, job *Job, stepIndex int, step *Step) error {
	hash, err := s.stepHasher.ComputeHash(step, tgt, s.fileSystem)
	if err != nil {
		return fmt.Errorf("failed to compute step hash: %v", err)
	}

	if err := s.hashStorage.SaveHash(tgt.GetName(), job.Name, stepIndex, hash); err != nil {
		return fmt.Errorf("failed to save step hash: %v", err)
	}

	return nil
}

// shouldExecuteStep determines if a step should be executed based on its hash
func (s *Service) shouldExecuteStep(tgt *target.Target, job *Job, stepIndex int, step *Step, forceExecute bool) (bool, error) {
	// Always execute if any previous step was executed or if skipping is disabled
	if forceExecute || !s.skipUnchanged || s.hashStorage == nil {
		return true, nil
	}

	// Compute the current hash
	currentHash, err := s.stepHasher.ComputeHash(step, tgt, s.fileSystem)
	if err != nil {
		return true, fmt.Errorf("failed to compute step hash: %w", err)
	}

	// Get the stored hash
	storedHash, err := s.hashStorage.GetHash(tgt.GetName(), job.Name, stepIndex)
	if err != nil {
		return true, fmt.Errorf("failed to get stored hash: %w", err)
	}

	// If there's no stored hash or it's different, we should execute the step
	return storedHash == "" || storedHash != currentHash, nil
}

// ClearHashes clears all stored hashes
func (s *Service) ClearHashes() error {
	if s.hashStorage == nil {
		return nil
	}
	return s.hashStorage.Clear()
}
