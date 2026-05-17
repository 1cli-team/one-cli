package infisical

// Coverage for the pure helpers in init.go that don't need an Infisical
// server. The auto-create flow (Init itself) is exercised end-to-end
// against an httptest backend in client_projects_test.go; here we pin
// the supporting logic that decides defaults, project names, and the
// manifest back-fill behavior.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestApplyInitDefaults(t *testing.T) {
	cases := []struct {
		name string
		in   InitInput
		want InitInput
	}{
		{
			name: "empty input fills every default",
			in:   InitInput{},
			want: InitInput{
				Environments: append([]string{}, DefaultEnvironments...),
				DefaultEnv:   DefaultEnvironments[0],
				RootPath:     "/",
			},
		},
		{
			name: "explicit environments pin DefaultEnv to first when DefaultEnv blank",
			in: InitInput{
				Environments: []string{"qa", "prod"},
			},
			want: InitInput{
				Environments: []string{"qa", "prod"},
				DefaultEnv:   "qa",
				RootPath:     "/",
			},
		},
		{
			name: "explicit DefaultEnv preserved",
			in: InitInput{
				Environments: []string{"qa", "prod"},
				DefaultEnv:   "prod",
			},
			want: InitInput{
				Environments: []string{"qa", "prod"},
				DefaultEnv:   "prod",
				RootPath:     "/",
			},
		},
		{
			name: "explicit RootPath preserved",
			in: InitInput{
				RootPath: "/teams/web",
			},
			want: InitInput{
				Environments: append([]string{}, DefaultEnvironments...),
				DefaultEnv:   DefaultEnvironments[0],
				RootPath:     "/teams/web",
			},
		},
		{
			name: "whitespace-only DefaultEnv treated as blank",
			in: InitInput{
				Environments: []string{"qa", "prod"},
				DefaultEnv:   "   ",
			},
			want: InitInput{
				Environments: []string{"qa", "prod"},
				DefaultEnv:   "qa",
				RootPath:     "/",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyInitDefaults(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\n  want %+v", got, tc.want)
			}
		})
	}
}

func TestRandomSuffix_DeterministicLength(t *testing.T) {
	for _, n := range []int{4, 8, 12} {
		got := randomSuffix(n)
		if len(got) != n {
			t.Errorf("randomSuffix(%d) length = %d", n, len(got))
		}
	}
}

func TestRandomSuffix_HexAlphabet(t *testing.T) {
	got := randomSuffix(16)
	for i, c := range got {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			t.Errorf("randomSuffix produced non-hex char %q at offset %d (full: %q)", c, i, got)
		}
	}
}

func TestRandomSuffix_LowCollisionRate(t *testing.T) {
	// Cheap statistical sanity check: 200 calls of 4-char hex (16-bit
	// space) should produce way more than ~10 unique values. If random
	// is silently broken (e.g. always-zero fallback), this catches it.
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[randomSuffix(4)] = true
	}
	if len(seen) < 100 {
		t.Errorf("randomSuffix appears non-random: only %d unique values from 200 calls", len(seen))
	}
}

func TestDedupeStrings(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "empty", in: nil, want: []string{}},
		{name: "no dups, no whitespace", in: []string{"a", "b", "c"}, want: []string{"a", "b", "c"}},
		{name: "drop exact dup", in: []string{"a", "a", "b"}, want: []string{"a", "b"}},
		{name: "trim whitespace before dedup", in: []string{"a", "  a  ", "b"}, want: []string{"a", "b"}},
		{name: "drop blank entries", in: []string{"", "  ", "x", ""}, want: []string{"x"}},
		{name: "preserve first-seen order", in: []string{"c", "a", "c", "b"}, want: []string{"c", "a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeStrings(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveProjectName_OverrideWins(t *testing.T) {
	tmp := t.TempDir()
	got, err := resolveProjectName(tmp, "explicit-name")
	if err != nil {
		t.Fatal(err)
	}
	if got != "explicit-name" {
		t.Errorf("override should win: got %q", got)
	}
}

func TestResolveProjectName_OverrideTrimsWhitespace(t *testing.T) {
	tmp := t.TempDir()
	got, err := resolveProjectName(tmp, "   spaced-name   ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "spaced-name" {
		t.Errorf("override should trim: got %q", got)
	}
}

func TestResolveProjectName_FromManifest(t *testing.T) {
	tmp := t.TempDir()
	mustWriteManifest(t, tmp, &workspace.Manifest{
		Version:   workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{ID: "ws_abc", Name: "manifest-name"},
		Projects:  []workspace.ManifestProject{},
	})

	got, err := resolveProjectName(tmp, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "manifest-name" {
		t.Errorf("manifest should win when override empty: got %q", got)
	}
}

func TestResolveProjectName_FallsBackToPackageJSON(t *testing.T) {
	tmp := t.TempDir()
	mustWritePackageJSON(t, tmp, `{"name":"pkg-name","version":"0.1.0"}`)

	got, err := resolveProjectName(tmp, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pkg-name" {
		t.Errorf("package.json should be used when manifest empty: got %q", got)
	}
}

func TestResolveProjectName_FallsBackToDirBasename(t *testing.T) {
	parent := t.TempDir()
	tmp := filepath.Join(parent, "billing-platform")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveProjectName(tmp, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "billing-platform" {
		t.Errorf("dir basename fallback: got %q", got)
	}
}

func TestResolveProjectName_PrecedenceOrder(t *testing.T) {
	// All three sources present — manifest must beat package.json must
	// beat dir basename.
	parent := t.TempDir()
	tmp := filepath.Join(parent, "dirname-loses")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWritePackageJSON(t, tmp, `{"name":"pkgname-loses"}`)
	mustWriteManifest(t, tmp, &workspace.Manifest{
		Version:   workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{ID: "ws_x", Name: "manifest-wins"},
		Projects:  []workspace.ManifestProject{},
	})

	got, err := resolveProjectName(tmp, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "manifest-wins" {
		t.Errorf("precedence broken: got %q", got)
	}
}

func TestEnsureManifestProject_BackfillsMissingBlock(t *testing.T) {
	tmp := t.TempDir()
	// Write a v2 manifest with NO Project block.
	mustWriteManifest(t, tmp, &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{},
	})

	if err := ensureManifestProject(tmp, "auto-name"); err != nil {
		t.Fatal(err)
	}

	loaded, err := workspace.ReadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Workspace == nil {
		t.Fatal("Project block not back-filled")
	}
	if loaded.Workspace.Name != "auto-name" {
		t.Errorf("backfill name: want auto-name, got %q", loaded.Workspace.Name)
	}
	if loaded.Workspace.ID == "" {
		t.Error("backfill ID should be non-empty (GenerateProjectID)")
	}
}

func TestEnsureManifestProject_PreservesExisting(t *testing.T) {
	tmp := t.TempDir()
	mustWriteManifest(t, tmp, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{
			ID: "ws_preexisting", Name: "preexisting-name",
		},
		Projects: []workspace.ManifestProject{},
	})

	if err := ensureManifestProject(tmp, "ignored-fallback"); err != nil {
		t.Fatal(err)
	}

	loaded, _ := workspace.ReadManifest(tmp)
	if loaded.Workspace.ID != "ws_preexisting" {
		t.Errorf("ID should not be overwritten: got %q", loaded.Workspace.ID)
	}
	if loaded.Workspace.Name != "preexisting-name" {
		t.Errorf("Name should not be overwritten: got %q", loaded.Workspace.Name)
	}
}

func TestEnsureManifestProject_BackfillsMissingIDOnly(t *testing.T) {
	// Project block has Name but no ID — back-fill the ID, keep the Name.
	tmp := t.TempDir()
	mustWriteManifest(t, tmp, &workspace.Manifest{
		Version:   workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{ID: "", Name: "name-was-set"},
		Projects:  []workspace.ManifestProject{},
	})

	if err := ensureManifestProject(tmp, "fallback-name"); err != nil {
		t.Fatal(err)
	}

	loaded, _ := workspace.ReadManifest(tmp)
	if loaded.Workspace.Name != "name-was-set" {
		t.Errorf("Name should be preserved: got %q", loaded.Workspace.Name)
	}
	if loaded.Workspace.ID == "" {
		t.Error("ID should be back-filled")
	}
}

// --- helpers ---

func mustWriteManifest(t *testing.T, dir string, m *workspace.Manifest) {
	t.Helper()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, workspace.ManifestFilename), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWritePackageJSON(t *testing.T, dir string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
