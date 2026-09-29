package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func TestManifest_RoundTrip(t *testing.T) {
	tmp := t.TempDir()

	disabled := true
	inheritsTrue := true
	original := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo-id", Name: "demo"},
		Environments: &Environments{
			Names:   []string{"dev", "prod"},
			Default: "dev",
		},

		Env: &EnvironmentConfig{ProjectID: "proj-123", RootPath: "/", ProjectName: "demo"},

		Projects: []ManifestProject{
			{
				Name:        "api",
				RelativeDir: "services/api",
				TemplateID:  "go-api",
				Toolchain:   "go",

				Env: &ProjectEnvOverride{Disabled: disabled},
			},
			{
				Name:           "web",
				RelativeDir:    "apps/web",
				TemplateID:     "react-spa",
				Toolchain:      "node",
				PackageManager: "pnpm",

				Env: &ProjectEnvOverride{
					Path:     "/web-secrets",
					Inherits: &inheritsTrue,
				},
			},
		},
	}

	if err := WriteManifest(tmp, original); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.Version != ManifestVersion {
		t.Errorf("version = %d; want %d", got.Version, ManifestVersion)
	}
	if got.Workspace == nil || got.Workspace.ID != "demo-id" || got.Workspace.Name != "demo" {
		t.Errorf("workspace identity not preserved: %+v", got.Workspace)
	}
	if got.Environments == nil || len(got.Environments.Names) != 2 || got.Environments.Default != "dev" {
		t.Errorf("environments not preserved: %+v", got.Environments)
	}
	if got.Env == nil || EnvBackend(got) != EnvBackendInfisical {
		t.Errorf("env backend not preserved: %+v", got.Env)
	}
	if len(got.Projects) != 2 {
		t.Fatalf("projects count = %d; want 2", len(got.Projects))
	}
	// Sorted by relativeDir, so apps/web comes first.
	web := got.Projects[0]
	if web.Env == nil || web.Env.Path != "/web-secrets" {
		t.Errorf("apps/web env override not preserved: %+v", web.Env)
	}
	api := got.Projects[1]
	if api.Env == nil || !api.Env.Disabled {
		t.Errorf("services/api env.disabled not preserved: %+v", api.Env)
	}
}

func TestManifest_LegacyFieldsRejected(t *testing.T) {
	tmp := t.TempDir()
	v1 := []byte(`{"version":1,"updatedAt":"2026-01-01T00:00:00.000Z","subprojects":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(tmp); err == nil {
		t.Fatal("expected ReadManifest to reject legacy fields; got nil err")
	}
}

func TestManifest_V2Rejected(t *testing.T) {
	tmp := t.TempDir()
	// v2 manifest used the old `subprojects` key and is no longer accepted
	// after the v0.7 hard rename to `projects`.
	v2 := []byte(`{"version":2,"subprojects":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), v2, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(tmp); err == nil {
		t.Fatal("expected ReadManifest to reject v2; got nil err")
	}
}

func TestManifest_V4Rejected(t *testing.T) {
	tmp := t.TempDir()
	v4 := []byte(`{"version":4,"projects":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), v4, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(tmp); err == nil {
		t.Fatal("expected ReadManifest to reject v4; got nil err")
	}
}

func TestInitWorkspaceEnv_PreservesProjects(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name: "api", RelativeDir: "services/api",
			TemplateID: "go-api", Toolchain: "go",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	cfgRaw, _ := json.Marshal(map[string]string{"projectId": "p1"})
	if err := InitWorkspaceEnv(tmp, EnvInit{
		Kind:             EnvBackendInfisical,
		ConfigJSON:       cfgRaw,
		EnvironmentNames: []string{"dev", "prod"},
		DefaultEnv:       "dev",
	}); err != nil {
		t.Fatalf("InitWorkspaceEnv: %v", err)
	}

	got, err := ReadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if EnvBackend(got) != EnvBackendInfisical {
		t.Errorf("env backend not written: %+v", got.Env)
	}
	if got.Environments == nil || got.Environments.Default != "dev" {
		t.Errorf("environments not written: %+v", got.Environments)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("projects wiped: %d", len(got.Projects))
	}
}

func TestManifestParseErrorIncludesPathLocationAndCause(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		root := t.TempDir()
		path := ManifestPath(root)
		// The invalid character is on line 2, column 11, after a Unicode name.
		if err := os.WriteFile(path, []byte("{\n\"中文\":true,@}"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadManifest(root)
		var coded *output.Error
		var syntax *json.SyntaxError
		if !errors.As(err, &coded) || coded.Code != "MANIFEST_INVALID" || !errors.As(err, &syntax) {
			t.Fatalf("lost structured error or cause: %v", err)
		}
		if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "@") || coded.Context["line"] != 2 || coded.Context["column"] != 11 {
			t.Fatalf("missing error detail: %v; context=%v", err, coded.Context)
		}
	}
}

func TestManifestReadErrorIncludesPathAndCause(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(ManifestPath(root), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := ReadManifest(root)
	var pathError *os.PathError
	if !errors.As(err, &pathError) || !strings.Contains(err.Error(), ManifestPath(root)) {
		t.Fatalf("missing read cause/path: %v", err)
	}
}
