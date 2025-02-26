// Package job provides core functionality for defining and executing deployment jobs.
package job

import (
	"github.com/nickalie/nship/internal/core/target"
)

// Client defines the interface for executing deployment steps
type Client interface {
	// ExecuteStep executes a single step with progress information
	ExecuteStep(step *Step, stepNum, totalSteps int) error
	// Close releases all resources associated with the client
	Close()
}

// ClientFactory creates remote clients
type ClientFactory interface {
	NewClient(target *target.Target) (Client, error)
}
