package config

import (
	"github.com/go-playground/validator/v10"
	"os"
	"path/filepath"
	"runtime"
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

func TestFormatValidationErrors(t *testing.T) {
	// Create a struct for testing validation
	type TestStruct struct {
		Field1 string `validate:"required"`
		Field2 int    `validate:"min=1,max=10"`
	}

	// Initialize validator
	validate := validator.New()

	t.Run("test single validation error", func(t *testing.T) {
		test := TestStruct{
			Field1: "",
			Field2: 5,
		}

		err := validate.Struct(test)
		assert.Error(t, err)

		valErrs, ok := err.(validator.ValidationErrors)
		assert.True(t, ok)

		result := formatValidationErrors(valErrs)
		expected := "Field 'Field1' failed validation: required (condition: )"
		assert.Equal(t, expected, result)
	})

	t.Run("test multiple validation errors", func(t *testing.T) {
		test := TestStruct{
			Field1: "",
			Field2: 11,
		}

		err := validate.Struct(test)
		assert.Error(t, err)

		valErrs, ok := err.(validator.ValidationErrors)
		assert.True(t, ok)

		result := formatValidationErrors(valErrs)
		expected := "Field 'Field1' failed validation: required (condition: )\n" +
			"Field 'Field2' failed validation: max (condition: 10)"
		assert.Equal(t, expected, result)
	})

	t.Run("test various validation rules", func(t *testing.T) {
		type ComplexStruct struct {
			Email    string `validate:"required,email"`
			Age      int    `validate:"required,min=18"`
			Password string `validate:"required,min=8"`
		}

		test := ComplexStruct{
			Email:    "invalid-email",
			Age:      15,
			Password: "123",
		}

		err := validate.Struct(test)
		assert.Error(t, err)

		valErrs, ok := err.(validator.ValidationErrors)
		assert.True(t, ok)

		result := formatValidationErrors(valErrs)
		assert.Contains(t, result, "Field 'Email' failed validation: email")
		assert.Contains(t, result, "Field 'Age' failed validation: min (condition: 18)")
		assert.Contains(t, result, "Field 'Password' failed validation: min (condition: 8)")
	})
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			Targets: []*Target{
				{
					Host:     "localhost",
					User:     "testuser",
					Password: "secret",
				},
			},
			Jobs: []*Job{
				{
					Name: "test-job",
					Steps: []*Step{
						{Run: "echo 'test'"},
					},
				},
			},
		}

		err := validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("missing required fields", func(t *testing.T) {
		config := &Config{
			Targets: []*Target{
				{
					Host: "localhost",
					// Missing User and Password/PrivateKey
				},
			},
			Jobs: []*Job{
				{
					Name:  "test-job",
					Steps: []*Step{}, // Empty steps
				},
			},
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Field 'User' failed validation: required")
	})

	t.Run("invalid host", func(t *testing.T) {
		config := &Config{
			Targets: []*Target{
				{
					Host:     "invalid@host",
					User:     "testuser",
					Password: "secret",
				},
			},
			Jobs: []*Job{
				{
					Steps: []*Step{
						{Run: "echo 'test'"},
					},
				},
			},
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Field 'Host' failed validation: hostname|ip")
	})

	t.Run("invalid port", func(t *testing.T) {
		config := &Config{
			Targets: []*Target{
				{
					Host:     "localhost",
					User:     "testuser",
					Password: "secret",
					Port:     70000,
				},
			},
			Jobs: []*Job{
				{
					Steps: []*Step{
						{Run: "echo 'test'"},
					},
				},
			},
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Field 'Port' failed validation: max")
	})
}

func TestLoadCmdConfig(t *testing.T) {
	// Determine shell command and script extension based on OS
	var shell, shellArg, scriptExt string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		shellArg = "/C"
		scriptExt = ".bat"
	} else {
		shell = "sh"
		shellArg = "-c"
		scriptExt = ".sh"
	}

	t.Run("successful command execution", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cmdtest")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		var scriptContent string
		if runtime.GOOS == "windows" {
			scriptContent = `@echo {"targets":[{"host":"localhost","user":"test","password":"pass"}],"jobs":[{"name":"test","steps":[{"run":"echo hello"}]}]}`
		} else {
			scriptContent = `echo '{"targets":[{"host":"localhost","user":"test","password":"pass"}],"jobs":[{"name":"test","steps":[{"run":"echo hello"}]}]}'`
		}

		scriptPath := filepath.Join(tmpDir, "test"+scriptExt)
		err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
		assert.NoError(t, err)

		config, err := loadCmdConfig(tmpDir, shell, shellArg, scriptPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Len(t, config.Targets, 1)
		assert.Equal(t, "localhost", config.Targets[0].Host)
		assert.Len(t, config.Jobs, 1)
		assert.Equal(t, "test", config.Jobs[0].Name)
	})

	t.Run("command execution error", func(t *testing.T) {
		config, err := loadCmdConfig("", "nonexistentcommand")
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("invalid JSON output", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cmdtest")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		var scriptContent string
		if runtime.GOOS == "windows" {
			scriptContent = "@echo invalid json"
		} else {
			scriptContent = "echo 'invalid json'"
		}

		scriptPath := filepath.Join(tmpDir, "invalid"+scriptExt)
		err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
		assert.NoError(t, err)

		config, err := loadCmdConfig(tmpDir, shell, shellArg, scriptPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to parse config output")
	})

	t.Run("empty output", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cmdtest")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		scriptPath := filepath.Join(tmpDir, "empty"+scriptExt)
		err = os.WriteFile(scriptPath, []byte(""), 0755)
		assert.NoError(t, err)

		config, err := loadCmdConfig(tmpDir, shell, shellArg, scriptPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "invalid output config cmd output")
	})
}

func TestLoadJavaScriptConfig(t *testing.T) {
	t.Run("successful JavaScript config", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "jsconfig")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create test JavaScript file that exports valid config
		jsContent := `
export default {
    targets: [{
        host: "localhost",
        user: "test",
        password: "pass"
    }],
    jobs: [{
        name: "test",
        steps: [{
            run: "echo hello"
        }]
    }]
};`
		jsPath := filepath.Join(tmpDir, "config.js")
		err = os.WriteFile(jsPath, []byte(jsContent), 0755)
		assert.NoError(t, err)

		// Write package.json for ES modules support
		pkgContent := `{"type": "module"}`
		err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgContent), 0644)
		assert.NoError(t, err)

		config, err := loadJavaScriptConfig(jsPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Len(t, config.Targets, 1)
		assert.Equal(t, "localhost", config.Targets[0].Host)
		assert.Len(t, config.Jobs, 1)
		assert.Equal(t, "test", config.Jobs[0].Name)
	})

	t.Run("invalid JavaScript syntax", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "jsconfig")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		jsContent := `
export default {
    invalid syntax
};`
		jsPath := filepath.Join(tmpDir, "invalid.js")
		err = os.WriteFile(jsPath, []byte(jsContent), 0755)
		assert.NoError(t, err)

		config, err := loadJavaScriptConfig(jsPath)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("invalid config structure", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "jsconfig")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		jsContent := `
export default {
    invalid: "config"
};`
		jsPath := filepath.Join(tmpDir, "invalid-config.js")
		err = os.WriteFile(jsPath, []byte(jsContent), 0755)
		assert.NoError(t, err)

		config, err := loadJavaScriptConfig(jsPath)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("non-existent file", func(t *testing.T) {
		config, err := loadJavaScriptConfig("nonexistent.js")
		assert.Error(t, err)
		assert.Nil(t, config)
	})
}

func TestLoadTypeScriptConfig(t *testing.T) {
	t.Run("successful TypeScript config", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "tsconfig")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create test TypeScript file that exports valid config
		tsContent := `
interface Target {
    host: string;
    user: string;
    password: string;
}

interface Job {
    name: string;
    steps: { run: string }[];
}

interface Config {
    targets: Target[];
    jobs: Job[];
}

export default {
    targets: [{
        host: "localhost",
        user: "test",
        password: "pass"
    }],
    jobs: [{
        name: "test",
        steps: [{
            run: "echo hello"
        }]
    }]
} as Config;`

		tsPath := filepath.Join(tmpDir, "config.ts")
		err = os.WriteFile(tsPath, []byte(tsContent), 0644)
		assert.NoError(t, err)

		config, err := loadTypeScriptConfig(tsPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Len(t, config.Targets, 1)
		assert.Equal(t, "localhost", config.Targets[0].Host)
		assert.Len(t, config.Jobs, 1)
		assert.Equal(t, "test", config.Jobs[0].Name)
	})

	t.Run("invalid TypeScript syntax", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "tsconfig")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		tsContent := `
export default {
    invalid syntax
};`
		tsPath := filepath.Join(tmpDir, "invalid.ts")
		err = os.WriteFile(tsPath, []byte(tsContent), 0644)
		assert.NoError(t, err)

		config, err := loadTypeScriptConfig(tsPath)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("non-existent file", func(t *testing.T) {
		config, err := loadTypeScriptConfig("nonexistent.ts")
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("async function export", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "tsconfig")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		tsContent := `
export default async function() {
    return {
        targets: [{
            host: "localhost",
            user: "test",
            password: "pass"
        }],
        jobs: [{
            name: "test",
            steps: [{
                run: "echo hello"
            }]
        }]
    };
}`
		tsPath := filepath.Join(tmpDir, "async-config.ts")
		err = os.WriteFile(tsPath, []byte(tsContent), 0644)
		assert.NoError(t, err)

		config, err := loadTypeScriptConfig(tsPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Len(t, config.Targets, 1)
		assert.Equal(t, "localhost", config.Targets[0].Host)
		assert.Len(t, config.Jobs, 1)
		assert.Equal(t, "test", config.Jobs[0].Name)
	})
}
