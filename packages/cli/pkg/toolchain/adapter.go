package toolchain

// Adapter is the contract every toolchain must satisfy for package-manager
// selection and dependency installation planning.
//
// The CLI composition root registers bundled implementations explicitly.
// Register remains public for compatibility with downstream adapters.
type Adapter interface {
	ID() Toolchain
	UsesPackageManager() bool
	InstallPlan(in PlanInput) CommandStep
	PackageManagerForManifest(pm PackageManager) PackageManager
}
