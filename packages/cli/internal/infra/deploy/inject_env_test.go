package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// stubLoader implements secrets.Loader for testing LoadInjectionEnv
// without touching disk or network.
type stubLoader struct {
	id        string
	priority  secrets.Priority
	available bool
	vars      map[string]string
	err       error
}

func (s *stubLoader) ID() string                        { return s.id }
func (s *stubLoader) Priority() secrets.Priority        { return s.priority }
func (s *stubLoader) Available(projectRoot string) bool { return s.available }
func (s *stubLoader) Load(_ context.Context, _, _, _ string) (map[string]string, error) {
	return s.vars, s.err
}

// writeManifest writes a minimal manifest via WriteManifest so the
// version stays in lockstep with workspace.ManifestVersion.
func writeManifest(t *testing.T, dir, projectName string, envDisabled bool) {
	t.Helper()
	mp := workspace.ManifestProject{
		Name:        projectName,
		RelativeDir: "apps/web",
		TemplateID:  "node-pkg",
		Toolchain:   "node",
	}
	if envDisabled {
		mp.Domains = &workspace.ProjectDomains{
			Env: &workspace.ProjectEnvOverride{Disabled: true},
		}
	}
	m := &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{mp},
	}
	if err := workspace.WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
}

func baseInput(t *testing.T, root, projectName string) ApplyInput {
	t.Helper()
	m, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	return ApplyInput{
		ProjectRoot: root,
		Project:     workspace.Project{Name: projectName, RelativeDir: "apps/web"},
		Manifest:    m,
	}
}

func TestLoadInjectionEnv_HappyPath(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	secrets.Register(&stubLoader{
		id:        "stub",
		priority:  secrets.PriorityFilesystem,
		available: true,
		vars:      map[string]string{"API_URL": "https://api.example.com", "FEATURE_FLAGS": "a,b"},
	})

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	res, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Source != "stub" {
		t.Errorf("Source = %q, want stub", res.Source)
	}
	if len(res.Vars) != 2 {
		t.Errorf("Vars len = %d, want 2", len(res.Vars))
	}
	wantKeys := []string{"API_URL", "FEATURE_FLAGS"}
	if len(res.Keys) != len(wantKeys) {
		t.Fatalf("Keys = %v, want %v", res.Keys, wantKeys)
	}
	for i, k := range wantKeys {
		if res.Keys[i] != k {
			t.Errorf("Keys[%d] = %q, want %q (Keys must be sorted)", i, res.Keys[i], k)
		}
	}
}

func TestLoadInjectionEnv_PerProjectDisabled(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	secrets.Register(&stubLoader{id: "stub", priority: secrets.PriorityFilesystem, available: true, vars: map[string]string{"X": "y"}})

	root := t.TempDir()
	writeManifest(t, root, "web", true) // env.disabled = true
	in := baseInput(t, root, "web")

	res, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("project-level disabled should return nil, got %+v", res)
	}
}

func TestLoadInjectionEnv_EmptyVarsDowngrades(t *testing.T) {
	// A loader that returns (empty map, nil) — e.g. dotenv when no
	// .env file exists, or any backend with a project that has no
	// configured secrets — must downgrade to a nil InjectionResult so
	// `one deploy` proceeds without injection rather than failing.
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	secrets.Register(&stubLoader{id: "dotenv", priority: secrets.PriorityFilesystem, available: true, vars: map[string]string{}})

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	res, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err != nil {
		t.Fatalf("empty vars should downgrade to nil, got error: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result, got %+v", res)
	}
}

func TestLoadInjectionEnv_OtherErrorBubbles(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	netErr := errors.New("infisical: connection refused")
	secrets.Register(&stubLoader{id: "infisical", priority: secrets.PriorityRemoteBackend, available: true, err: netErr})

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	_, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err == nil {
		t.Fatal("expected non-dotenv-missing error to bubble, got nil")
	}
	if !errors.Is(err, netErr) {
		t.Errorf("expected wrapped netErr, got %v", err)
	}
}

func TestLoadInjectionEnv_NoLoaderAvailable(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	// No loaders registered.

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	res, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err != nil {
		t.Fatalf("no loader should be benign, got error: %v", err)
	}
	if res != nil {
		t.Errorf("no loader should return nil, got %+v", res)
	}
}

func TestLoadInjectionEnv_UnknownLoaderID(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	secrets.Register(&stubLoader{id: "dotenv", priority: secrets.PriorityFilesystem, available: true})

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	_, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{LoaderID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown loader id, got nil")
	}
	type coded interface{ ErrorCode() string }
	if c, ok := err.(coded); !ok || c.ErrorCode() != "BACKEND_NOT_ENABLED" {
		t.Errorf("expected BACKEND_NOT_ENABLED, got %v", err)
	}
}

func TestLoadInjectionEnv_EmptyVarsReturnsNil(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	secrets.Register(&stubLoader{id: "stub", priority: secrets.PriorityFilesystem, available: true, vars: map[string]string{}})

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	res, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("zero vars should return nil (no point dispatching empty map), got %+v", res)
	}
}

func TestLoadInjectionEnv_InvalidKeyRejected(t *testing.T) {
	t.Cleanup(secrets.Reset)
	secrets.Reset()
	secrets.Register(&stubLoader{
		id:        "stub",
		priority:  secrets.PriorityFilesystem,
		available: true,
		vars:      map[string]string{"VALID": "1", "with-dash": "2"},
	})

	root := t.TempDir()
	writeManifest(t, root, "web", false)
	in := baseInput(t, root, "web")

	_, err := LoadInjectionEnv(context.Background(), in, LoadInjectionOptions{})
	if err == nil {
		t.Fatal("expected validation error for non-POSIX key, got nil")
	}
}
