// Package preset implements the preset id encoding/decoding (v1) and
// the workspace-scaffolding engine that materialises a preset into a
// concrete workspace + projects.
//
// The id format is a bit-packed string: a leading single-char version,
// followed by dot-separated segments. Each segment carries a single-char
// kind prefix + a fixed-width payload whose length is decided by the
// kind. See SKILL.md / docs/concepts/preset.md for the on-disk grammar.
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

// Deploy backend codes (v1). Single ASCII char from [a-z0-9].
//
// Adding a backend: pick the next unused letter (or digit), append the
// pair here, append the same pair to testdata/preset/v1_codes.json. The
// codes_test.go CI gate refuses renaming or deleting an existing entry.
//
// Note on the `s` slot: the v6 schema break that split the legacy
// `deploy/s3` backend into six S3-protocol-compatible variants happened
// during the same release window that introduced this preset table, so
// `s` was re-pointed at `aws-s3` (the closest semantic successor)
// before any preset id naming `s3` could escape into the wild. No
// frozen vector under testdata/preset/v1_vectors.json uses code `s`,
// so the swap is safe at the encode/decode layer.
var deployCodes = map[byte]string{
	'2': "r2",
	'a': "aliyun-oss",
	'c': "cloudflare",
	'd': "docker",
	'e': "edgeone",
	'k': "kustomize",
	'm': "minio",
	'r': "rustfs",
	's': "aws-s3",
	't': "tencent-cos",
	'v': "vercel",
}

// Env provider codes (v1). Single ASCII char.
var envCodes = map[byte]string{
	'd': "dotenv",
	'i': "infisical",
}

// Container backend codes (v1). Single ASCII char from [a-z0-9].
var containerCodes = map[byte]string{
	'a': "acr",
	'd': "docker",
	'g': "ghcr",
	'h': "dockerhub",
}

// Reverse maps, eagerly built so encoding stays O(1).
var (
	deployCodeReverse    = invertByteMap(deployCodes)
	envCodeReverse       = invertByteMap(envCodes)
	containerCodeReverse = invertByteMap(containerCodes)
)

func invertByteMap(m map[byte]string) map[string]byte {
	out := make(map[string]byte, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

// DeployBackendForCode returns the deploy backend id for a code, or ""
// if the code is not recognised.
func DeployBackendForCode(c byte) string { return deployCodes[c] }

// CodeForDeployBackend returns the deploy code for a backend, or 0 if
// no code is registered for that name.
func CodeForDeployBackend(name string) byte { return deployCodeReverse[name] }

// EnvProviderForCode returns the env provider id for a code, or "" if
// the code is not recognised.
func EnvProviderForCode(c byte) string { return envCodes[c] }

// CodeForEnvProvider returns the env code for a provider, or 0 if none.
func CodeForEnvProvider(name string) byte { return envCodeReverse[name] }

// ContainerBackendForCode returns the container backend id for a code,
// or "" if the code is not recognised.
func ContainerBackendForCode(c byte) string { return containerCodes[c] }

// CodeForContainerBackend returns the container code for a backend, or
// 0 if no code is registered for that name.
func CodeForContainerBackend(name string) byte { return containerCodeReverse[name] }

// DeployCodesSnapshot returns a stable snapshot of (code, backend) pairs
// for use by codes_test.go's lock check against the golden file.
func DeployCodesSnapshot() []CodeEntry { return snapshotByteMap(deployCodes) }

// EnvCodesSnapshot returns a stable snapshot of (code, env) pairs.
func EnvCodesSnapshot() []CodeEntry { return snapshotByteMap(envCodes) }

// ContainerCodesSnapshot returns a stable snapshot of (code, container)
// pairs.
func ContainerCodesSnapshot() []CodeEntry { return snapshotByteMap(containerCodes) }

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
