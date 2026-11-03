#!/usr/bin/env bash
# Writes THIRD_PARTY_NOTICES: the license and NOTICE files of every Go module
# linked into the release binaries. goreleaser runs this before building.
set -euo pipefail

out=${1:-THIRD_PARTY_NOTICES}

# Modules linked into any release target; some (dbus, wincred) are platform-specific.
modules() {
	for os in linux darwin windows; do
		GOOS=$os go list -deps \
			-f '{{if not .Standard}}{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}{{end}}' .
	done | sort -u
}

rule=$(printf '=%.0s' {1..80})

{
	echo "esql includes the following third-party Go modules."
	echo "Their license and notice texts are reproduced below."
	modules | while read -r path version dir; do
		files=$(find "$dir" -maxdepth 1 -type f \( -iname 'licen[cs]e*' -o -iname 'copying*' -o -iname 'notice*' \) | sort)
		if [ -z "$files" ]; then
			echo "error: no license file found for $path $version" >&2
			exit 1
		fi
		printf '\n%s\n%s %s\n%s\n' "$rule" "$path" "$version" "$rule"
		for f in $files; do
			printf '\n--- %s ---\n\n' "$(basename "$f")"
			cat "$f"
		done
	done
} >"$out"
