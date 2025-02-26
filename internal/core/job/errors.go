// Package job provides core functionality for defining and executing deployment jobs.
package job

import "fmt"

// ConnectionError represents an error that occurs when connecting to a target.
type ConnectionError struct {
	Target string
	Cause  error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection to target %s failed: %v", e.Target, e.Cause)
}

// StepError represents an error that occurs during step execution.
type StepError struct {
	JobName  string
	Target   string
	StepNum  int
	TotalNum int
	Cause    error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("job '%s' step %d/%d on '%s' failed: %v", e.JobName, e.StepNum, e.TotalNum, e.Target, e.Cause)
}

// CommandError represents an error that occurs when executing a command.
type CommandError struct {
	Command string
	Output  string
	Cause   error
}

func (e *CommandError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("command '%s' failed: %v\nOutput: %s", e.Command, e.Cause, e.Output)
	}
	return fmt.Sprintf("command '%s' failed: %v", e.Command, e.Cause)
}

// CopyError represents an error that occurs during file copying.
type CopyError struct {
	Source      string
	Destination string
	Cause       error
}

func (e *CopyError) Error() string {
	return fmt.Sprintf("copying '%s' to '%s' failed: %v", e.Source, e.Destination, e.Cause)
}

// DockerError represents an error that occurs during Docker operations.
type DockerError struct {
	ContainerName string
	Operation     string
	Cause         error
}

func (e *DockerError) Error() string {
	return fmt.Sprintf("Docker operation '%s' on container '%s' failed: %v", e.Operation, e.ContainerName, e.Cause)
}
