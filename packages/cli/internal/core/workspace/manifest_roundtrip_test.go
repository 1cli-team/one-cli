package workspace

// Manifest persistence preserves project overrides, canonical ordering, and deterministic bytes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest_ReadOnMissing_ReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	m, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest on missing: %v", err)
	}
	if m.Version != ManifestVersion {
		t.Errorf("Version: want %d, got %d", ManifestVersion, m.Version)
	}
	if len(m.Projects) != 0 {
		t.Errorf("Subprojects: want empty, got %d entries", len(m.Projects))
	}
}

func TestManifest_WriteReadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	original := &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{
			{
				Name:        "user-api",
				RelativeDir: "services/user-api",
				TemplateID:  "go-api",
				Toolchain:   "go",
			},
		},
	}
	if err := WriteManifest(tmp, original); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	loaded, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if loaded.Version != ManifestVersion {
		t.Errorf("Version: want %d, got %d", ManifestVersion, loaded.Version)
	}
	if len(loaded.Projects) != 1 {
		t.Fatalf("Subprojects: want 1, got %d", len(loaded.Projects))
	}
	sub := loaded.Projects[0]
	if sub.Name != "user-api" || sub.TemplateID != "go-api" || sub.Toolchain != "go" {
		t.Errorf("subproject identity changed across round-trip: %+v", sub)
	}
	if sub.BuildVersion != DefaultBuildVersion {
		t.Errorf("BuildVersion: want %s, got %q", DefaultBuildVersion, sub.BuildVersion)
	}
}

func TestManifest_WriteIsByteDeterministic(t *testing.T) {
	tmp := t.TempDir()

	m := &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{
			{Name: "a", RelativeDir: "services/a", TemplateID: "go-api", Toolchain: "go"},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(tmp, ManifestFilename))

	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(tmp, ManifestFilename))

	if string(first) != string(second) {
		t.Errorf("identical input produced different bytes:\n  first:\n%s\n  second:\n%s", first, second)
	}
}

func TestManifestRetiresDevCommandAndPreservesServiceURL(t *testing.T) {
	root := t.TempDir()
	raw := []byte(`{"version":1,"projects":[{"name":"api","relativeDir":"services/api","toolchain":"node","dev":{"command":"echo retired","url":"http://localhost:3000/"}}]}`)
	if err := os.WriteFile(filepath.Join(root, ManifestFilename), raw, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Projects[0].Service == nil || m.Projects[0].Service.URL != "http://localhost:3000/" {
		t.Fatal(m.Projects[0])
	}
	if err = WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "retired") || strings.Contains(string(after), `"dev"`) {
		t.Fatal("retired execution metadata was persisted")
	}
}

func TestManifest_WriteReadPreservesOverrides(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{Projects: []ManifestProject{{
		Name: "billing", RelativeDir: "services/billing", TemplateID: "go-api", Toolchain: "go",
		BuildVersion: "1.2.3",
		Env:          &ProjectEnvOverride{Path: "/teams/payments/billing", Keys: []string{"DATABASE_URL"}},
	}}}); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	loaded, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(loaded.Projects) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Projects))
	}
	sub := loaded.Projects[0]
	if sub.Env == nil {
		t.Fatalf("env override lost on round-trip")
	}
	if sub.Env.Path != "/teams/payments/billing" {
		t.Errorf("env.path drifted on round-trip: got %q", sub.Env.Path)
	}
	if len(sub.Env.Keys) != 1 || sub.Env.Keys[0] != "DATABASE_URL" {
		t.Errorf("env.keys drifted on round-trip: got %v", sub.Env.Keys)
	}
	if sub.BuildVersion != "1.2.3" {
		t.Errorf("buildVersion drifted on round-trip: got %q", sub.BuildVersion)
	}
}

func TestManifest_SubprojectsAlwaysSortedByRelativeDir(t *testing.T) {
	tmp := t.TempDir()

	// Persist projects out of order; expect sorted output.
	m := &Manifest{}
	for _, name := range []string{"zeta", "alpha", "mu"} {
		m.Projects = append(m.Projects, ManifestProject{
			Name: name, RelativeDir: "services/" + name, TemplateID: "go-api", Toolchain: "go",
		})
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	wantOrder := []string{"services/alpha", "services/mu", "services/zeta"}
	for i, want := range wantOrder {
		if loaded.Projects[i].RelativeDir != want {
			t.Errorf("sort order [%d]: want %q, got %q", i, want, loaded.Projects[i].RelativeDir)
		}
	}
}

func TestManifest_RecordProjectEnvKey(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{Projects: []ManifestProject{{
		Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go",
	}}}); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	// Add two keys, dedupe attempt, sorted result.
	for _, k := range []string{"DATABASE_URL", "JWT_SECRET", "DATABASE_URL"} {
		if err := RecordProjectEnvKey(tmp, "api", k); err != nil {
			t.Fatalf("record %s: %v", k, err)
		}
	}
	m, _ := ReadManifest(tmp)
	if m.Projects[0].Env == nil {
		t.Fatalf("env override missing")
	}
	got := m.Projects[0].Env.Keys
	want := []string{"DATABASE_URL", "JWT_SECRET"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("env.keys: want %v, got %v", want, got)
	}

	// Unknown subproject name → no error, no write.
	if err := RecordProjectEnvKey(tmp, "ghost", "X"); err != nil {
		t.Errorf("unknown subproject should be silent, got: %v", err)
	}
}
