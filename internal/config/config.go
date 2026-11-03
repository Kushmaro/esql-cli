package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Cluster holds connection settings for a single Elasticsearch cluster.
type Cluster struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user,omitempty"`
	Password string `yaml:"password,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
	// Keyring means Password and APIKey live in the OS keyring, not this file.
	Keyring bool `yaml:"keyring,omitempty"`
}

// SavedQuery stores a query string and the cluster it was saved from.
type SavedQuery struct {
	Query   string `yaml:"query"`
	Cluster string `yaml:"cluster,omitempty"`
}

// UnmarshalYAML handles both the legacy plain-string format and the new
// structured format so existing config files keep working.
func (sq *SavedQuery) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		sq.Query = value.Value
		return nil
	}
	type raw SavedQuery
	return value.Decode((*raw)(sq))
}

// FileConfig represents the full ~/.esql.yaml structure.
type FileConfig struct {
	DefaultCluster string                `yaml:"default_cluster,omitempty"`
	Clusters       map[string]Cluster    `yaml:"clusters,omitempty"`
	Queries        map[string]SavedQuery `yaml:"queries,omitempty"`
}

// ClientConfig builds a client.Config from the current Viper state.
// If --cluster is set, it loads that cluster's settings.
// Returns an error if the requested cluster does not exist.
func ClientConfig() (client.Config, error) {
	cluster := viper.GetString("cluster")

	verbose := viper.GetBool("verbose")

	if cluster != "" {
		cfg, found, err := clusterClientConfig(cluster, verbose)
		if err != nil {
			return client.Config{}, err
		}
		if !found {
			return client.Config{}, fmt.Errorf("unknown cluster %q (run 'esql clusters list' to see available clusters)", cluster)
		}
		return cfg, nil
	}

	if dc := viper.GetString("default_cluster"); dc != "" {
		cfg, found, err := clusterClientConfig(dc, verbose)
		if err != nil {
			return client.Config{}, err
		}
		if found {
			return cfg, nil
		}
	}

	return client.Config{
		Host:     viper.GetString("host"),
		User:     viper.GetString("user"),
		Password: viper.GetString("password"),
		APIKey:   viper.GetString("api_key"),
		Insecure: viper.GetBool("insecure"),
		Verbose:  verbose,
	}, nil
}

// clusterClientConfig loads a named cluster's settings, reading its secrets
// from the OS keyring when the cluster is marked keyring: true.
// found is false when no cluster with that name has a host configured.
func clusterClientConfig(name string, verbose bool) (cfg client.Config, found bool, err error) {
	key := "clusters." + name + "."
	host := viper.GetString(key + "host")
	if host == "" {
		return client.Config{}, false, nil
	}
	cfg = client.Config{
		Host:     host,
		User:     viper.GetString(key + "user"),
		Password: viper.GetString(key + "password"),
		APIKey:   viper.GetString(key + "api_key"),
		Insecure: viper.GetBool(key + "insecure"),
		Verbose:  verbose,
	}
	if viper.GetBool(key + "keyring") {
		creds, err := LoadCredentials(name)
		if err != nil {
			return client.Config{}, true, fmt.Errorf("reading credentials for cluster %q from the OS keyring: %w (run 'esql init' to store them again)", name, err)
		}
		cfg.Password = creds.Password
		cfg.APIKey = creds.APIKey
	}
	return cfg, true, nil
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
			return &FileConfig{
				Clusters: make(map[string]Cluster),
				Queries:  make(map[string]SavedQuery),
			}, nil
		}
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.Clusters == nil {
		cfg.Clusters = make(map[string]Cluster)
	}
	if cfg.Queries == nil {
		cfg.Queries = make(map[string]SavedQuery)
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

	// Open without truncating first: WriteFile's mode only applies to new
	// files, so an existing config must be secured before writing secrets.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return fmt.Errorf("securing config permissions: %w", err)
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Close()
}

// ListClusters returns the names of all configured clusters.
func ListClusters() ([]string, string, error) {
	cfg, err := LoadFileConfig()
	if err != nil {
		return nil, "", err
	}

	names := make([]string, 0, len(cfg.Clusters))
	for name := range cfg.Clusters {
		names = append(names, name)
	}
	return names, cfg.DefaultCluster, nil
}

// HasConnectionConfig reports whether any Elasticsearch connection
// settings have been configured -- via config file clusters, environment
// variables, or top-level config keys. It does NOT consider CLI flags
// (those are checked separately by the caller).
func HasConnectionConfig() bool {
	if viper.GetString("user") != "" || viper.GetString("api_key") != "" {
		return true
	}
	if viper.GetString("default_cluster") != "" {
		return true
	}
	if clusters := viper.GetStringMap("clusters"); len(clusters) > 0 {
		return true
	}
	return false
}

// ActiveClusterName returns the name of the cluster that will be used for
// queries — either the explicit --cluster flag or the default_cluster from config.
// Returns "" when using bare host/credentials without a named cluster.
func ActiveClusterName() string {
	if c := viper.GetString("cluster"); c != "" {
		return c
	}
	return viper.GetString("default_cluster")
}

// ListQueryNames returns the names of all saved queries.
func ListQueryNames() ([]string, error) {
	cfg, err := LoadFileConfig()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Queries))
	for name := range cfg.Queries {
		names = append(names, name)
	}
	return names, nil
}
