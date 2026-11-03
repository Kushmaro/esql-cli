package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Kushmaro/esql-cli/internal/client"
	"github.com/Kushmaro/esql-cli/internal/config"
	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

func TestInitCredentialStorage(t *testing.T) {
	for _, auth := range []string{"password", "api_key"} {
		for _, available := range []bool{true, false} {
			for _, existing := range []bool{false, true} {
				name := fmt.Sprintf("%s/keyring=%t/existing=%t", auth, available, existing)
				t.Run(name, func(t *testing.T) {
					viper.Reset()
					t.Cleanup(viper.Reset)
					keyring.MockInit()
					t.Cleanup(keyring.MockInit)
					if !available {
						keyring.MockInitWithError(errors.New("secure store unavailable"))
					}

					home := t.TempDir()
					t.Setenv("HOME", home)
					t.Setenv("USERPROFILE", home)
					oldCfgFile := cfgFile
					cfgFile = ""
					t.Cleanup(func() { cfgFile = oldCfgFile })
					path := filepath.Join(home, ".esql.yaml")
					if existing {
						// Replacing a keyring-backed cluster must clear its marker
						// on fallback and secure an existing, broadly readable file.
						data := "default_cluster: prod\nclusters:\n  prod:\n    host: http://old:9200\n    keyring: true\nqueries:\n  saved: FROM logs\n"
						if err := os.WriteFile(path, []byte(data), 0600); err != nil {
							t.Fatal(err)
						}
						if err := os.Chmod(path, 0644); err != nil {
							t.Fatal(err)
						}
					}
					initConfig()

					creds := config.Credentials{Password: "test-password"}
					answers := "1\nelastic\ntest-password\n"
					if auth == "api_key" {
						creds = config.Credentials{APIKey: "test-api-key=="}
						answers = "2\ntest-api-key==\n"
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if auth == "api_key" {
							if r.Header.Get("Authorization") != "ApiKey "+creds.APIKey {
								t.Error("request did not use the saved API key")
							}
						} else if user, password, ok := r.BasicAuth(); !ok || user != "elastic" || password != creds.Password {
							t.Error("request did not use the saved password")
						}
						fmt.Fprint(w, `{"cluster_name":"test","version":{"number":"9.0.0"}}`)
					}))
					t.Cleanup(server.Close)

					input, err := os.CreateTemp(t.TempDir(), "stdin")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { input.Close() })
					if _, err := fmt.Fprintf(input, "prod\n%s\n%sN\n", server.URL, answers); err != nil {
						t.Fatal(err)
					}
					if _, err := input.Seek(0, 0); err != nil {
						t.Fatal(err)
					}
					output, err := os.CreateTemp(t.TempDir(), "stderr")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { output.Close() })
					oldStdin, oldStderr := os.Stdin, os.Stderr
					os.Stdin, os.Stderr = input, output
					t.Cleanup(func() { os.Stdin, os.Stderr = oldStdin, oldStderr })
					if err := runInit(initCmd, nil); err != nil {
						t.Fatal(err)
					}

					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
						t.Errorf("config permissions = %o, want 600", info.Mode().Perm())
					}
					fileCfg, err := config.LoadFileConfig()
					if err != nil {
						t.Fatal(err)
					}
					cluster := fileCfg.Clusters["prod"]
					if cluster.Keyring != available {
						t.Errorf("keyring marker = %t, want %t", cluster.Keyring, available)
					}
					if existing && fileCfg.Queries["saved"].Query != "FROM logs" {
						t.Error("existing saved query was not preserved")
					}
					if available {
						if cluster.Password != "" || cluster.APIKey != "" {
							t.Error("secrets were written to the config despite successful keyring storage")
						}
						stored, err := config.LoadCredentials("prod")
						if err != nil || stored != creds {
							t.Errorf("keyring credentials do not match: %v", err)
						}
					} else if cluster.Password != creds.Password || cluster.APIKey != creds.APIKey {
						t.Error("fallback credentials were not saved in the home config")
					}
					messages, err := os.ReadFile(output.Name())
					if err != nil {
						t.Fatal(err)
					}
					if warned := strings.Contains(string(messages), "plain text"); warned == available {
						t.Errorf("plaintext warning = %t, want %t", warned, !available)
					}

					// Reload as a fresh CLI invocation and authenticate using only
					// the saved state, while the keyring is still unavailable.
					viper.Reset()
					initConfig()
					clientCfg, err := config.ClientConfig()
					if err != nil {
						t.Fatal(err)
					}
					if _, err := client.New(clientCfg).Ping(); err != nil {
						t.Fatalf("authentication with saved credentials failed: %v", err)
					}
				})
			}
		}
	}
}
