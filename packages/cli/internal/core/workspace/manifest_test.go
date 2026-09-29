package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func TestManifestV2RejectsUnknownAndRemovedFields(t *testing.T) {
	for _, raw := range []string{
		"version = 2\n[projects.web]\npath='apps/web'\ntoolchain='node'\ntemplate='react-spa'\n",
		`{"version":2,"projects":[]}`, "version = 1\n", "version = 3\n",
		"version = 2\n[env]\nenvironments = ['dev']\n",
		"version = 2\n[env.infisical]\nprojectId='p'\nenvironments=['dev']\ndefault='dev'\n",
		"version = 2\n[env.infisical]\nprojectId='p'\nenvironments=['dev']\nrootPath='/'\n",
		"version = 2\n[env.infisical]\nprojectId='p'\nenvironments=['dev']\nprojectName='Demo'\n",
		"version = 2\n[env.infisical]\nprojectId='p'\nenvironments=['dev']\nkeys=['TOKEN']\n",
		"version = 2\n[env.infisical]\nprojectId='p'\nenvironments=['prod']\n",
		"version = 2\n[env.infisical]\nprojectId='p'\nenvironments=['dev','dev']\n",
		"version = 2\n[projects.web]\npath='../web'\ntoolchain='node'\n",
		"version = 2\n[projects.web]\npath='apps/web'\ntoolchain='unknown'\n",
		"version = 2\n[projects.web]\npath='apps/web'\n",
		"version = 2\n[projects.web]\npath='apps/web'\ntoolchain='node'\nservice.url='http://localhost'\n",
		"version = 2\n[projects.web]\npath='apps/web'\ntoolchain='node'\nenv.enabled=false\n",
		"version = 2\n[projects.web]\npath='apps/web'\ntoolchain='node'\npackageManager='pnpm'\n",
	} {
		if _, err := ParseManifest([]byte(raw)); err == nil {
			t.Errorf("accepted invalid document: %s", raw)
		}
	}
}

func TestManifestV2RoundTripAndNoGeneratedComments(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{Version: ManifestVersion, Workspace: &ManifestWorkspace{ID: "demo", Name: "示例"}, Env: &EnvironmentConfig{ProjectID: "remote", Environments: []string{"prod", "dev", "staging"}}, Projects: []ManifestProject{
		{Name: "api", RelativeDir: "services/api", Toolchain: "go"},
		{Name: "web", RelativeDir: "apps/web", Toolchain: "node"},
	}}
	if err := WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(ManifestPath(root))
	for _, removed := range []string{"#", "relativeDir", "template", "templateId", "buildVersion", "packageManager", "default =", "[env]", "[projects]"} {
		if bytes.Contains(raw, []byte(removed)) {
			t.Errorf("generated unwanted %q: %s", removed, raw)
		}
	}
	if !bytes.Contains(raw, []byte("[env.infisical]")) || !bytes.Contains(raw, []byte("[projects.web]")) {
		t.Fatalf("unexpected TOML: %s", raw)
	}
	got, rev, err := ReadManifestSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace.Name != "示例" || got.Env.ProjectID != "remote" || got.Projects[0].Name != "web" || rev == "" {
		t.Fatalf("bad roundtrip: %+v", got)
	}
	if err := WriteManifest(root, got); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(ManifestPath(root))
	if !bytes.Equal(raw, after) {
		t.Fatalf("unchanged write reformatted file: %s", after)
	}
}

func TestManifestParseErrorIncludesPathLocationAndCause(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		root := t.TempDir()
		path := ManifestPath(root)
		if err := os.WriteFile(path, []byte("version = 2\n[workspace\n"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadManifest(root)
		var coded *output.Error
		var syntax *toml.DecodeError
		if !errors.As(err, &coded) || coded.Code != "MANIFEST_INVALID" || !errors.As(err, &syntax) {
			t.Fatalf("lost cause: %v", err)
		}
		if !strings.Contains(err.Error(), path) || coded.Context["line"] != 2 {
			t.Fatalf("missing location: %v %+v", err, coded.Context)
		}
	}
}
func TestManifestReadErrorIncludesPathAndCause(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(ManifestPath(root), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := ReadManifest(root)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || !strings.Contains(err.Error(), ManifestPath(root)) {
		t.Fatal(err)
	}
}
func TestManifestOnlyDiscoversTOML(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "one.manifest.json"), []byte(`{"version":1}`), 0644)
	if HasManifest(root) {
		t.Fatal("legacy JSON is still a workspace marker")
	}
}
