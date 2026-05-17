// Package toolchain is the public contract for one-cli toolchain adapters.
//
// Adapters generate Dockerfiles, GitHub Actions workflows, and runtime
// resolution (entrypoint command + container port) for newly-added
// subprojects. Each language family ships one adapter.
//
// Stability: this package's exported surface is the integration point
// for future external extensions. Implementations live in
// internal/toolchain/, which registers itself via init() when imported
// for side-effects from internal/cli.
package toolchain

// Toolchain is the canonical id stamped into the registry / manifest.
type Toolchain string

const (
	Node Toolchain = "node"
	Go   Toolchain = "go"
)

// PackageManager is the optional Node-side toolchain. Go subprojects
// don't have one (empty string).
type PackageManager string

const (
	PMpnpm PackageManager = "pnpm"
	PMnpm  PackageManager = "npm"
	PMyarn PackageManager = "yarn"
)

// CommandStep is the install-plan output type used by `add`'s
// post-render success message ("now run pnpm install").
type CommandStep struct {
	Kind    string // "install" / "check" / "test" / "build"
	Command string
	Args    []string
}

// RuntimeResolution is what the Dockerfile / docker-compose service
// block needs from a toolchain: the container entrypoint command + the
// port the service listens on.
type RuntimeResolution struct {
	RunCommand    string
	ContainerPort int
}

// PlanInput is supplied by `add` to drive runtime resolution.
type PlanInput struct {
	PackageManager PackageManager    // empty for Go
	Scripts        map[string]string // package.json#scripts; empty for Go
	TemplateID     string            // e.g. "nestjs-api"
}

// WorkflowInput drives GitHub Actions workflow generation.
type WorkflowInput struct {
	ProjectName      string
	RelativeDir      string
	WorkflowFilePath string
	PackageManager   PackageManager
	Scripts          map[string]string
}

// DockerfileInput drives Dockerfile generation.
type DockerfileInput struct {
	PackageManager PackageManager
	Runtime        RuntimeResolution
}

// StringifyCommandStep formats a CommandStep as `command arg1 arg2 ...`
// so callers can surface it in human messages.
func StringifyCommandStep(step CommandStep) string {
	out := step.Command
	for _, a := range step.Args {
		out += " " + a
	}
	return out
}
