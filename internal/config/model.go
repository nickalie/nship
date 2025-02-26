// Package config provides functionality for creating and managing deployment configurations.
package config

import (
	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
)

// Config represents the main deployment configuration structure containing
// targets and jobs definitions.
type Config struct {
	Targets []*target.Target `yaml:"targets" json:"targets" validate:"required,dive"`
	Jobs    []*job.Job       `yaml:"jobs" json:"jobs" validate:"required,dive"`
}
