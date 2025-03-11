# nship ![logo64](https://github.com/user-attachments/assets/82bc1544-a664-4644-b66f-ca364aa01c4d)


nship is a flexible and efficient deployment automation tool designed to streamline the execution of deployment jobs across multiple targets. It simplifies the configuration and execution of deployment workflows by providing structured job and target management.

## Features

- Define deployment jobs with structured steps.
- Support for remote deployment targets with SSH authentication.
- Configuration management using YAML, JSON, TypeScript, JavaScript, or Golang.
- Built-in support for file copying, script execution, and Docker container management.
- Ansible Vault decryption support for handling secure credentials.
- Skipping unchanged steps for optimized execution.
- CLI-based execution with customizable environment loading.

## Getting Started

### Installation

To install nship, clone the repository and build the binary using Go:

```sh
# Clone the repository
git clone https://github.com/nickalie/nship.git
cd nship

# Build the application
make build
```

Alternatively, you can install nship using Go:

```sh
go install github.com/nickalie/nship/cmd/nship@latest
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

nship uses a structured configuration file that can be written in YAML, JSON, TOML, TypeScript, JavaScript, Golang.

For TypeScript and JavaScript configuration files, Node.js must be installed. For Golang configuration files, Go must be installed.

### Example Configuration (YAML)

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

### Example Configuration (TypeScript)

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

### Example Configuration (Golang)

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

### Example Configuration (TOML)

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
    commands:
      - "npm start"
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
- `commands` (list of strings, optional): List of commands to run inside the container.

## Vault Support

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

