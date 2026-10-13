// Package workspace owns workspace-level read and write operations
// (manifest, infra detection, CI sync, project-name normalisation).
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// nameRE is the canonical project / workspace name shape.
	nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	// camelToWordsRE inserts a separator between a lowercase/digit and an
	// uppercase to support splitting CamelCase identifiers.
	camelToWordsRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	// nonAlphaNumRE splits a string on any run of non-alphanumeric chars.
	nonAlphaNumRE = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// IsValidProjectName reports whether s is a legal project / workspace
// name. Used by both create and add when the user supplies a name.
func IsValidProjectName(s string) bool {
	return nameRE.MatchString(s)
}

func splitWords(s string) []string {
	s = strings.TrimSpace(s)
	s = camelToWordsRE.ReplaceAllString(s, "$1 $2")
	parts := nonAlphaNumRE.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, strings.ToLower(p))
	}
	return out
}

// ToKebabCase lowercases s and joins its word fragments with hyphens.
func ToKebabCase(s string) string {
	return strings.Join(splitWords(s), "-")
}

// ToPosixPath converts a possibly-Windows-style path to forward slashes for
// stable JSON output and cross-platform manifest comparisons.
func ToPosixPath(p string) string {
	return strings.ReplaceAll(p, string(filepath.Separator), "/")
}

// GenerateProjectID returns a stable workspace identifier shaped as
// "<kebab-name>-<6-char-hex>". The hex suffix uses crypto/rand so two
// workspaces created with the same name on different machines do not
// collide.
//
// If the name kebab-cases to an empty string (extreme edge case — name
// was all separators), the prefix falls back to "ws".
func GenerateProjectID(name string) string {
	prefix := ToKebabCase(name)
	if prefix == "" {
		prefix = "ws"
	}
	var raw [3]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return prefix + "-000000"
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}
