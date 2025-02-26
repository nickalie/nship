package job

import (
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
			if got := tt.step.GetShell(); got != tt.expectedShell {
				t.Errorf("Step.GetShell() = %v, want %v", got, tt.expectedShell)
			}
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
					Src: "source.txt",
					Dst: "dest.txt",
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
			if got := tt.step.GetType(); got != tt.expectedType {
				t.Errorf("Step.GetType() = %v, want %v", got, tt.expectedType)
			}
		})
	}
}

func TestPanicOnInvalidStepType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("GetType() should panic on invalid step type")
		}
	}()

	step := Step{}
	step.GetType()
}
