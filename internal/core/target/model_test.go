package target

import (
	"testing"
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
			if got := tt.target.GetPort(); got != tt.expectedPort {
				t.Errorf("Target.GetPort() = %v, want %v", got, tt.expectedPort)
			}
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
			if got := tt.target.GetName(); got != tt.expectedName {
				t.Errorf("Target.GetName() = %v, want %v", got, tt.expectedName)
			}
		})
	}
}
