package job

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetShell(t *testing.T) {
	tests := []struct {
		name          string
		step          Step
		expectedShell string
	}{
		{
			name: "with explicit shell",
			step: Step{
				Run:   "echo hello",
				Shell: "bash",
			},
			expectedShell: "bash",
		},
		{
			name: "with default shell",
			step: Step{
				Run: "echo hello",
			},
			expectedShell: "sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedShell, tt.step.GetShell(), "Shell should match expected value")
		})
	}
}

func TestGetType(t *testing.T) {
	tests := []struct {
		name         string
		step         Step
		expectedType StepType
	}{
		{
			name: "run step",
			step: Step{
				Run: "echo hello",
			},
			expectedType: RunStep,
		},
		{
			name: "copy step",
			step: Step{
				Copy: &CopyStep{
					Local:  "source.txt",
					Remote: "dest.txt",
				},
			},
			expectedType: CopyStepType,
		},
		{
			name: "docker step",
			step: Step{
				Docker: &DockerStep{
					Image: "nginx",
					Name:  "web-server",
				},
			},
			expectedType: DockerStepType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedType, tt.step.GetType(), "Step type should match expected type")
		})
	}
}

func TestPanicOnInvalidStepType(t *testing.T) {
	step := Step{}

	assert.Panics(t, func() {
		step.GetType()
	}, "GetType() should panic on invalid step type")
}
