// Package docker is the OCI execution adapter used by the compiled container
// module. It shells out to the `docker` CLI for build / push
// / login; also owns the per-subproject Dockerfile writer (Sync /
// ShouldSync) because Dockerfile authoring is toolchain-driven and
// happens at `one add` time, before any container backend runs.
// Toolchain-specific Dockerfile content lives inside the toolchain
// Adapter (so Rust support adds a Rust adapter, not a Rust plugin).
package docker

const backendName = "docker"

// Stable JSON envelope schema strings. JSON consumers + cli snapshot
// tests pin these — DO NOT change without bumping schema versions on
// both the producer and consumer.
const (
	SchemaInfo  = "one-cli/container-info/v2"
	SchemaBuild = "one-cli/container-build/v2"
	SchemaPush  = "one-cli/container-push/v1"
)
