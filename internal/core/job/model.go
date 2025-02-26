package job

// Job represents a collection of steps to be executed on targets.
type Job struct {
	Name  string  `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`
	Steps []*Step `yaml:"steps" json:"steps" validate:"required,dive"`
}

// Step defines a single deployment action that can be either
// a command execution, file copy operation, or Docker operation.
type Step struct {
	Run    string      `yaml:"run,omitempty" json:"run,omitempty" validate:"required_without_all=Copy Shell Docker"`
	Copy   *CopyStep   `yaml:"copy,omitempty" json:"copy,omitempty" validate:"required_without_all=Run Shell Docker"`
	Shell  string      `yaml:"shell,omitempty" json:"shell,omitempty"`
	Docker *DockerStep `yaml:"docker,omitempty" json:"docker,omitempty" validate:"required_without_all=Run Copy Shell"`
}

// DockerStep defines Docker container configuration and execution parameters.
type DockerStep struct {
	Image       string            `yaml:"image" json:"image" validate:"required"`
	Name        string            `yaml:"name" json:"name" validate:"required"`
	Environment map[string]string `yaml:"environment" json:"environment" validate:"omitempty"`
	Ports       []string          `yaml:"ports" json:"ports" validate:"omitempty,dive,required"`
	Volumes     []string          `yaml:"volumes" json:"volumes" validate:"omitempty,dive,required"`
	Labels      map[string]string `yaml:"labels" json:"labels" validate:"omitempty"`
	Networks    []string          `yaml:"networks" json:"networks" validate:"omitempty,dive,required"`
	Commands    []string          `yaml:"commands" json:"commands" validate:"omitempty,dive,required"`
	Restart     string            `yaml:"restart" json:"restart" validate:"omitempty,oneof=no on-failure always unless-stopped"`
}

// CopyStep defines source and destination paths for file copy operations.
type CopyStep struct {
	Src     string   `yaml:"src" json:"src" validate:"required"`
	Dst     string   `yaml:"dst" json:"dst" validate:"required"`
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty" validate:"omitempty,dive,required"`
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
