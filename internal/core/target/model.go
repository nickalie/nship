// Package target defines data structures and methods for deployment targets.
package target

// Target defines a deployment destination with connection details.
type Target struct {
	Name       string `yaml:"name" json:"name" toml:"name" validate:"omitempty"`
	Host       string `yaml:"host" json:"host" toml:"host" validate:"required,hostname|ip"`
	User       string `yaml:"user" json:"user" toml:"user" validate:"required"`
	Password   string `yaml:"password" json:"password" toml:"password" validate:"required_without=PrivateKey"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty" toml:"private_key,omitempty" validate:"required_without=Password,omitempty,file"` //nolint:lll // long struct tag needed for complete configuration
	Port       int    `yaml:"port,omitempty" json:"port,omitempty" toml:"port,omitempty" validate:"omitempty,min=1,max=65535"`
}

// GetPort returns the SSH port to use, defaulting to 22 if not specified.
func (t *Target) GetPort() int {
	if t.Port == 0 {
		return 22
	}
	return t.Port
}

// GetName returns the target name, defaulting to host if not specified.
func (t *Target) GetName() string {
	if t.Name == "" {
		return t.Host
	}
	return t.Name
}
