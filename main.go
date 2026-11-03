package main

import "github.com/Kushmaro/esql-cli/cmd"

// Set at build time via -ldflags (see Makefile and .goreleaser.yaml).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
