package workspace

// UsesPNPM identifies JavaScript runtimes that share the workspace's package
// manager, package discovery and dependency graph.
func UsesPNPM(toolchain string) bool {
	return toolchain == "node" || toolchain == "bun" || toolchain == "deno"
}
