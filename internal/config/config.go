package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Profile holds connection settings for a single cluster.
type Profile struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user,omitempty"`
	Password string `yaml:"password,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
}

// FileConfig represents the full ~/.esql.yaml structure.
type FileConfig struct {
	DefaultProfile string             `yaml:"default_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
}

// ClientConfig builds a client.Config from the current Viper state.
// If --profile is set, it loads that profile's settings.
func ClientConfig() client.Config {
	profile := viper.GetString("profile")

	if profile != "" {
		key := "profiles." + profile + "."
		return client.Config{
			Host:     viper.GetString(key + "host"),
			User:     viper.GetString(key + "user"),
			Password: viper.GetString(key + "password"),
			APIKey:   viper.GetString(key + "api_key"),
			Insecure: viper.GetBool(key + "insecure"),
		}
	}

	if dp := viper.GetString("default_profile"); dp != "" {
		key := "profiles." + dp + "."
		host := viper.GetString(key + "host")
		if host != "" {
			return client.Config{
				Host:     host,
				User:     viper.GetString(key + "user"),
				Password: viper.GetString(key + "password"),
				APIKey:   viper.GetString(key + "api_key"),
				Insecure: viper.GetBool(key + "insecure"),
			}
		}
	}

	return client.Config{
		Host:     viper.GetString("host"),
		User:     viper.GetString("user"),
		Password: viper.GetString("password"),
		APIKey:   viper.GetString("api_key"),
		Insecure: viper.GetBool("insecure"),
	}
}

// ConfigPath returns the path to the config file.
func ConfigPath() (string, error) {
	if f := viper.ConfigFileUsed(); f != "" {
		return f, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".esql.yaml"), nil
}

// LoadFileConfig reads and parses the config file.
func LoadFileConfig() (*FileConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileConfig{Profiles: make(map[string]Profile)}, nil
		}
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return &cfg, nil
}

// SaveFileConfig writes the config to disk.
func SaveFileConfig(cfg *FileConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// ListProfiles returns the names of all configured profiles.
func ListProfiles() ([]string, string, error) {
	cfg, err := LoadFileConfig()
	if err != nil {
		return nil, "", err
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	return names, cfg.DefaultProfile, nil
}
