package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
)

type HostEndpoint struct {
	Name     string `yaml:"name" json:"name"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Token    string `yaml:"token" json:"token"`
}

type Config struct {
	CurrentContext string         `yaml:"currentContext" json:"currentContext"`
	Hosts          []HostEndpoint `yaml:"hosts" json:"hosts"`
	path           string
}

// --- Pure value operations (no IO) ---

// ActiveHost returns the host matching CurrentContext.
func (c Config) ActiveHost() (HostEndpoint, error) {
	if c.CurrentContext == "" {
		return HostEndpoint{}, fmt.Errorf("no active context set")
	}
	for _, h := range c.Hosts {
		if h.Name == c.CurrentContext {
			return h, nil
		}
	}
	return HostEndpoint{}, fmt.Errorf("context %q not found in config", c.CurrentContext)
}

// SetContext sets the current context if the name exists. Returns error if not found.
// Pure value transform — caller must Save() if persistence is needed.
func (c *Config) SetContext(name string) error {
	for _, h := range c.Hosts {
		if h.Name == name {
			c.CurrentContext = name
			return nil
		}
	}
	return fmt.Errorf("context %q not found in config", name)
}

// UpsertHost adds or updates a host by name or endpoint match.
// Pure value transform — caller must Save() if persistence is needed.
func (c *Config) UpsertHost(h HostEndpoint) {
	for i, existing := range c.Hosts {
		if existing.Name == h.Name || existing.Endpoint == h.Endpoint {
			c.Hosts[i] = h
			return
		}
	}
	c.Hosts = append(c.Hosts, h)
}

// Marshal serializes the config to YAML bytes.
func (c Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

// Unmarshal deserializes YAML bytes into a Config.
func Unmarshal(data []byte) (Config, error) {
	var cfg Config
	if len(data) == 0 {
		return cfg, nil
	}
	err := yaml.Unmarshal(data, &cfg)
	return cfg, err
}

// --- IO boundary ---

func (c Config) Path() string {
	return c.path
}

func Load(path string) (Config, error) {
	slog.Debug("loading config", "path", path)
	cfg := Config{path: path}

	if _, err := os.Stat(path); err != nil {
		slog.Debug("config file not found, creating", "path", path)
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		_, _ = os.Create(path)
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	parsed, err := Unmarshal(data)
	if err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	slog.Debug("config loaded", "path", path, "hosts", len(parsed.Hosts), "current_context", parsed.CurrentContext)
	parsed.path = path
	return parsed, nil
}

func (c Config) Save() error {
	slog.Debug("saving config", "path", c.path)
	data, err := c.Marshal()
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(c.path, data, 0644)
}

func ResolvePath() string {
	if envPath, ok := os.LookupEnv("DAPTIN_CLI_CONFIG"); ok {
		return envPath
	}
	home, _ := os.UserHomeDir()
	return home + string(os.PathSeparator) + ".daptin" + string(os.PathSeparator) + "config.yaml"
}
