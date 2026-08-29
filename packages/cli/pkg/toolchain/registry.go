package toolchain

// registry holds bundled toolchain adapters keyed by their Toolchain id.
// The CLI composition root registers its adapters explicitly.
//
// This registry remains a public compatibility surface for callers of pkg;
// new internal backend registries are instance-scoped instead.
var registry = map[Toolchain]Adapter{}

// Register adds an adapter to the registry. Multiple registrations for
// the same id overwrite previous entries — this matters only if a
// downstream user wants to swap a bundled adapter for their own.
func Register(a Adapter) {
	if a == nil {
		return
	}
	registry[a.ID()] = a
}

// Get returns the adapter for the given toolchain. Falls back to Node if
// the id is unknown — this preserves the legacy switch-statement default
// where any non-"go" string was treated as Node.
func Get(t Toolchain) Adapter {
	if a, ok := registry[t]; ok {
		return a
	}
	if a, ok := registry[Node]; ok {
		return a
	}
	return nil
}

// Registered returns the ids of all registered adapters in no
// particular order. Reserved for diagnostics / tests.
func Registered() []Toolchain {
	out := make([]Toolchain, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	return out
}
