package ci

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
)

type providerStub struct{}

func (providerStub) ID() string { return pkgci.DefaultProviderID }

func (providerStub) WorkflowFilename(input pkgci.Input) string {
	return ".github/workflows/ci-" + input.ProjectName + ".yml"
}

func (providerStub) Render(input pkgci.Input) string { return "project: " + input.ProjectName + "\n" }

func TestServiceOwnsCIWorkspaceLifecycle(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
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
	service, err := NewService(pkgci.MustRegistry(providerStub{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := execution.WithScope(
		context.Background(), execution.NewScope(context.Background(), root),
	)

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Configured || len(status.Projects) != 1 || status.Projects[0].Enabled {
		t.Fatalf("initial status = %#v", status)
	}

	enabled, err := service.Enable(ctx, EnableRequest{Selector: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if len(enabled.Projects) != 1 || enabled.Projects[0].Status != "created" {
		t.Fatalf("enable = %#v", enabled)
	}
	workflowPath := filepath.Join(root, ".github", "workflows", "ci-web.yml")
	if raw, err := os.ReadFile(workflowPath); err != nil || string(raw) != "project: web\n" {
		t.Fatalf("workflow = %q, %v", raw, err)
	}

	synced, err := service.Sync(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Projects) != 1 || synced.Projects[0].Status != "updated" {
		t.Fatalf("sync = %#v", synced)
	}

	plan, err := service.PlanDisable(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if plan.EnabledCount != 1 {
		t.Fatalf("disable plan = %#v", plan)
	}
	if _, err := service.Disable(plan, false); errorCode(err) != "CI_DISABLE_CONFIRMATION_REQUIRED" {
		t.Fatalf("unconfirmed disable error = %v", err)
	}
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("unconfirmed disable removed workflow: %v", err)
	}
	disabled, err := service.Disable(plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(disabled.Projects) != 1 || disabled.Projects[0].Status != "removed" {
		t.Fatalf("disable = %#v", disabled)
	}
	if _, err := os.Stat(workflowPath); !os.IsNotExist(err) {
		t.Fatalf("workflow still exists: %v", err)
	}

	stalePlan, err := service.PlanDisable(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if stalePlan.EnabledCount != 0 {
		t.Fatalf("stale plan = %#v", stalePlan)
	}
	if err := os.WriteFile(workflowPath, []byte("appeared later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Disable(stalePlan, false); errorCode(err) != "CI_DISABLE_CONFIRMATION_REQUIRED" {
		t.Fatalf("stale-plan disable error = %v", err)
	}
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("stale-plan disable removed workflow: %v", err)
	}
}

func TestEnableRejectsUnknownProvider(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web"}},
	}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pkgci.MustRegistry(providerStub{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := execution.WithScope(
		context.Background(), execution.NewScope(context.Background(), root),
	)
	_, err = service.Enable(ctx, EnableRequest{Provider: "ci/unknown"})
	if errorCode(err) != "CI_PROVIDER_UNKNOWN" {
		t.Fatalf("error = %v", err)
	}
}

func errorCode(err error) string {
	if coded, ok := err.(interface{ ErrorCode() string }); ok {
		return coded.ErrorCode()
	}
	return ""
}
