package secrets

import (
	"context"
	"testing"
)

type fake struct {
	id        string
	priority  Priority
	available bool
}

func (f fake) ID() string            { return f.id }
func (f fake) Priority() Priority    { return f.priority }
func (f fake) Available(string) bool { return f.available }
func (f fake) Load(context.Context, string, string, string) (map[string]string, error) {
	return map[string]string{f.id: "ok"}, nil
}

func TestRegistrySortsByPriorityDescending(t *testing.T) {
	registry := MustRegistry(
		fake{id: "low", priority: 5},
		fake{id: "high", priority: 100},
		fake{id: "mid", priority: 50},
	)

	got := []string{}
	for _, l := range registry.All() {
		got = append(got, l.ID())
	}
	want := []string{"high", "mid", "low"}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("priority order: got %v, want %v", got, want)
		}
	}
}

func TestFind(t *testing.T) {
	registry := MustRegistry(
		fake{id: "infisical", priority: 100},
		fake{id: "dotenv", priority: 10},
	)

	if l := registry.Find("dotenv"); l == nil || l.ID() != "dotenv" {
		t.Errorf("Find(dotenv) returned %v", l)
	}
	if l := registry.Find("nope"); l != nil {
		t.Errorf("Find for unknown id should return nil, got %v", l)
	}
}

func TestPickAvailableSkipsUnavailable(t *testing.T) {
	registry := MustRegistry(
		fake{id: "infisical", priority: 100, available: false},
		fake{id: "dotenv", priority: 10, available: true},
	)

	got := registry.PickAvailable("/whatever")
	if got == nil || got.ID() != "dotenv" {
		t.Fatalf("PickAvailable should fall through to dotenv, got %v", got)
	}
}

func TestPickAvailableReturnsHighestPriorityWhenBothAvailable(t *testing.T) {
	registry := MustRegistry(
		fake{id: "infisical", priority: 100, available: true},
		fake{id: "dotenv", priority: 10, available: true},
	)

	got := registry.PickAvailable("/whatever")
	if got == nil || got.ID() != "infisical" {
		t.Fatalf("highest priority should win, got %v", got)
	}
}

func TestPickAvailableReturnsNilWhenAllUnavailable(t *testing.T) {
	registry := MustRegistry(
		fake{id: "infisical", priority: 100, available: false},
		fake{id: "dotenv", priority: 10, available: false},
	)

	if got := registry.PickAvailable("/whatever"); got != nil {
		t.Fatalf("all-unavailable should yield nil, got %v", got)
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	if _, err := NewRegistry(fake{id: "dotenv"}, fake{id: "dotenv"}); err == nil {
		t.Fatal("expected duplicate loader IDs to fail")
	}
}
