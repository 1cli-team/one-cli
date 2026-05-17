package toolchain

// Adapter is the contract every toolchain must satisfy. The render
// methods produce string content; callers (internal/infra, internal/ci)
// decide where on disk to write it.
//
// Implementations register themselves via Register() in init(). See
// internal/toolchain/{node,go}.go for the bundled adapters.
type Adapter interface {
	ID() Toolchain
	UsesPackageManager() bool
	InstallPlan(in PlanInput) CommandStep
	PackageManagerForManifest(pm PackageManager) PackageManager
	ResolveRuntime(in PlanInput) RuntimeResolution
	RenderDockerfile(in DockerfileInput) string
	RenderWorkflow(in WorkflowInput) string
}
