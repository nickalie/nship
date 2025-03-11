package job

import (
	"github.com/nickalie/nship/internal/core/target"
	"github.com/stretchr/testify/mock"
)

// MockClient mocks the Client interface for testing
type MockClient struct {
	mock.Mock
}

// ExecuteStep implements the Client.ExecuteStep method
func (m *MockClient) ExecuteStep(step *Step, stepNum, totalSteps int) error {
	args := m.Called(step, stepNum, totalSteps)
	return args.Error(0)
}

// Close implements the Client.Close method
func (m *MockClient) Close() {
	m.Called()
}

// MockClientFactory mocks the ClientFactory interface for testing
type MockClientFactory struct {
	mock.Mock
}

// NewClient implements the ClientFactory.NewClient method
func (m *MockClientFactory) NewClient(tgt *target.Target) (Client, error) {
	args := m.Called(tgt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(Client), args.Error(1)
}
