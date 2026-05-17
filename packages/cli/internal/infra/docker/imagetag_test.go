package docker

// imagetag_test.go locks the tag-composition rules used by Build /
// Push. Three axes interact: registry endpoint (none / host-only /
// host+namespace), per-subproject image override (absent / bare /
// already-namespaced), and the workload name fall-back. Pinning these
// here keeps `one container build` and `one container push` agreeing
// on the same tag for a given input — drift here is silent (the
// build succeeds, the push fails because the tag points at the wrong
// place). HasCredentials behaviour lives next to container.Registry's
// own tests in infra/container/provider_test.go.

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestImageTagFor_NoRegistry(t *testing.T) {
	s := workspace.ManifestProject{Name: "user-api"}
	got := imageTagFor(s, nil, "dev")
	if got != "user-api:dev" {
		t.Errorf("no registry, no override: got %q want %q", got, "user-api:dev")
	}
}

func TestImageTagFor_RegistryWithoutNamespace(t *testing.T) {
	s := workspace.ManifestProject{Name: "user-api"}
	r := &container.Registry{Registry: "ghcr.io"}
	got := imageTagFor(s, r, "dev")
	if got != "ghcr.io/user-api:dev" {
		t.Errorf("got %q", got)
	}
}

func TestImageTagFor_RegistryWithNamespace(t *testing.T) {
	s := workspace.ManifestProject{Name: "user-api"}
	r := &container.Registry{Registry: "registry.example.com", Namespace: "acme-corp"}
	got := imageTagFor(s, r, "dev")
	want := "registry.example.com/acme-corp/user-api:dev"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// Per-subproject override containing a slash is treated as already-
// fully-qualified — the user is composing the tag themselves, so the
// registry prefix is skipped.
func TestImageTagFor_QualifiedOverride_BypassesRegistry(t *testing.T) {
	s := workspace.ManifestProject{
		Name: "user-api",
		Domains: &workspace.ProjectDomains{
			Container: &workspace.ProjectContainerOverride{Image: "myorg/user-api:custom"},
		},
	}
	r := &container.Registry{Registry: "ghcr.io", Namespace: "ignored"}
	got := imageTagFor(s, r, "dev")
	if got != "myorg/user-api:custom" {
		t.Errorf("qualified override should pass through, got %q", got)
	}
}

// Bare override (no slash, no colon) gets `:dev` appended and respects
// the registry prefix — so `image: api` with a private registry yields
// `<registry>/<namespace>/api:dev`.
func TestImageTagFor_BareOverride_RespectsRegistry(t *testing.T) {
	s := workspace.ManifestProject{
		Name: "user-api",
		Domains: &workspace.ProjectDomains{
			Container: &workspace.ProjectContainerOverride{Image: "api"},
		},
	}
	r := &container.Registry{Registry: "registry.example.com", Namespace: "acme-corp"}
	got := imageTagFor(s, r, "dev")
	want := "registry.example.com/acme-corp/api:dev"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
