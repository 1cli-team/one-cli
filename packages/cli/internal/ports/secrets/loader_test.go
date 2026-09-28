package secrets

import (
	"context"
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
