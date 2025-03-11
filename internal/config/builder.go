package config

import (
	"encoding/json"
	"fmt"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

// Builder facilitates the construction of deployment configurations
// using a fluent interface pattern.
type Builder struct {
	config     *Config
	currentJob *job.Job
}

// NewBuilder creates and returns a new Builder instance with an initialized
// empty configuration.
func NewBuilder() *Builder {
	return &Builder{
		config: &Config{},
	}
}

// AddTarget appends a new target to the configuration and returns
// the builder for method chaining.
func (b *Builder) AddTarget(tgt *target.Target) *Builder {
	b.config.Targets = append(b.config.Targets, tgt)
	return b
}

// AddJob creates a new job with the specified name, adds it to the configuration,
// and sets it as the current job. Returns the builder for method chaining.
func (b *Builder) AddJob(name string) *Builder {
	newJob := &job.Job{
		Name: name,
	}
	b.config.Jobs = append(b.config.Jobs, newJob)
	b.currentJob = newJob
	return b
}

// AddStep adds the provided step to the current job and returns
// the builder for method chaining.
func (b *Builder) AddStep(step *job.Step) *Builder {
	b.currentJob.Steps = append(b.currentJob.Steps, step)
	return b
}

// AddRunStep creates and adds a new command execution step with the specified
// command to the current job. Returns the builder for method chaining.
func (b *Builder) AddRunStep(command string) *Builder {
	step := &job.Step{
		Run: command,
	}
	return b.AddStep(step)
}

// AddCopyStep creates and adds a new file copy step with the specified
// source and destination paths. Returns the builder for method chaining.
func (b *Builder) AddCopyStep(local, remote string) *Builder {
	step := &job.Step{
		Copy: &job.CopyStep{
			Local:  local,
			Remote: remote,
		},
	}
	return b.AddStep(step)
}

// AddDockerStep adds a new Docker execution step with the specified
// Docker configuration. Returns the builder for method chaining.
func (b *Builder) AddDockerStep(docker *job.DockerStep) *Builder {
	step := &job.Step{
		Docker: docker,
	}
	return b.AddStep(step)
}

// GetConfig returns the built configuration.
func (b *Builder) GetConfig() *Config {
	return b.config
}

// Print marshals the configuration to JSON and prints it to stdout.
// Returns an error if JSON marshaling fails.
func (b *Builder) Print() error {
	d, err := json.Marshal(b.config)
	if err != nil {
		return err
	}

	fmt.Println(string(d))
	return nil
}
