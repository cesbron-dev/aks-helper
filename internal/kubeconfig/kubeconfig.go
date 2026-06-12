// Package kubeconfig provides a minimal representation of a kubeconfig file,
// sufficient for reading, rewriting and merging the files produced by
// `az aks get-credentials` and `kubelogin convert-kubeconfig`.
//
// We deliberately avoid depending on client-go: the structures below preserve
// unknown fields through map[string]any so we never lose data when round
// tripping a file.
package kubeconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config mirrors the on-disk kubeconfig structure.
type Config struct {
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	CurrentContext string         `yaml:"current-context,omitempty"`
	Clusters       []NamedCluster `yaml:"clusters"`
	Contexts       []NamedContext `yaml:"contexts"`
	Users          []NamedUser    `yaml:"users"`
	Preferences    map[string]any `yaml:"preferences,omitempty"`
}

type NamedCluster struct {
	Name    string         `yaml:"name"`
	Cluster map[string]any `yaml:"cluster"`
}

type NamedContext struct {
	Name    string      `yaml:"name"`
	Context ContextSpec `yaml:"context"`
}

type ContextSpec struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace,omitempty"`
}

type NamedUser struct {
	Name string         `yaml:"name"`
	User map[string]any `yaml:"user"`
}

// Load reads and parses a kubeconfig file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the kubeconfig to disk with restrictive permissions, since it can
// contain client certificates and tokens.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Rename changes the single context name (and the embedded current-context) to
// the provided value. It is used to give stored clusters stable, friendly names
// regardless of how az named them.
func (c *Config) Rename(name string) {
	if len(c.Contexts) == 1 {
		c.Contexts[0].Name = name
	}
	c.CurrentContext = name
}

// ContextNames returns the names of every context defined in the file.
func (c *Config) ContextNames() []string {
	names := make([]string, 0, len(c.Contexts))
	for _, ctx := range c.Contexts {
		names = append(names, ctx.Name)
	}
	return names
}
