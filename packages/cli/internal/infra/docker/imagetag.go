package docker

// imagetag.go: image-tag composition rules used by Build / Push +
// `<workload>:<version>` <-> `<registry>/[<namespace>/]<workload>:<version>`
// fall-backs. Locked by imagetag_test.go: drift here breaks the
// build/push round-trip silently (build succeeds, push fails or
// targets the wrong place).

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/internalcommon"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// imageTagFor derives the image tag for a project. When r is set and
// has a Registry host, prepends `<registry>/[<namespace>/]` so the
// resulting tag is push-ready
// (`<registry-host>/<namespace>/web:<version>`).
//
// Namespace lookup precedence:
//  1. projects[i].container.namespace (per-project manifest field)
//  2. r.Namespace fall-back (tests + future CLI shorthand)
//
// Honors any `projects[].container.image` override; an override
// containing a slash is treated as already-fully-qualified (the user
// is doing their own composition) and the registry prefix is skipped.
func imageTagFor(s workspace.ManifestProject, r *container.Registry, defaultTag string) string {
	if defaultTag == "" {
		defaultTag = "dev"
	}
	bare := internalcommon.ToKebabCase(s.Name) + ":" + defaultTag
	override := ""
	namespace := ""
	if c := containerOverride(s); c != nil {
		override = c.Image
		namespace = c.Namespace
	}
	if override != "" {
		// Caller-supplied tag already has its colon-tag form (...:dev) —
		// keep the v0.5 contract: the override IS the tag, with the
		// default version tag suffixed when not already present.
		if !strings.Contains(override, ":") {
			override += ":" + defaultTag
		}
		// Registry-qualified override (contains '/') → use as-is.
		if strings.Contains(override, "/") {
			return override
		}
		bare = override
	}
	if r == nil || r.Registry == "" {
		return bare
	}
	if namespace == "" {
		namespace = r.Namespace
	}
	prefix := r.Registry
	if namespace != "" {
		prefix = prefix + "/" + namespace
	}
	return prefix + "/" + bare
}

func imageTagVersion(ref string) string {
	ref = strings.TrimSpace(ref)
	idx := strings.LastIndex(ref, ":")
	slash := strings.LastIndex(ref, "/")
	if idx > slash {
		return ref[idx+1:]
	}
	return ""
}
