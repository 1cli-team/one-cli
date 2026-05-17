// Package internalcommon holds string helpers shared by the bundled
// infra backends. Lives under internal/infra/ so it stays a sibling of
// the backend packages without becoming part of the public API.
package internalcommon

import (
	"path/filepath"
	"regexp"
	"strings"
)

// NormalizeNewlines folds CRLF line endings into LF so backend output
// is consistent regardless of how the user's tools (or git) wrote the
// file we're editing.
func NormalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// EnsureTrailingNewline guarantees the returned string ends with one
// "\n", which is the convention every infra file in this repo follows.
func EnsureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// EscapeForDoubleQuotedValue protects backslashes and double-quotes so
// the value can be safely embedded in a YAML double-quoted scalar.
func EscapeForDoubleQuotedValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// ResolveWorkloadName returns the docker-compose service / k8s workload
// name for a subproject. Prefers kebab-case of the project name, falls
// back to kebab-case of the dir basename.
func ResolveWorkloadName(projectName, targetDir string) string {
	if k := ToKebabCase(projectName); k != "" {
		return k
	}
	return ToKebabCase(filepath.Base(targetDir))
}

var (
	kebabCamelRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	kebabSplitRE = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// ToKebabCase converts CamelCase, snake_case, or space-separated names
// into a kebab-case slug. Returns "" for inputs that contain no letters
// or digits.
func ToKebabCase(s string) string {
	s = strings.TrimSpace(s)
	s = kebabCamelRE.ReplaceAllString(s, "$1 $2")
	parts := kebabSplitRE.Split(s, -1)
	out := []string{}
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, strings.ToLower(p))
	}
	return strings.Join(out, "-")
}
