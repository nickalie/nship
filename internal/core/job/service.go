package job

import (
	"fmt"

	"github.com/nickalie/nship/internal/core/target"
)

// Service manages job execution across targets.
type Service struct {
	clientFactory ClientFactory
}

// NewService creates a new job service with the provided client factory.
func NewService(clientFactory ClientFactory) *Service {
	return &Service{
		clientFactory: clientFactory,
	}
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

	for i, step := range job.Steps {
		if err := client.ExecuteStep(step, i+1, len(job.Steps)); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}
	return nil
}
