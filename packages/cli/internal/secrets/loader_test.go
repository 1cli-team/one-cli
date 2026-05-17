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

func TestRegisterSortsByPriorityDescending(t *testing.T) {
	t.Cleanup(Reset)
	Reset()

	// Register out of order; All() must come back priority-desc.
	Register(fake{id: "low", priority: 5})
	Register(fake{id: "high", priority: 100})
	Register(fake{id: "mid", priority: 50})

	got := []string{}
	for _, l := range All() {
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
	t.Cleanup(Reset)
	Reset()
	Register(fake{id: "infisical", priority: 100})
	Register(fake{id: "dotenv", priority: 10})

	if l := Find("dotenv"); l == nil || l.ID() != "dotenv" {
		t.Errorf("Find(dotenv) returned %v", l)
	}
	if l := Find("nope"); l != nil {
		t.Errorf("Find for unknown id should return nil, got %v", l)
	}
}

func TestPickAvailableSkipsUnavailable(t *testing.T) {
	t.Cleanup(Reset)
	Reset()
	Register(fake{id: "infisical", priority: 100, available: false})
	Register(fake{id: "dotenv", priority: 10, available: true})

	got := PickAvailable("/whatever")
	if got == nil || got.ID() != "dotenv" {
		t.Fatalf("PickAvailable should fall through to dotenv, got %v", got)
	}
}

func TestPickAvailableReturnsHighestPriorityWhenBothAvailable(t *testing.T) {
	t.Cleanup(Reset)
	Reset()
	Register(fake{id: "infisical", priority: 100, available: true})
	Register(fake{id: "dotenv", priority: 10, available: true})

	got := PickAvailable("/whatever")
	if got == nil || got.ID() != "infisical" {
		t.Fatalf("highest priority should win, got %v", got)
	}
}

func TestPickAvailableReturnsNilWhenAllUnavailable(t *testing.T) {
	t.Cleanup(Reset)
	Reset()
	Register(fake{id: "infisical", priority: 100, available: false})
	Register(fake{id: "dotenv", priority: 10, available: false})

	if got := PickAvailable("/whatever"); got != nil {
		t.Fatalf("all-unavailable should yield nil, got %v", got)
	}
}
