package job

import (
	"errors"
	"strings"
	"testing"

	"github.com/nickalie/nship/internal/core/target"
)

// MockClient implements the Client interface for testing
type MockClient struct {
	ExecuteStepFunc func(step *Step, stepNum, totalSteps int) error
	CloseFunc       func()
	closed          bool
}

func (m *MockClient) ExecuteStep(step *Step, stepNum, totalSteps int) error {
	if m.ExecuteStepFunc != nil {
		return m.ExecuteStepFunc(step, stepNum, totalSteps)
	}
	return nil
}

func (m *MockClient) Close() {
	m.closed = true
	if m.CloseFunc != nil {
		m.CloseFunc()
	}
}

// MockClientFactory implements the ClientFactory interface for testing
type MockClientFactory struct {
	NewClientFunc func(target *target.Target) (Client, error)
	clients       []Client
}

func (m *MockClientFactory) NewClient(target *target.Target) (Client, error) {
	if m.NewClientFunc != nil {
		client, err := m.NewClientFunc(target)
		if err == nil && client != nil {
			m.clients = append(m.clients, client)
		}
		return client, err
	}
	mockClient := &MockClient{}
	m.clients = append(m.clients, mockClient)
	return mockClient, nil
}

func TestExecuteJob(t *testing.T) {
	tests := []struct {
		name          string
		job           *Job
		target        *target.Target
		clientFactory func() *MockClientFactory
		wantErr       bool
		errMessage    string
	}{
		{
			name: "successful job execution",
			job: &Job{
				Name: "test-job",
				Steps: []*Step{
					{Run: "echo hello"},
					{Run: "echo world"},
				},
			},
			target: &target.Target{
				Name: "test-target",
				Host: "localhost",
				User: "user",
			},
			clientFactory: func() *MockClientFactory {
				return &MockClientFactory{
					NewClientFunc: func(target *target.Target) (Client, error) {
						return &MockClient{
							ExecuteStepFunc: func(step *Step, stepNum, totalSteps int) error {
								return nil
							},
						}, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "client creation failure",
			job: &Job{
				Name: "test-job",
				Steps: []*Step{
					{Run: "echo hello"},
				},
			},
			target: &target.Target{
				Name: "test-target",
				Host: "localhost",
				User: "user",
			},
			clientFactory: func() *MockClientFactory {
				return &MockClientFactory{
					NewClientFunc: func(target *target.Target) (Client, error) {
						return nil, errors.New("connection failed")
					},
				}
			},
			wantErr:    true,
			errMessage: "failed to create client",
		},
		{
			name: "step execution failure",
			job: &Job{
				Name: "test-job",
				Steps: []*Step{
					{Run: "echo hello"},
					{Run: "failing command"},
				},
			},
			target: &target.Target{
				Name: "test-target",
				Host: "localhost",
				User: "user",
			},
			clientFactory: func() *MockClientFactory {
				return &MockClientFactory{
					NewClientFunc: func(target *target.Target) (Client, error) {
						return &MockClient{
							ExecuteStepFunc: func(step *Step, stepNum, totalSteps int) error {
								if step.Run == "failing command" {
									return errors.New("command failed")
								}
								return nil
							},
						}, nil
					},
				}
			},
			wantErr:    true,
			errMessage: "step 2 failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := tt.clientFactory()
			service := NewService(factory)

			err := service.ExecuteJob(tt.target, tt.job)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMessage != "" {
				if !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf("ExecuteJob() error message = %v, want to contain %v", err.Error(), tt.errMessage)
				}
			}

			// Check that clients are closed
			for _, client := range factory.clients {
				if mockClient, ok := client.(*MockClient); ok {
					if !mockClient.closed {
						t.Errorf("Client not closed after ExecuteJob()")
					}
				}
			}
		})
	}
}

func TestExecuteJobs(t *testing.T) {
	targets := []*target.Target{
		{Name: "target1", Host: "host1", User: "user1"},
		{Name: "target2", Host: "host2", User: "user2"},
	}

	jobs := []*Job{
		{Name: "job1", Steps: []*Step{{Run: "echo job1"}}},
		{Name: "job2", Steps: []*Step{{Run: "echo job2"}}},
	}

	tests := []struct {
		name          string
		clientFactory func() *MockClientFactory
		wantErr       bool
	}{
		{
			name: "all jobs succeed",
			clientFactory: func() *MockClientFactory {
				return &MockClientFactory{
					NewClientFunc: func(target *target.Target) (Client, error) {
						return &MockClient{
							ExecuteStepFunc: func(step *Step, stepNum, totalSteps int) error {
								return nil
							},
						}, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "job fails on first target",
			clientFactory: func() *MockClientFactory {
				return &MockClientFactory{
					NewClientFunc: func(target *target.Target) (Client, error) {
						if target.Name == "target1" {
							return &MockClient{
								ExecuteStepFunc: func(step *Step, stepNum, totalSteps int) error {
									return errors.New("step failed")
								},
							}, nil
						}
						return &MockClient{
							ExecuteStepFunc: func(step *Step, stepNum, totalSteps int) error {
								return nil
							},
						}, nil
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := tt.clientFactory()
			service := NewService(factory)

			err := service.ExecuteJobs(targets, jobs)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteJobs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// Helper function to check if an error message contains a specific string
func containsErrorMessage(err error, message string) bool {
	if err == nil {
		return message == ""
	}

	if err.Error() == message {
		return true
	}

	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		return containsErrorMessage(unwrapped, message)
	}

	return false
}
