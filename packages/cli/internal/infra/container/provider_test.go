package container

import (
	"context"
	"reflect"
	"testing"
)

type fakeProvider struct{ id string }

func (f fakeProvider) ID() string { return f.id }
func (f fakeProvider) Info(context.Context, InfoInput) (*InfoResult, error) {
	return &InfoResult{Schema: "test/v1"}, nil
}
func (f fakeProvider) Build(context.Context, BuildInput) (*BuildResult, error) {
	return &BuildResult{Schema: "test/v1"}, nil
}
func (f fakeProvider) Push(context.Context, PushInput) (*PushResult, error) {
	return &PushResult{Schema: "test/v1"}, nil
}

func resetRegistry(t *testing.T) func() {
	t.Helper()
	saved := registry
	registry = map[string]Provider{}
	return func() { registry = saved }
}

func TestRegisterAndGet(t *testing.T) {
	defer resetRegistry(t)()
	Register(fakeProvider{id: "alpha"})
	Register(fakeProvider{id: "beta"})

	if _, ok := Get("alpha"); !ok {
		t.Fatalf("Get(alpha) = false, want true")
	}
	if _, ok := Get("missing"); ok {
		t.Fatalf("Get(missing) = true, want false")
	}
	if got, want := IDs(), []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs() = %v, want %v", got, want)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer resetRegistry(t)()
	Register(fakeProvider{id: "dup"})
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate Register, got none")
		}
	}()
	Register(fakeProvider{id: "dup"})
}

func TestRegisterEmptyIDPanics(t *testing.T) {
	defer resetRegistry(t)()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty ID, got none")
		}
	}()
	Register(fakeProvider{id: ""})
}

func TestRegisterNilPanics(t *testing.T) {
	defer resetRegistry(t)()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil provider, got none")
		}
	}()
	Register(nil)
}

func TestRegistryHasCredentials(t *testing.T) {
	tests := []struct {
		name string
		r    *Registry
		want bool
	}{
		{"nil", nil, false},
		{"empty", &Registry{}, false},
		{"only host", &Registry{Registry: "ghcr.io"}, false},
		{"only user", &Registry{Registry: "ghcr.io", Username: "u"}, false},
		{"full", &Registry{Registry: "ghcr.io", Username: "u", Password: "p"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasCredentials(); got != tt.want {
				t.Fatalf("HasCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}
