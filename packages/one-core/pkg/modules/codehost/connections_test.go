package codehost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyGiteaConnection(t *testing.T) {
	const repository = "team/repo"
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user":
			_, _ = w.Write([]byte(`{"login":"fixture-user"}`))
		case "/api/v1/repos/" + repository:
			_, _ = w.Write([]byte(`{"full_name":"Team/Repo"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()

	for _, tt := range []struct {
		name       string
		repository string
		account    string
		want       error
	}{
		{name: "account", account: "fixture-user"},
		{name: "repository", repository: repository},
		{name: "inaccessible repository", repository: "team/missing", want: ErrRepositoryAccess},
		{name: "invalid repository", repository: "../repo", want: ErrInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ONE_HOME", t.TempDir())
			service := Service{}
			got, err := service.Save(t.Context(), Input{URL: remote.URL, Token: "fixture-token", Repository: tt.repository})
			if err != tt.want {
				t.Fatalf("save = %v, want %v", err, tt.want)
			}
			if err != nil {
				return
			}
			verified, err := service.Test(t.Context(), got.Host)
			if err != nil || verified != got || verified.Repository != tt.repository || verified.Account != tt.account {
				t.Fatalf("verified connection = %+v, %v", verified, err)
			}
		})
	}
}

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
