// Package target defines data structures and methods for deployment targets.
package target

// Target defines a deployment destination with connection details.
type Target struct {
	Name       string `yaml:"name" json:"name" validate:"omitempty"`
	Host       string `yaml:"host" json:"host" validate:"required,hostname|ip"`
	User       string `yaml:"user" json:"user" validate:"required"`
	Password   string `yaml:"password" json:"password" validate:"required_without=PrivateKey"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty" validate:"required_without=Password,omitempty,file"`
	Port       int    `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
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
