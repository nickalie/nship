package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		envVars     map[string]string
		expected    *Config
		expectedErr string
	}{
		{
			name: "valid config with environment variables",
			content: `
targets:
  - name: test-server
    host: ${HOST}
    user: ${USER}
    password: secret
    port: 22
jobs:
  - name: deploy
    steps:
      - run: echo "hello"
      - copy:
          src: ./app
          dst: /opt/app
      - docker:
          image: nginx
          name: web
          ports:
            - "80:80"`,
			envVars: map[string]string{
				"HOST": "localhost",
				"USER": "testuser",
			},
			expected: &Config{
				Targets: []*Target{
					{
						Name:     "test-server",
						Host:     "localhost",
						User:     "testuser",
						Password: "secret",
						Port:     22,
					},
				},
				Jobs: []*Job{
					{
						Name: "deploy",
						Steps: []*Step{
							{Run: "echo \"hello\""},
							{Copy: &CopyStep{
								Src: "./app",
								Dst: "/opt/app",
							}},
							{Docker: &DockerStep{
								Image: "nginx",
								Name:  "web",
								Ports: []string{"80:80"},
							}},
						},
					},
				},
			},
		},
		{
			name:        "invalid yaml",
			content:     "invalid: [yaml",
			expectedErr: "failed to parse YAML",
		},
		{
			name:        "file not found",
			content:     "",
			expectedErr: "failed to read config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			var configPath string
			if tt.content != "" {
				// Create temporary config file
				tmpFile, err := os.CreateTemp("", "config-*.yaml")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpFile.Name())

				if err := os.WriteFile(tmpFile.Name(), []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}
				configPath = tmpFile.Name()
			} else {
				configPath = "nonexistent.yaml"
			}

			config, err := LoadConfig(configPath)

			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, config)
			}
		})
	}
}

func TestReplaceEnvVariables(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "replace single variable",
			content:  "host: ${HOST}",
			envVars:  map[string]string{"HOST": "localhost"},
			expected: "host: localhost",
		},
		{
			name:    "replace multiple variables",
			content: "host: ${HOST}\nuser: ${USER}",
			envVars: map[string]string{
				"HOST": "localhost",
				"USER": "testuser",
			},
			expected: "host: localhost\nuser: testuser",
		},
		{
			name:     "no variables to replace",
			content:  "host: localhost",
			envVars:  map[string]string{},
			expected: "host: localhost",
		},
		{
			name:     "undefined variable",
			content:  "host: ${UNDEFINED}",
			envVars:  map[string]string{},
			expected: "host: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			result := replaceEnvVariables(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}
