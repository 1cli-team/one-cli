package adapters

import "github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"

// init wires the bundled adapters into the public registry. Importing
// this package for side-effects (e.g. `_ "internal/toolchain"`) is
// enough to make Get(Node) / Get(Go) return the correct adapter.
func init() {
	toolchain.Register(nodeAdapter{})
	toolchain.Register(goAdapter{})
}
