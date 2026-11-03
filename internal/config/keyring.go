package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService is the service name credentials are stored under in the
// OS keyring (macOS Keychain, Secret Service on Linux, Windows Credential Manager).
const keyringService = "esql-cli"

// Credentials holds the secret parts of a cluster connection.
type Credentials struct {
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
}

// SaveCredentials stores a cluster's secrets in the OS keyring.
func SaveCredentials(cluster string, creds Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, cluster, string(data))
}

// LoadCredentials reads a cluster's secrets from the OS keyring.
func LoadCredentials(cluster string) (Credentials, error) {
	data, err := keyring.Get(keyringService, cluster)
	if err != nil {
		return Credentials{}, err
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return Credentials{}, fmt.Errorf("decoding keyring entry: %w", err)
	}
	return creds, nil
}

// DeleteCredentials removes a cluster's secrets from the OS keyring.
// A missing entry is not an error.
func DeleteCredentials(cluster string) error {
	if err := keyring.Delete(keyringService, cluster); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}
