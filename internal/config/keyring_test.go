package config

import (
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

func TestCredentialsRoundTrip(t *testing.T) {
	keyring.MockInit()

	want := Credentials{Password: "s3cret", APIKey: "abc=="}
	if err := SaveCredentials("prod", want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCredentials("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if err := DeleteCredentials("prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredentials("prod"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := DeleteCredentials("prod"); err != nil {
		t.Fatalf("deleting a missing entry should not fail: %v", err)
	}
}

func TestClientConfigReadsKeyringCluster(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(viper.Reset)

	if err := SaveCredentials("prod", Credentials{Password: "s3cret"}); err != nil {
		t.Fatal(err)
	}
	viper.Set("default_cluster", "prod")
	viper.Set("clusters", map[string]any{
		"prod": map[string]any{"host": "https://prod:9200", "user": "elastic", "keyring": true},
	})

	cfg, err := ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "https://prod:9200" || cfg.User != "elastic" || cfg.Password != "s3cret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestClientConfigKeyringMissingEntry(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(viper.Reset)

	viper.Set("cluster", "prod")
	viper.Set("clusters", map[string]any{
		"prod": map[string]any{"host": "https://prod:9200", "keyring": true},
	})

	if _, err := ClientConfig(); err == nil {
		t.Fatal("expected an error when the keyring entry is missing")
	}
}

func TestClientConfigPlaintextClusterUnchanged(t *testing.T) {
	keyring.MockInitWithError(errors.New("keyring must not be used"))
	t.Cleanup(viper.Reset)

	viper.Set("cluster", "dev")
	viper.Set("clusters", map[string]any{
		"dev": map[string]any{"host": "http://localhost:9200", "api_key": "plain=="},
	})

	cfg, err := ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "plain==" {
		t.Fatalf("expected plaintext api key, got %+v", cfg)
	}
}
