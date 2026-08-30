package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestLocalWorkflowOwnsDispatchAndManifestBookkeeping(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Environments: &workspace.Environments{
			Names: []string{"dev"}, Default: "dev",
		},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".env.dev"), []byte("TOKEN=before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	scope := execution.NewScope(context.Background(), root)
	getResult, err := service.Get(context.Background(), GetInput{
		Scope: scope, Project: "web", Environment: "dev", Key: "TOKEN",
	})
	if err != nil || getResult.Value != "before" {
		t.Fatalf("get result = %#v, err = %v", getResult, err)
	}
	plan, err := service.PlanSet(PlanSetInput{
		Scope: scope, Environment: "qa", Project: "web",
	})
	if err != nil {
		t.Fatal(err)
	}
	setResult, err := service.Set(context.Background(), SetInput{
		Plan: plan, Key: "TOKEN", Value: "after", Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !setResult.CreatedEnvironment || setResult.Action != "created" {
		t.Fatalf("set result = %#v", setResult)
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(manifest.Environments.Names, "qa") {
		t.Fatalf("environment names = %v", manifest.Environments.Names)
	}
	if keys := manifest.Projects[0].Domains.Env.Keys; len(keys) != 1 || keys[0] != "TOKEN" {
		t.Fatalf("project keys = %v", keys)
	}
}

func TestCollectDotenvTuplesUsesOverlayPrecedence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev"}},
		Projects:     []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web"}},
	}
	for name, content := range map[string]string{
		".env":           "TOKEN=base\n",
		".env.dev":       "TOKEN=environment\n",
		".env.local":     "TOKEN=local\n",
		".env.dev.local": "TOKEN=final\n",
	} {
		if err := os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tuples, err := collectDotenvTuples(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(tuples) != 1 || tuples[0].value != "final" || tuples[0].path != "/apps/web" {
		t.Fatalf("tuples = %#v", tuples)
	}
}
