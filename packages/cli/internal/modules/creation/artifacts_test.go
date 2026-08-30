package creation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

func TestSyncProjectOwnsDevAndEnvironmentArtifacts(t *testing.T) {
	internaltoolchain.RegisterBundled()
	root := t.TempDir()
	targetDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(targetDir, "package.json"),
		[]byte(`{"scripts":{"dev":"vite"}}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "pnpm",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	err := syncProject(syncProjectOptions{
		ProjectRoot: root, TargetDir: targetDir, ProjectName: "web",
		Toolchain: toolchain.Node, PackageManager: toolchain.PMpnpm,
		Selected: map[string]string{"env": "env/infisical"},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	dev := manifest.Projects[0].Domains.Dev
	if dev == nil || dev.Command != "pnpm run dev" {
		t.Fatalf("dev override = %#v", dev)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{".env", ".env.*", "!.env.example"} {
		if !strings.Contains(string(raw), line) {
			t.Fatalf(".gitignore = %q, missing %q", raw, line)
		}
	}
}
