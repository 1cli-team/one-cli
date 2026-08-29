// Package internalcommon holds string helpers shared by the bundled
// infra backends. Lives under internal/adapters/ so it stays a sibling of
// the backend packages without becoming part of the public API.
package internalcommon

import (
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
