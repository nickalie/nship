package job

import (
	"fmt"

	"github.com/nickalie/nship/internal/core/target"
)

// Service handles the execution of jobs
type Service struct {
	clientFactory ClientFactory
	hashStorage   HashStorage
	stepHasher    StepHasherInterface
	skipUnchanged bool
}

// ServiceOption represents an option for configuring a Service
type ServiceOption func(*Service)

// WithHashStorage sets the hash storage to use
func WithHashStorage(storage HashStorage) ServiceOption {
	return func(s *Service) {
		s.hashStorage = storage
	}
}

// WithSkipUnchanged sets whether unchanged steps should be skipped
func WithSkipUnchanged(skip bool) ServiceOption {
	return func(s *Service) {
		s.skipUnchanged = skip
	}
}

// NewService creates a new Service with the given options
func NewService(clientFactory ClientFactory, opts ...ServiceOption) *Service {
	service := &Service{
		clientFactory: clientFactory,
		stepHasher:    NewStepHasher(),
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

// determineStepsToExecute returns a slice indicating which steps need execution
func (s *Service) determineStepsToExecute(tgt *target.Target, job *Job) ([]bool, error) {
	stepShouldExecute := make([]bool, len(job.Steps))
	var foundChange bool

	for i, step := range job.Steps {
		shouldExecute, err := s.shouldExecuteStep(tgt, job, i, step, false)
		if err != nil {
			return nil, err
		}
		if shouldExecute {
			foundChange = true
		}
		stepShouldExecute[i] = foundChange
	}
	return stepShouldExecute, nil
}

// executeRequiredSteps executes the steps marked as required
func (s *Service) executeRequiredSteps(client Client, tgt *target.Target, job *Job, stepShouldExecute []bool) error {
	for i, step := range job.Steps {
		if !stepShouldExecute[i] {
			continue
		}

		if err := client.ExecuteStep(step, i+1, len(job.Steps)); err != nil {
			return err
		}

		if s.hashStorage != nil {
			if err := s.storeStepHash(tgt, job, i, step); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExecuteJob executes a job on a target
func (s *Service) ExecuteJob(tgt *target.Target, job *Job) error {
	client, err := s.clientFactory.NewClient(tgt)
	if err != nil {
		return err
	}
	defer client.Close()

	stepShouldExecute, err := s.determineStepsToExecute(tgt, job)
	if err != nil {
		return err
	}

	return s.executeRequiredSteps(client, tgt, job, stepShouldExecute)
}

// ExecuteJobs executes multiple jobs on multiple targets
func (s *Service) ExecuteJobs(targets []*target.Target, jobs []*Job) error {
	for _, tgt := range targets {
		for _, job := range jobs {
			if err := s.ExecuteJob(tgt, job); err != nil {
				return fmt.Errorf("failed to execute job %s on target %s: %w", job.Name, tgt.GetName(), err)
			}
		}
	}
	return nil
}

// storeStepHash stores the hash of a step
func (s *Service) storeStepHash(tgt *target.Target, job *Job, stepIndex int, step *Step) error {
	hash, err := s.stepHasher.ComputeHash(step, tgt)
	if err != nil {
		return fmt.Errorf("failed to compute step hash: %v", err)
	}

	if err := s.hashStorage.SaveHash(tgt.GetName(), job.Name, stepIndex, hash); err != nil {
		return fmt.Errorf("failed to save step hash: %v", err)
	}

	return nil
}

// shouldSkipExecution checks if step execution can be skipped based on service configuration
func (s *Service) shouldSkipExecution(forceExecute bool) bool {
	return !forceExecute && s.skipUnchanged && s.hashStorage != nil
}

// getStepHashes computes current hash and retrieves stored hash
func (s *Service) getStepHashes(tgt *target.Target, job *Job, stepIndex int, step *Step) (currentHash, storedHash string, err error) {
	currentHash, err = s.stepHasher.ComputeHash(step, tgt)
	if err != nil {
		return "", "", fmt.Errorf("failed to compute step hash: %w", err)
	}

	storedHash, err = s.hashStorage.GetHash(tgt.GetName(), job.Name, stepIndex)
	if err != nil {
		return "", "", fmt.Errorf("failed to get stored hash: %w", err)
	}

	return currentHash, storedHash, nil
}

// shouldExecuteStep determines if a step should be executed based on its hash
func (s *Service) shouldExecuteStep(tgt *target.Target, job *Job, stepIndex int, step *Step, forceExecute bool) (bool, error) {
	if !s.shouldSkipExecution(forceExecute) {
		return true, nil
	}

	currentHash, storedHash, err := s.getStepHashes(tgt, job, stepIndex, step)

	if err != nil {
		return true, err
	}

	shouldExecute := storedHash == "" || storedHash != currentHash
	if !shouldExecute {
		fmt.Printf("[%s] Skipping step %d in job '%s' (unchanged)\n", tgt.GetName(), stepIndex+1, job.Name)
	}

	return shouldExecute, nil
}

// ClearHashes clears all stored hashes
func (s *Service) ClearHashes() error {
	if s.hashStorage == nil {
		return nil
	}
	return s.hashStorage.Clear()
}
