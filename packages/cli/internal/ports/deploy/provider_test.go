package deploy

import (
	"context"
	"reflect"
	"testing"
)

type fakeProvider struct{ id string }

func (f fakeProvider) ID() string { return f.id }
func (f fakeProvider) Apply(context.Context, ApplyInput) (*ApplyResult, error) {
	return &ApplyResult{Schema: "test/v1"}, nil
}

func TestRegistryGetAndIDs(t *testing.T) {
	registry, err := NewRegistry(fakeProvider{id: "alpha"}, fakeProvider{id: "beta"})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := registry.Get("alpha"); !ok {
		t.Fatalf("Get(alpha) = false, want true")
	}
	if _, ok := registry.Get("missing"); ok {
		t.Fatalf("Get(missing) = true, want false")
	}
	if got, want := registry.IDs(), []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs() = %v, want %v", got, want)
	}
}

func TestRegistryRejectsInvalidProviders(t *testing.T) {
	for _, providers := range [][]Provider{
		{fakeProvider{id: "dup"}, fakeProvider{id: "dup"}},
		{fakeProvider{id: ""}},
		{nil},
	} {
		if _, err := NewRegistry(providers...); err == nil {
			t.Fatalf("NewRegistry(%#v) succeeded", providers)
		}
	}
}
