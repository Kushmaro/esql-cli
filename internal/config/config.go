package config

import (
	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/spf13/viper"
)

// ClientConfig builds a client.Config from the current Viper state,
// which merges config file, env vars, and CLI flags.
func ClientConfig() client.Config {
	return client.Config{
		Host:     viper.GetString("host"),
		User:     viper.GetString("user"),
		Password: viper.GetString("password"),
		APIKey:   viper.GetString("api_key"),
		Insecure: viper.GetBool("insecure"),
	}
}
