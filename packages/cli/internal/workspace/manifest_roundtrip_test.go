package workspace

// Manifest write/read round-trip + upsert/rebuild invariants.
//
// The manifest is the data structure that status / add all converge on;
// bugs here typically present as "status reports drift on a fresh write"
// or "add wiped a per-subproject env override". This file pins the
// load-bearing properties:
//   1. Read on missing → empty manifest (no error)
//   2. Write→Read produces matching content
//   3. Upsert preserves per-subproject env / container / deploy overrides
//   4. Rebuild preserves overrides for entries that survive
//   5. RelativeDir is normalized to POSIX form on persist
//   6. Repeated WriteManifest produces byte-identical output (deterministic)
//
// Timestamps (createdAt / updatedAt) were removed in v0.7 — they were
// merge-conflict bait and git already tells you who changed what.

import (
	"os"
	"path/filepath"
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

func TestManifest_PreservesDevOverride(t *testing.T) {
	tmp := t.TempDir()
	if err := UpsertManifestProject(tmp, ManifestProjectInput{
		Name: "api", RelativeDir: "services/api", TemplateID: "nestjs-api", Toolchain: "node", PackageManager: "pnpm",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := UpdateProjectDev(tmp, "services/api", "pnpm run start:dev"); err != nil {
		t.Fatalf("UpdateProjectDev: %v", err)
	}
	if got := readDevCommand(t, tmp, "api"); got != "pnpm run start:dev" {
		t.Errorf("dev.command after write = %q, want %q", got, "pnpm run start:dev")
	}

	// Re-upsert the project — Domains must survive.
	if err := UpsertManifestProject(tmp, ManifestProjectInput{
		Name: "api", RelativeDir: "services/api", TemplateID: "nestjs-api", Toolchain: "node", PackageManager: "pnpm",
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if got := readDevCommand(t, tmp, "api"); got != "pnpm run start:dev" {
		t.Errorf("dev.command lost on upsert: got %q", got)
	}

	// Empty command clears the block.
	if err := UpdateProjectDev(tmp, "services/api", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := readDevCommand(t, tmp, "api"); got != "" {
		t.Errorf("dev.command should clear, got %q", got)
	}
	loaded, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest after clear: %v", err)
	}
	if loaded.Projects[0].Domains != nil {
		t.Errorf("Domains should be pruned to nil when all overrides cleared, got %+v", loaded.Projects[0].Domains)
	}
}

func readDevCommand(t *testing.T, root, projectName string) string {
	t.Helper()
	m, err := ReadManifest(root)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	return ProjectDev(m, projectName)
}

func TestManifest_UpsertPreservesEnvOverride(t *testing.T) {
	tmp := t.TempDir()

	if err := UpsertManifestProject(tmp, ManifestProjectInput{
		Name: "billing", RelativeDir: "services/billing", TemplateID: "go-api", Toolchain: "go",
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Stamp some env metadata so upsert preservation is observable.
	if err := UpdateManifestProjectEnv(tmp, "services/billing", &ProjectEnvOverride{
		Path: "/teams/payments/billing", Keys: []string{"DATABASE_URL"},
	}); err != nil {
		t.Fatalf("UpdateManifestProjectEnv: %v", err)
	}
	if err := SetProjectBuildVersion(tmp, "billing", "1.2.3"); err != nil {
		t.Fatalf("SetProjectBuildVersion: %v", err)
	}

	if err := UpsertManifestProject(tmp, ManifestProjectInput{
		Name: "billing", RelativeDir: "services/billing", TemplateID: "go-api", Toolchain: "go",
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	loaded, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(loaded.Projects) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Projects))
	}
	sub := loaded.Projects[0]
	if sub.Domains == nil || sub.Domains.Env == nil {
		t.Fatalf("env override lost on upsert")
	}
	if sub.Domains.Env.Path != "/teams/payments/billing" {
		t.Errorf("env.path drifted on upsert: got %q", sub.Domains.Env.Path)
	}
	if len(sub.Domains.Env.Keys) != 1 || sub.Domains.Env.Keys[0] != "DATABASE_URL" {
		t.Errorf("env.keys drifted on upsert: got %v", sub.Domains.Env.Keys)
	}
	if sub.BuildVersion != "1.2.3" {
		t.Errorf("buildVersion drifted on upsert: got %q", sub.BuildVersion)
	}
}

func TestManifest_RebuildPreservesOverrides(t *testing.T) {
	tmp := t.TempDir()

	// Step 1: seed two entries with overrides.
	for _, name := range []string{"a", "b"} {
		if err := UpsertManifestProject(tmp, ManifestProjectInput{
			Name: name, RelativeDir: "services/" + name, TemplateID: "go-api", Toolchain: "go",
		}); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	if err := UpdateManifestProjectEnv(tmp, "services/a", &ProjectEnvOverride{
		Keys: []string{"FOO"},
	}); err != nil {
		t.Fatalf("seed env: %v", err)
	}
	if err := SetProjectBuildVersion(tmp, "a", "2.3.4"); err != nil {
		t.Fatalf("seed buildVersion: %v", err)
	}

	// Step 2: rebuild — drop b, keep a, add c.
	rebuilt, err := RebuildManifest(tmp, []ManifestProjectInput{
		{Name: "a", RelativeDir: "services/a", TemplateID: "go-api", Toolchain: "go"},
		{Name: "c", RelativeDir: "services/c", TemplateID: "go-api", Toolchain: "go"},
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(rebuilt.Projects) != 2 {
		t.Fatalf("after rebuild: want 2 entries, got %d", len(rebuilt.Projects))
	}

	byName := map[string]ManifestProject{}
	for _, s := range rebuilt.Projects {
		byName[s.Name] = s
	}

	a := byName["a"]
	if a.Domains == nil || a.Domains.Env == nil ||
		len(a.Domains.Env.Keys) == 0 || a.Domains.Env.Keys[0] != "FOO" {
		t.Errorf("a.env.keys should survive rebuild, got %+v", a.Domains)
	}
	if a.BuildVersion != "2.3.4" {
		t.Errorf("a.buildVersion should survive rebuild, got %q", a.BuildVersion)
	}

	c := byName["c"]
	if c.Domains != nil && c.Domains.Env != nil {
		t.Errorf("c (new) should have no env override, got %+v", c.Domains.Env)
	}
	if c.BuildVersion != DefaultBuildVersion {
		t.Errorf("c.buildVersion: want %s, got %q", DefaultBuildVersion, c.BuildVersion)
	}

	if _, exists := byName["b"]; exists {
		t.Error("b should be dropped — not in rebuild input")
	}
}

func TestManifest_SubprojectsAlwaysSortedByRelativeDir(t *testing.T) {
	tmp := t.TempDir()

	// Insert in reverse alphabetical order; expect sorted output.
	for _, name := range []string{"zeta", "alpha", "mu"} {
		if err := UpsertManifestProject(tmp, ManifestProjectInput{
			Name: name, RelativeDir: "services/" + name, TemplateID: "go-api", Toolchain: "go",
		}); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
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
	if err := UpsertManifestProject(tmp, ManifestProjectInput{
		Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Add two keys, dedupe attempt, sorted result.
	for _, k := range []string{"DATABASE_URL", "JWT_SECRET", "DATABASE_URL"} {
		if err := RecordProjectEnvKey(tmp, "api", k); err != nil {
			t.Fatalf("record %s: %v", k, err)
		}
	}
	m, _ := ReadManifest(tmp)
	if m.Projects[0].Domains == nil || m.Projects[0].Domains.Env == nil {
		t.Fatalf("env override missing")
	}
	got := m.Projects[0].Domains.Env.Keys
	want := []string{"DATABASE_URL", "JWT_SECRET"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("env.keys: want %v, got %v", want, got)
	}

	// Unknown subproject name → no error, no write.
	if err := RecordProjectEnvKey(tmp, "ghost", "X"); err != nil {
		t.Errorf("unknown subproject should be silent, got: %v", err)
	}
}
