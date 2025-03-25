# NShip

nship is a flexible and efficient deployment automation tool designed to streamline the execution of deployment jobs across multiple targets. It simplifies the configuration and execution of deployment workflows by providing structured job and target management.

## Features

- Define deployment jobs with structured steps.
- Support for remote deployment targets with SSH authentication.
- Configuration management using YAML, JSON, TOML, TypeScript, JavaScript, Golang, or any command output.
- Built-in support for file copying, script execution, and Docker container management.
- Ansible Vault decryption support for handling secure credentials.
- Skipping unchanged steps for optimized execution.
- CLI-based execution with customizable environment loading.

## Getting Started

### Installation

You can install nship in several ways:

#### Binary Releases

Download pre-built binaries for your platform from the [GitHub Releases page](https://github.com/nickalie/nship/releases).

We provide binaries for:

- Linux (x86_64, arm64)

- macOS (x86_64, arm64)

- Windows (x86_64, arm64)

#### Linux Package Managers

For Debian/Ubuntu:
```sh
curl -LO https://github.com/nickalie/nship/releases/download/v{version}/nship_{version}_linux_x86_64.deb
sudo dpkg -i nship_{version}_linux_x86_64.deb
```

For RHEL/CentOS/Fedora:
```sh
curl -LO https://github.com/nickalie/nship/releases/download/v{version}/nship_{version}_linux_x86_64.rpm
sudo rpm -i nship_{version}_linux_x86_64.rpm
```

For Alpine Linux:
```sh
curl -LO https://github.com/nickalie/nship/releases/download/v{version}/nship_{version}_linux_x86_64.apk
sudo apk add --allow-untrusted nship_{version}_linux_x86_64.apk
```

#### Go Install

If you have Go installed:

```sh
go install github.com/nickalie/nship/cmd/nship@latest
```

#### Build from Source

To build nship from source:

```sh
# Clone the repository
git clone https://github.com/nickalie/nship.git
cd nship

# Build the application
make build
```

### Usage

Run a deployment job using:

```sh
nship --config=config.yaml --job=deploy-app
```

Additional options:

- `--config=<path>`: Path to the configuration file (default: `nship.yaml`).
- `--job=<name>`: Name of the job to run.
- `--env-file=<path>`: Path to an environment file (can be specified multiple times).
- `--vault-password=<password>`: Password for decrypting Ansible Vault files.
- `--no-skip`: Disable skipping unchanged steps.
- `--version`: Show version information.

#### Environment Files

Environment files can be specified in several ways:

```sh
# Single environment file
nship --env-file=dev.env

# Multiple environment files using multiple flags
nship --env-file=dev.env --env-file=secrets.env
```

## Configuration

nship offers exceptional flexibility in how you define your deployment configurations. Choose the format that best fits your workflow:

### Configuration Formats

- **YAML/JSON/TOML**: Simple, structured formats for static configurations
- **TypeScript/JavaScript**: Leverage the full power of a programming language with type safety, variables, and logic
- **Golang**: Use Go's strong typing and performance for complex configuration needs
- **Command Output**: Generate configurations dynamically using any script or command

### Dynamic Configuration with Programming Languages

#### TypeScript/JavaScript Benefits
- **Type Safety**: Catch configuration errors before runtime with TypeScript
- **Code Reuse**: Create functions for repeated configuration patterns
- **Environment Handling**: Programmatically include environment-specific settings
- **Dynamic Generation**: Generate configurations based on external data sources

#### Golang Configuration Benefits
- **Strong Typing**: Ensure configuration correctness with Go's type system
- **Builder Pattern**: Use the builder API for cleaner configuration construction
- **Performance**: Generate complex configurations efficiently
- **Direct Integration**: Access Go libraries within your configuration

### Command-based Configuration

Command-based configurations provide maximum flexibility by letting you generate configurations dynamically:

```sh
# Use output from any command as configuration
nship --config="cmd:curl https://example.com/config"

# Generate environment-specific configurations
nship --config="cmd:./generate-config.sh --env=production"

# Use database-driven configurations
nship --config="cmd:python scripts/db_to_config.py"
```

This approach enables:
- **CI/CD Integration**: Generate configurations during pipeline execution
- **Centralized Management**: Pull configurations from central repositories
- **Dynamic Settings**: Include real-time system information in deployments

### Example Configurations

#### YAML Configuration
```yaml
targets:
  - name: production
    host: prod.example.com
    user: deploy
    private_key: ~/.ssh/id_rsa

jobs:
  - name: deploy-app
    steps:
      - run: echo "Deploying application..."
      - copy:
          local: ./app/
          remote: /var/www/app/
      - docker:
          image: myapp:latest
          name: myapp-container
          ports: ["8080:80"]
```

#### JSON Configuration
```json
{
  "targets": [
    {
      "name": "production",
      "host": "prod.example.com",
      "user": "deploy",
      "private_key": "~/.ssh/id_rsa"
    }
  ],
  "jobs": [
    {
      "name": "deploy-app",
      "steps": [
        {
          "run": "echo \"Deploying application...\""
        },
        {
          "copy": {
            "local": "./app/",
            "remote": "/var/www/app/"
          }
        },
        {
          "docker": {
            "image": "myapp:latest",
            "name": "myapp-container",
            "ports": ["8080:80"],
            "build": {
              "context": "./app",
              "args": {
                "VERSION": "1.0.0"
              }
            }
          }
        }
      ]
    }
  ]
}
```

#### TypeScript Configuration
```ts
export default {
  targets: [
    { name: "production", host: "prod.example.com", user: "deploy", private_key: "~/.ssh/id_rsa" }
  ],
  jobs: [
    {
      name: "deploy-app",
      steps: [
        { run: "echo \"Deploying application...\"" },
        { copy: { local: "./app/", remote: "/var/www/app/" } },
        { docker: { image: "myapp:latest", name: "myapp-container", ports: ["8080:80"] } }
      ]
    }
  ]
};
```

#### TypeScript with Dynamic Configuration
```ts
// Example of dynamic configuration with TypeScript
import * as fs from 'fs';

// Load environments from external file
const environments = JSON.parse(fs.readFileSync('./environments.json', 'utf8'));

// Create reusable step patterns
const createDeploymentSteps = (appName: string, version: string) => [
  { run: `echo "Deploying ${appName} version ${version}..."` },
  { copy: { local: `./${appName}/`, remote: `/var/www/${appName}/` } },
  { docker: { 
      image: `${appName}:${version}`, 
      name: `${appName}-container`,
      ports: ["8080:80"] 
    }
  }
];

export default {
  targets: environments.map(env => ({
    name: env.name,
    host: env.host,
    user: env.user,
    private_key: env.keyPath
  })),
  jobs: [
    {
      name: "deploy-frontend",
      steps: createDeploymentSteps("frontend", "1.2.3")
    },
    {
      name: "deploy-backend",
      steps: createDeploymentSteps("backend", "4.5.6")
    }
  ]
};
```

#### Golang Configuration
```go
package main

import (
	"github.com/nickalie/nship/pkg/nship"
	"log"
)

func main() {
	err := nship.NewBuilder().
		AddTarget(&nship.Target{
			Host: "prod.example.com",
			User: "deploy",
			PrivateKey: "~/.ssh/id_rsa",
		}).
		AddJob("deploy-app").
		AddRunStep("echo 'Deploying application...'").
		AddCopyStep("./app/", "/var/www/app/").
		AddDockerStep(&nship.DockerStep{
			Image:    "myapp:latest",
			Name:     "myapp-container",
			Ports:    []string{"8080:80"},
		}).
		Print()

	if err != nil {
		log.Fatal(err)
	}
}
```

#### Example Configuration (TOML)

```toml
[[targets]]
name = "production"
host = "prod.example.com"
user = "deploy"
private_key = "~/.ssh/id_rsa"

[[jobs]]
name = "deploy-app"

[[jobs.steps]]
run = "echo 'Deploying application...'"

[[jobs.steps.copy]]
local = "./app/"
remote = "/var/www/app/"

[[jobs.steps.docker]]
image = "myapp:latest"
name = "myapp-container"
ports = ["8080:80"]
```

## Deployment Steps

### Run Step

Executes a shell command:

```yaml
- run: |
    echo "Starting deployment..."
    mkdir -p /var/www/app
    cp -r ./app/* /var/www/app/
    systemctl restart myapp

- run: systemctl restart myapp
```

### Copy Step

Copies files to a remote target. Identical files are not copied to optimize performance:

```yaml
- copy:
    local: ./config/
    remote: /etc/myapp/
```

### Docker Step

Runs a Docker container on the target. If the container already exists, it will be removed before starting a new instance:

```yaml
- docker:
    image: nginx:latest
    name: web-server
    ports: ["80:80"]
    environment:
      - "ENV_VAR=value"
    volumes:
      - "/host/path:/container/path"
    labels:
      app: "my-web-app"
    networks:
      - "custom-network"
    restart: "always"
    command:
      - "npm start"
    build:
      context: ./path/to/directory/with/dockerfile
      args:
        VERSION: "1.0.0"
        DEBUG: "true"
```

#### Supported Keys in Docker Step

- `image` (string, required): Docker image to use.
- `name` (string, required): Name of the Docker container.
- `ports` (list of strings, optional): List of port mappings in the format `host:container`.
- `environment` (list of strings, optional): List of environment variables.
- `volumes` (list of strings, optional): List of volume mounts in the format `host_path:container_path`.
- `labels` (map of key-value pairs, optional): Labels to assign to the container.
- `networks` (list of strings, optional): List of network names to connect the container.
- `restart` (string, optional): Restart policy (`no`, `on-failure`, `always`, `unless-stopped`).
- `command` (list of strings, optional): List of commands to run inside the container.
- `build` (object, optional): Configuration for building the Docker image before running the container.
  - `context` (string, required): Build context path where the Dockerfile is located.
  - `args` (map of key-value pairs, optional): Build arguments to pass to the Docker build command.

## Ansible Vault Support

nship supports Ansible Vault for secure credentials management. To decrypt a vault file, use:

```sh
nship --env-file=env.vault --vault-password=yourpassword
```
The VAULT_PASSWORD environment variable can also be used to provide the password for decrypting Ansible Vault files, allowing for more secure automation workflows.

If both --vault-password and the VAULT_PASSWORD environment variable are missing, the tool will prompt for the password in the terminal.

## Skipping Unchanged Steps

By default, nship skips execution of unchanged steps to optimize performance. Use `--no-skip` to disable this behavior.

## Contributing

Contributions are welcome! Feel free to submit issues and pull requests.

## License

This project is licensed under the MIT License.

