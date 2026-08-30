package build

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
)

func TestLocalDryRunPlansNodeBuild(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(projectDir, "package.json"),
		[]byte(`{"scripts":{"build":"vite build"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	lines, err := (Local{}).Build(context.Background(), deployport.BuildInput{
		Apply: deployport.ApplyInput{
			ProjectRoot: projectDir,
			Project:     workspace.Project{Name: "web", TargetDir: projectDir},
			DryRun:      true,
		},
		Backend:        workspace.DeployBackendCloudflare,
		Toolchain:      "node",
		PackageManager: "npm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lines, []string{"npm run build"}) {
		t.Fatalf("Build() = %#v", lines)
	}
}

func TestLocalSkipsBackendsWithoutAutoBuild(t *testing.T) {
	lines, err := (Local{}).Build(context.Background(), deployport.BuildInput{
		Apply:     deployport.ApplyInput{DryRun: true},
		Backend:   workspace.DeployBackendVercel,
		Toolchain: "node",
	})
	if err != nil || lines != nil {
		t.Fatalf("Build() = %#v, %v", lines, err)
	}
}
