package job_test

import (
	"errors"
	"testing"

	"github.com/nickalie/nship/internal/core/job"
	"github.com/nickalie/nship/internal/core/target"
	"github.com/nickalie/nship/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJobService_ExecuteJobs(t *testing.T) {
	tests := []struct {
		name        string
		targets     []*target.Target
		jobs        []*job.Job
		setupMocks  func(*mocks.MockClientFactory)
		expectedErr bool
		errContains string
	}{
		{
			name: "successful execution",
			targets: []*target.Target{
				{Host: "host1", User: "user1", Password: "pass1"},
				{Host: "host2", User: "user2", Password: "pass2"},
			},
			jobs: []*job.Job{
				{Name: "job1", Steps: []*job.Step{{Run: "command1"}}},
				{Name: "job2", Steps: []*job.Step{{Run: "command2"}}},
			},
			setupMocks: func(m *mocks.MockClientFactory) {
				mockClient := &mocks.MockClient{}
				mockClient.On("ExecuteStep", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				mockClient.On("Close").Return()

				m.On("NewClient", mock.Anything).Return(mockClient, nil)
			},
			expectedErr: false,
		},
		{
			name: "client creation fails",
			targets: []*target.Target{
				{Host: "host1", User: "user1", Password: "pass1"},
			},
			jobs: []*job.Job{
				{Name: "job1", Steps: []*job.Step{{Run: "command1"}}},
			},
			setupMocks: func(m *mocks.MockClientFactory) {
				m.On("NewClient", mock.Anything).Return(nil, errors.New("connection failed"))
			},
			expectedErr: true,
			errContains: "connection failed",
		},
		{
			name: "step execution fails",
			targets: []*target.Target{
				{Host: "host1", User: "user1", Password: "pass1"},
			},
			jobs: []*job.Job{
				{Name: "job1", Steps: []*job.Step{{Run: "command1"}}},
			},
			setupMocks: func(m *mocks.MockClientFactory) {
				mockClient := &mocks.MockClient{}
				mockClient.On("ExecuteStep", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("command failed"))
				mockClient.On("Close").Return()

				m.On("NewClient", mock.Anything).Return(mockClient, nil)
			},
			expectedErr: true,
			errContains: "step 1 failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFactory := &mocks.MockClientFactory{}
			tt.setupMocks(mockFactory)

			service := job.NewService(mockFactory)
			err := service.ExecuteJobs(tt.targets, tt.jobs)

			if tt.expectedErr {
				assert.Error(t, err, "Expected error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}

			mockFactory.AssertExpectations(t)
		})
	}
}

func TestJobService_ExecuteJob(t *testing.T) {
	tests := []struct {
		name        string
		target      *target.Target
		job         *job.Job
		setupMocks  func(*mocks.MockClientFactory)
		expectedErr bool
		errContains string
	}{
		{
			name: "successful execution",
			target: &target.Target{
				Host:     "localhost",
				User:     "user",
				Password: "pass",
			},
			job: &job.Job{
				Name: "test-job",
				Steps: []*job.Step{
					{Run: "echo hello"},
					{Run: "ls -la"},
				},
			},
			setupMocks: func(m *mocks.MockClientFactory) {
				mockClient := &mocks.MockClient{}
				mockClient.On("ExecuteStep", mock.Anything, 1, 2).Return(nil)
				mockClient.On("ExecuteStep", mock.Anything, 2, 2).Return(nil)
				mockClient.On("Close").Return()

				m.On("NewClient", mock.Anything).Return(mockClient, nil)
			},
			expectedErr: false,
		},
		{
			name: "client creation fails with custom error",
			target: &target.Target{
				Host:     "localhost",
				User:     "user",
				Password: "pass",
			},
			job: &job.Job{
				Name: "test-job",
				Steps: []*job.Step{
					{Run: "echo hello"},
				},
			},
			setupMocks: func(m *mocks.MockClientFactory) {
				// Return a specific error type
				connErr := &job.ConnectionError{
					Target: "localhost",
					Cause:  errors.New("connection refused"),
				}
				m.On("NewClient", mock.Anything).Return(nil, connErr)
			},
			expectedErr: true,
			errContains: "connection to target localhost failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFactory := &mocks.MockClientFactory{}
			tt.setupMocks(mockFactory)

			service := job.NewService(mockFactory)
			err := service.ExecuteJob(tt.target, tt.job)

			if tt.expectedErr {
				assert.Error(t, err, "Expected error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, "Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no error")
			}

			mockFactory.AssertExpectations(t)
		})
	}
}
