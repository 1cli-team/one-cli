package ci

import "testing"

type providerStub struct{ id string }

func (providerStub) WorkflowFilename(Input) string { return "workflow.yml" }
func (providerStub) Render(Input) string           { return "workflow" }
func (p providerStub) ID() string                  { return p.id }

func TestRegistryIsInstanceScoped(t *testing.T) {
	first := MustRegistry(providerStub{id: "ci/first"})
	second := MustRegistry(providerStub{id: "ci/second"})

	if first.Lookup("ci/second") != nil || second.Lookup("ci/first") != nil {
		t.Fatal("provider registries leaked state between instances")
	}
	providers := first.Providers()
	providers[0] = providerStub{id: "ci/mutated"}
	if first.Lookup("ci/first") == nil {
		t.Fatal("Providers returned mutable registry storage")
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	if _, err := NewRegistry(providerStub{id: "ci/test"}, providerStub{id: "ci/test"}); err == nil {
		t.Fatal("expected duplicate provider IDs to fail")
	}
}
