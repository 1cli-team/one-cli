// Package preset implements the preset id encoding/decoding (v1) and
// the workspace-scaffolding engine that materialises a preset into a
// concrete workspace + projects.
//
// The id format is a bit-packed string: a leading single-char version,
// followed by dot-separated segments. Each segment carries a single-char
// kind prefix + a fixed-width payload whose length is decided by the
// kind. See the preset documentation for the on-disk grammar.
//
// Stability promise: once a code is assigned in this file (or in
// registry.json's `code` field for templates), it is FROZEN forever and
// never re-used for a different backend. testdata/preset/v1_codes.json
// is the golden file that CI uses to enforce this.
package preset

// SchemaVersion is the current preset id schema version. Bumping this
// is a breaking change; the parser must continue to accept all earlier
// versions forever (don't delete v1 parsing when introducing v2).
const SchemaVersion = 1

// schemaVersionByte is the on-wire encoding of the current schema
// version (single ASCII digit).
const schemaVersionByte byte = '1'

// PresetIDPrefix is the optional human-friendly prefix users may paste
// when sharing an id in docs / chat. The parser strips it; the canonical
// encoder never emits it (id stays as short as possible).
const PresetIDPrefix = "preset:"

// Env provider codes (v1). Single ASCII char.
var envCodes = map[byte]string{
	'd': "dotenv", // Reserved historical code; creation rejects this source.
	'i': "infisical",
}

// Reverse maps, eagerly built so encoding stays O(1).
var (
	envCodeReverse = invertByteMap(envCodes)
)

func invertByteMap(m map[byte]string) map[string]byte {
	out := make(map[string]byte, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

// EnvProviderForCode returns the env provider id for a code, or "" if
// the code is not recognised.
func EnvProviderForCode(c byte) string { return envCodes[c] }

// CodeForEnvProvider returns the env code for a provider, or 0 if none.
func CodeForEnvProvider(name string) byte { return envCodeReverse[name] }

// EnvCodesSnapshot returns a stable snapshot of (code, env) pairs.
func EnvCodesSnapshot() []CodeEntry { return snapshotByteMap(envCodes) }

// CodeEntry is one (code, id) pair used by the golden-file lock test.
type CodeEntry struct {
	Code byte
	ID   string
}

func snapshotByteMap(m map[byte]string) []CodeEntry {
	out := make([]CodeEntry, 0, len(m))
	for c, id := range m {
		out = append(out, CodeEntry{Code: c, ID: id})
	}
	// Sorted by code so codes_test.go's iteration order is deterministic.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Code > out[j].Code; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
