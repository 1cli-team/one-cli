package adapters

import "github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"

// RegisterBundled wires the bundled adapters into the public compatibility
// registry. The CLI composition root calls this explicitly; importing this
// package has no side effects.
func RegisterBundled() {
	toolchain.Register(nodeAdapter{})
	toolchain.Register(goAdapter{})
}
