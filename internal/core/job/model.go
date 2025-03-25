package job

// Job represents a collection of steps to be executed on targets.
type Job struct {
	Name  string  `yaml:"name,omitempty" json:"name,omitempty" toml:"name,omitempty" validate:"omitempty"`
	Steps []*Step `yaml:"steps" json:"steps" toml:"steps" validate:"required,dive"`
}

// Step defines a single deployment action that can be either
// a command execution, file copy operation, or Docker operation.
type Step struct {
	Run    string      `yaml:"run,omitempty" json:"run,omitempty" toml:"run,omitempty" validate:"required_without_all=Copy Shell Docker"`
	Copy   *CopyStep   `yaml:"copy,omitempty" json:"copy,omitempty" toml:"copy,omitempty" validate:"required_without_all=Run Shell Docker"`
	Shell  string      `yaml:"shell,omitempty" json:"shell,omitempty" toml:"shell,omitempty" validate:"omitempty"`
	Docker *DockerStep `yaml:"docker,omitempty" json:"docker,omitempty" toml:"docker,omitempty" validate:"required_without_all=Run Copy Shell"`
}

// DockerBuildStep defines Docker build configuration parameters.
type DockerBuildStep struct {
	Context string            `yaml:"context" json:"context" toml:"context" validate:"required"`
	Args    map[string]string `yaml:"args,omitempty" json:"args,omitempty" toml:"args,omitempty" validate:"omitempty"`
}

// DockerStep defines Docker container configuration and execution parameters.
type DockerStep struct {
	Image       string            `yaml:"image" json:"image" toml:"image" validate:"required"`
	Name        string            `yaml:"name" json:"name" toml:"name" validate:"required"`
	Build       *DockerBuildStep  `yaml:"build,omitempty" json:"build,omitempty" toml:"build,omitempty" validate:"omitempty"`
	Environment map[string]string `yaml:"environment" json:"environment" toml:"environment" validate:"omitempty"`
	Ports       []string          `yaml:"ports" json:"ports" toml:"ports" validate:"omitempty,dive,required"`
	Volumes     []string          `yaml:"volumes" json:"volumes" toml:"volumes" validate:"omitempty,dive,required"`
	Labels      map[string]string `yaml:"labels" json:"labels" toml:"labels" validate:"omitempty"`
	Networks    []string          `yaml:"networks" json:"networks" toml:"networks" validate:"omitempty,dive,required"`
	Command     []string          `yaml:"command" json:"command" toml:"command" validate:"omitempty,dive,required"`
	Restart     string            `yaml:"restart" json:"restart" toml:"restart" validate:"omitempty,oneof=no on-failure always unless-stopped"`
}

// CopyStep defines source and destination paths for file copy operations.
type CopyStep struct {
	Local   string   `yaml:"local" json:"local" toml:"local" validate:"required"`
	Remote  string   `yaml:"remote" json:"remote" toml:"remote" validate:"required"`
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty" toml:"exclude,omitempty" validate:"omitempty,dive,required"`
}

// GetShell returns the shell to use for command execution, defaulting to sh if not specified.
func (s *Step) GetShell() string {
	if s.Shell == "" {
		return "sh"
	}
	return s.Shell
}

// StepType represents the type of deployment step.
type StepType int

const (
	// RunStep represents a command execution step.
	RunStep StepType = iota
	// CopyStepType represents a file copy operation step.
	CopyStepType
	// DockerStepType represents a Docker container operation step.
	DockerStepType
)

// GetType returns the type of step.
func (s *Step) GetType() StepType {
	switch {
	case s.Run != "":
		return RunStep
	case s.Copy != nil:
		return CopyStepType
	case s.Docker != nil:
		return DockerStepType
	default:
		// This shouldn't happen if validation is working properly
		panic("invalid step: no type detected")
	}
}
