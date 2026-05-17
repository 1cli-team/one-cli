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
