package toolchain

// Adapter is the contract every toolchain must satisfy. The render
// methods produce string content; callers (internal/adapters, internal/ci)
// decide where on disk to write it.
//
// The CLI composition root registers bundled implementations explicitly.
// Register remains public for compatibility with downstream adapters.
type Adapter interface {
	ID() Toolchain
	UsesPackageManager() bool
	InstallPlan(in PlanInput) CommandStep
	PackageManagerForManifest(pm PackageManager) PackageManager
	ResolveRuntime(in PlanInput) RuntimeResolution
	RenderDockerfile(in DockerfileInput) string
	RenderWorkflow(in WorkflowInput) string
}
