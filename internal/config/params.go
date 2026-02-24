package config

import (
	"fmt"
	"regexp"
	"strings"
)

var paramRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// ApplyParams replaces {{key}} placeholders in query with the supplied
// --param key=value pairs. Returns an error if any placeholder is left
// unreplaced or if a --param key has no matching placeholder.
func ApplyParams(query string, params []string) (string, error) {
	if len(params) == 0 {
		return query, nil
	}

	kv := make(map[string]string, len(params))
	for _, p := range params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return "", fmt.Errorf("invalid --param %q (expected key=value)", p)
		}
		kv[parts[0]] = parts[1]
	}

	used := make(map[string]bool, len(kv))
	result := paramRe.ReplaceAllStringFunc(query, func(match string) string {
		key := match[2 : len(match)-2] // strip {{ and }}
		if val, ok := kv[key]; ok {
			used[key] = true
			return val
		}
		return match
	})

	// Check for unreplaced placeholders
	if remaining := paramRe.FindAllString(result, -1); len(remaining) > 0 {
		names := make([]string, len(remaining))
		for i, m := range remaining {
			names[i] = m[2 : len(m)-2]
		}
		return "", fmt.Errorf("unresolved placeholder(s): %s (use --param to provide values)", strings.Join(names, ", "))
	}

	// Check for unused --param keys
	for key := range kv {
		if !used[key] {
			return "", fmt.Errorf("--param %q does not match any {{%s}} placeholder in the query", key, key)
		}
	}

	return result, nil
}
