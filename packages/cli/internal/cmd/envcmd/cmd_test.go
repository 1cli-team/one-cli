package envcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// TestResolveInfisicalFolderPath locks the contract that `-p` (or cwd
// fallback) is correctly translated into the Infisical folder path
// before reaching the secrets backend. Direct in-process tests are the
// right level for this — exercising via the binary requires an
// authenticated Infisical, which would be flaky.
func TestResolveInfisicalFolderPath(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(tmp, "id", "demo"); err != nil {
		t.Fatal(err)
	}
	for _, in := range []workspace.ManifestProjectInput{
		{Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go"},
		{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node"},
	} {
		if err := workspace.UpsertManifestProject(tmp, in); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &infisical.WorkspaceConfig{ProjectID: "x", RootPath: "/"}

	cases := []struct {
		name     string
		selector string
		want     string
	}{
		{"empty + cwd at workspace root → rootPath", "", "/"},
		{"selector by name → /<relativeDir>", "api", "/services/api"},
		{"selector by name (web) → /<relativeDir>", "web", "/apps/web"},
		{"selector by relativeDir → /<relativeDir>", "services/api", "/services/api"},
		{"selector with ./ prefix → tolerated", "./apps/web", "/apps/web"},
		{"selector as absolute folder path → honoured verbatim", "/shared", "/shared"},
	}

	// Run from a deterministic cwd outside the workspace so the empty-
	// selector branch hits the workspace-root path and not cwd-walk.
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(filepath.Dir(tmp)); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveInfisicalFolderPath(tmp, cfg, tc.selector)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("path = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("unknown non-path selector → SUBPROJECT_NOT_FOUND", func(t *testing.T) {
		_, err := resolveInfisicalFolderPath(tmp, cfg, "nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown selector")
		}
		if codeErr, ok := err.(interface{ ErrorCode() string }); !ok || codeErr.ErrorCode() != "SUBPROJECT_NOT_FOUND" {
			t.Errorf("expected SUBPROJECT_NOT_FOUND, got: %v", err)
		}
	})

	t.Run("nil cfg → falls back to manifest rootPath", func(t *testing.T) {
		got, err := resolveInfisicalFolderPath(tmp, nil, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/services/api" {
			t.Errorf("path = %q, want /services/api", got)
		}
	})

	t.Run("cwd inside subproject + empty selector → that subproject's path", func(t *testing.T) {
		subDir := filepath.Join(tmp, "services", "api")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(subDir); err != nil {
			t.Fatal(err)
		}
		got, err := resolveInfisicalFolderPath(tmp, cfg, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/services/api" {
			t.Errorf("path = %q, want /services/api", got)
		}
	})
}
