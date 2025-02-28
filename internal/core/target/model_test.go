package target

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPort(t *testing.T) {
	tests := []struct {
		name         string
		target       Target
		expectedPort int
	}{
		{
			name: "with specified port",
			target: Target{
				Host: "example.com",
				User: "admin",
				Port: 2222,
			},
			expectedPort: 2222,
		},
		{
			name: "with default port",
			target: Target{
				Host: "example.com",
				User: "admin",
			},
			expectedPort: 22,
		},
		{
			name: "with zero port",
			target: Target{
				Host: "example.com",
				User: "admin",
				Port: 0,
			},
			expectedPort: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPort, tt.target.GetPort(), "GetPort() returned unexpected value")
		})
	}
}

func TestGetName(t *testing.T) {
	tests := []struct {
		name         string
		target       Target
		expectedName string
	}{
		{
			name: "with explicit name",
			target: Target{
				Name: "web-server",
				Host: "192.168.1.100",
				User: "admin",
			},
			expectedName: "web-server",
		},
		{
			name: "with default name (hostname)",
			target: Target{
				Host: "example.com",
				User: "admin",
			},
			expectedName: "example.com",
		},
		{
			name: "with empty name",
			target: Target{
				Name: "",
				Host: "192.168.1.101",
				User: "admin",
			},
			expectedName: "192.168.1.101",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedName, tt.target.GetName(), "GetName() returned unexpected value")
		})
	}
}
