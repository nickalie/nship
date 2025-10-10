# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

nship is a deployment automation tool written in Go that executes deployment jobs across multiple targets. It supports flexible configuration formats (YAML, JSON, TOML, TypeScript, JavaScript, Go) and provides SSH-based remote execution capabilities.

## Build and Development Commands

### Building
```bash
# Build using Task (preferred)
task build

# Direct build
cd cmd/nship && go build -o ../../bin/nship.exe
```

### Testing
```bash
# Run all tests
task test

# Run specific package tests
go test -v ./internal/config/...
go test -v ./internal/core/job/...

# Run specific test
go test -v ./internal/config/ -run TestLoaderLoad
```

### Linting
```bash
task lint
```

## Development Guidelines

- **Never mention Claude in commit messages** - Keep commits professional and tool-agnostic
- **Always run lint and test on any code change** - Ensure code quality by running `task lint` and `task test` before committing

## Architecture

### Directory Structure

- `cmd/nship/` - CLI entry point
- `internal/` - Internal packages (not exported)
  - `config/` - Configuration loading and validation for all supported formats
  - `core/` - Core domain logic
    - `job/` - Job execution service, step hashing, and hash storage interfaces
    - `target/` - Target definitions (SSH, Docker)
  - `infrastructure/` - External integrations
    - `ssh/` - SSH client implementation and session management
    - `fs/` - File system operations (copier, hash storage)
    - `env/` - Environment file loading (supports Ansible Vault decryption)
  - `platform/` - Platform-specific code
    - `cli/` - CLI application logic
  - `util/` - Shared utilities
- `pkg/nship/` - Public API for embedding nship in other applications

### Key Components

#### Configuration Loading (internal/config/loader.go)
- Multi-format loader supporting YAML, JSON, TOML, TypeScript, JavaScript, Go
- Command-based configuration: `cmd:<command>` executes a command and parses JSON output
- Environment variable replacement using `${VAR}` syntax
- TypeScript configs are transpiled using esbuild to JavaScript then executed with Node.js
- JavaScript configs are executed with Node.js, supporting both sync and async default exports
- Go configs are executed with `go run`

#### Job Execution (internal/core/job/service.go)
- Service orchestrates job execution with skip-unchanged optimization
- Steps are hashed; if hash matches stored hash, step is skipped
- Hash computation includes step definition and target-specific context
- Three step types: Run (command execution), Copy (file transfer), Docker (container management)

#### Step Hashing Strategy
- Located in internal/core/job/model.go and hash_storage.go
- Each step is hashed based on its configuration and target
- Hashes are stored per target/job/step index
- If a step hasn't changed and previous hash exists, execution is skipped (unless --no-skip flag is used)

#### SSH Client (internal/infrastructure/ssh/client.go)
- Supports key-based and password authentication
- Session management for command execution
- SFTP integration for file transfers
- Docker operations over SSH

#### File Copying (internal/infrastructure/fs/copier.go)
- Copies files via SFTP with exclusion pattern support
- Only copies files that differ (hash comparison)
- Supports glob patterns for exclusions

### Configuration Model

The configuration consists of:
- **Targets**: Define where to deploy (host, user, auth credentials)
- **Jobs**: Collections of steps to execute
- **Steps**: Individual actions (run command, copy files, manage Docker containers)

Each step can be one of:
- `run`: Execute shell command
- `copy`: Copy local files to remote (with optional exclusions)
- `docker`: Build/run Docker container with full configuration (ports, volumes, env, networks, restart policy)

### Validation

Uses go-playground/validator for struct validation. All config structs have validation tags (required, omitempty, etc.). See internal/config/model.go and internal/core/job/model.go.

### Testing Patterns

- Mock interfaces are defined in test files (e.g., mock_client_test.go)
- Use testify for assertions and mocks
- Test files are excluded from certain linters via .golangci.yml

## Important Notes

- The main CLI flags are in cmd/nship/main.go
- Environment files can be Ansible Vault encrypted (internal/infrastructure/env/loader.go)
- Hash storage is file-based by default (.nship-hashes/)
- Public API is in pkg/nship/api.go for embedding nship in other Go programs
