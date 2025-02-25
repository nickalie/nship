package config

import (
	"encoding/json"
	"fmt"
)

type Builder struct {
	config     *Config
	currentJob *Job
}

func NewBuilder() *Builder {
	return &Builder{
		config: &Config{},
	}
}

func (c *Builder) AddTarget(target *Target) *Builder {
	c.config.Targets = append(c.config.Targets, target)
	return c
}

func (c *Builder) AddJob(name string) *Builder {
	job := &Job{
		Name: name,
	}
	c.config.Jobs = append(c.config.Jobs, job)
	c.currentJob = job
	return c
}

func (c *Builder) AddStep(step *Step) *Builder {
	c.currentJob.Steps = append(c.currentJob.Steps, step)
	return c
}

func (c *Builder) AddRunStep(command string) *Builder {
	step := &Step{
		Run: command,
	}
	return c.AddStep(step)
}

func (c *Builder) AddCopyStep(src, dst string) *Builder {
	step := &Step{
		Copy: &CopyStep{
			Src: src,
			Dst: dst,
		},
	}
	return c.AddStep(step)
}

func (c *Builder) AddDockerStep(docker *DockerStep) *Builder {
	step := &Step{
		Docker: docker,
	}
	return c.AddStep(step)
}

func (c *Builder) Print() error {
	d, err := json.Marshal(c.config)

	if err != nil {
		return err
	}

	fmt.Println(string(d))
	return nil
}
