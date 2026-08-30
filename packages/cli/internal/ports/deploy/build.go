package deploy

import "context"

// Builder prepares a project before its deploy provider runs. Implementations
// decide whether a target needs a build and return the command lines that a
// dry-run should render.
type Builder interface {
	Build(context.Context, BuildInput) ([]string, error)
}

// BuildInput contains the transport-neutral facts needed to prepare one
// deployment target. Apply already carries project paths, streams, injected
// environment values, and the dry-run flag.
type BuildInput struct {
	Apply          ApplyInput
	Backend        string
	Toolchain      string
	PackageManager string
}
