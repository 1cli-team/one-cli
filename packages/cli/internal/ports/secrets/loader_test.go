package secrets

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fake struct{ id string }

func (f fake) ID() string { return f.id }
func (f fake) Load(context.Context, string, string, string) (map[string]string, error) {
	return map[string]string{f.id: "ok"}, nil
}

func TestFind(t *testing.T) {
	registry := MustRegistry(
		fake{id: "infisical"},
	)

	if l := registry.Find("infisical"); l == nil || l.ID() != "infisical" {
		t.Errorf("Find(infisical) returned %v", l)
	}
	if l := registry.Find("nope"); l != nil {
		t.Errorf("Find for unknown id should return nil, got %v", l)
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	if _, err := NewRegistry(fake{id: "infisical"}, fake{id: "infisical"}); err == nil {
		t.Fatal("expected duplicate loader IDs to fail")
	}
}

type countingLoader struct {
	dirs []string
	fail error
}

func (l *countingLoader) ID() string { return "fixture" }
func (l *countingLoader) Load(_ context.Context, root, dir, env string) (map[string]string, error) {
	l.dirs = append(l.dirs, dir)
	return map[string]string{"root": root, "dir": dir, "env": env}, l.fail
}
func TestLoadProjectsFallback(t *testing.T) {
	l := &countingLoader{}
	got, err := LoadProjects(context.Background(), l, "workspace", []string{"apps/a", "apps/b", "apps/a"}, "staging")
	if err != nil || len(got) != 2 || !reflect.DeepEqual(l.dirs, []string{"apps/a", "apps/b"}) || got["apps/b"]["env"] != "staging" {
		t.Fatalf("%v %v %v", got, l.dirs, err)
	}
	failure := errors.New("no snapshot")
	l = &countingLoader{fail: failure}
	got, err = LoadProjects(context.Background(), l, "workspace", []string{"apps/a", "apps/b"}, "dev")
	if got != nil || !errors.Is(err, failure) || len(l.dirs) != 1 {
		t.Fatal("fallback continued after failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = LoadProjects(ctx, l, "workspace", []string{"apps/b"}, "dev")
	if got != nil || !errors.Is(err, context.Canceled) || len(l.dirs) != 1 {
		t.Fatal("fallback ignored cancellation")
	}
}

// Single-project loaders may reuse their buffer; capture it before the next load.
type reuseLoader struct{ values map[string]string }

func (l reuseLoader) ID() string { return "reuse" }
func (l reuseLoader) Load(_ context.Context, _, dir, _ string) (map[string]string, error) {
	l.values["DIR"] = dir
	return l.values, nil
}
func TestLoadProjectsFallbackIsolatesReusedMaps(t *testing.T) {
	got, err := LoadProjects(context.Background(), reuseLoader{map[string]string{}}, "workspace", []string{"apps/a", "apps/b"}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got["apps/a"]["DIR"] != "apps/a" || got["apps/b"]["DIR"] != "apps/b" {
		t.Fatal("fallback overwrote an earlier snapshot")
	}
	got["apps/a"]["DIR"] = "changed"
	if got["apps/b"]["DIR"] != "apps/b" {
		t.Fatal("fallback project maps alias")
	}
}
