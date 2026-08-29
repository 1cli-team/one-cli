// Package dotenv implements both the local-filesystem secrets backend
// (loader + `one dotenv` CLI tree) and the canonical .env parser /
// serializer used across the codebase.
//
// The parser primitives (LoadDotenvFile / Parse / Serialize /
// Equal) lived inside the infisical package historically, since
// they were originally written for `one infisical pull` writing local
// .env files. They are not Infisical-specific. Moving them here makes
// the dependency direction match the architecture: infisical → dotenv
// (when pulling secrets out to a local file), not the reverse.
package dotenv

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Parse parses a `.env`-style file (KEY=value lines, # comments, blank
// lines). Quoted values are unwrapped. Returns an empty map for missing
// files so callers can probe `.env.example` without ENOENT noise.
//
// The parser is deliberately minimal — Infisical handles the value
// side, and the only thing we need from `.env.example` is the *set of
// declared keys*.
func Parse(content string) map[string]string {
	out := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:eq])
		val := strings.TrimSpace(trimmed[eq+1:])
		// Unwrap a single layer of single or double quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		out[key] = val
	}
	return out
}

// Serialize produces a deterministic .env-format byte slice from a
// key/value map. Keys are sorted alphabetically so re-pulling the same
// secrets never causes a spurious diff on disk.
//
// Values containing whitespace, '$', '#', or non-printable chars are
// double-quoted with backslash escaping for backslash and double-quote.
// Multiline values use '\n' inside the quoted form.
func Serialize(records map[string]string) string {
	keys := make([]string, 0, len(records))
	for k := range records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(serializeValue(records[k]))
		b.WriteByte('\n')
	}
	return b.String()
}

func serializeValue(v string) string {
	if needsQuoting(v) {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		return `"` + escaped + `"`
	}
	return v
}

func needsQuoting(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		switch r {
		case ' ', '\t', '\n', '\r', '$', '#', '"', '\'', '\\':
			return true
		}
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// LoadDotenvFile reads and parses a .env file at path. A missing file is
// reported via found=false rather than an error so callers can distinguish
// "no .env yet" (often actionable: tell the user to run `one env pull`)
// from "permission denied" or other I/O failures.
func LoadDotenvFile(path string) (vars map[string]string, found bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return Parse(string(raw)), true, nil
}

// Equal reports whether two parsed dotenv maps describe the same
// content. Used to short-circuit conflict detection during `pull`.
func Equal(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}
