// Package config provides functionality for creating and managing deployment configurations.
// It includes a builder pattern implementation for constructing configurations programmatically,
// along with support for loading configurations from various file formats including YAML,
// TypeScript, JavaScript, and Go.
package config

import (
	"encoding/json"
	"fmt"
)

// Builder facilitates the construction of deployment configurations
// using a fluent interface pattern.
type Builder struct {
	config     *Config
	currentJob *Job
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
func (c *Builder) AddTarget(target *Target) *Builder {
	c.config.Targets = append(c.config.Targets, target)
	return c
}

// AddJob creates a new job with the specified name, adds it to the configuration,
// and sets it as the current job. Returns the builder for method chaining.
func (c *Builder) AddJob(name string) *Builder {
	job := &Job{
		Name: name,
	}
	c.config.Jobs = append(c.config.Jobs, job)
	c.currentJob = job
	return c
}

// AddStep adds the provided step to the current job and returns
// the builder for method chaining.
func (c *Builder) AddStep(step *Step) *Builder {
	c.currentJob.Steps = append(c.currentJob.Steps, step)
	return c
}

// AddRunStep creates and adds a new command execution step with the specified
// command to the current job. Returns the builder for method chaining.
func (c *Builder) AddRunStep(command string) *Builder {
	step := &Step{
		Run: command,
	}
	return c.AddStep(step)
}

// AddCopyStep creates and adds a new file copy step with the specified
// source and destination paths. Returns the builder for method chaining.
func (c *Builder) AddCopyStep(src, dst string) *Builder {
	step := &Step{
		Copy: &CopyStep{
			Src: src,
			Dst: dst,
		},
	}
	return c.AddStep(step)
}

// AddDockerStep adds a new Docker execution step with the specified
// Docker configuration. Returns the builder for method chaining.
func (c *Builder) AddDockerStep(docker *DockerStep) *Builder {
	step := &Step{
		Docker: docker,
	}
	return c.AddStep(step)
}

// Print marshals the configuration to JSON and prints it to stdout.
// Returns an error if JSON marshaling fails.
func (c *Builder) Print() error {
	d, err := json.Marshal(c.config)

	if err != nil {
		return err
	}

	fmt.Println(string(d))
	return nil
}
