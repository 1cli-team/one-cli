package codehost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestImportLegacyPreservesCurrentCodeHostToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONE_HOME", home)
	state := t.TempDir()
	input := Input{URL: "https://git.example.invalid", Token: "fixture-legacy-token"}
	old, ok := parseInput(input)
	if !ok {
		t.Fatal("invalid fixture")
	}
	data, err := json.Marshal(map[string]credential{old.Host: old})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "code-hosts.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	s := Service{}
	if err := s.ImportLegacy(t.Context(), state); err != nil {
		t.Fatal(err)
	}
	got, err := s.connection(old.Host)
	if err != nil || got.Token != input.Token {
		t.Fatal("did not import the previous connection")
	}
	if err := update(t.Context(), func(values map[string]credential) (bool, error) {
		got.Token = "fixture-updated-token"
		values[old.Host] = got
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportLegacy(t.Context(), state); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.connection(old.Host)
	if err != nil || reloaded.Token != got.Token {
		t.Fatal("repeated migration did not preserve the current connection")
	}
}
