package deployment

import (
	"encoding/json"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func deployConfig(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestApplyEnvOverridePreservesBackendConfig(t *testing.T) {
	manifest := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}},
		Projects: []workspace.ManifestProject{
			{Name: "web", Domains: &workspace.ProjectDomains{Deploy: &workspace.ProjectDeployBackend{
				Kind:   workspace.DeployBackendVercel,
				Config: deployConfig(t, map[string]string{"env": "prod", "projectId": "project-1"}),
			}}},
			{Name: "api", Domains: &workspace.ProjectDomains{Deploy: &workspace.ProjectDeployBackend{
				Kind: workspace.DeployBackendCloudflare,
			}}},
		},
	}
	if err := applyEnvOverride(manifest, "staging"); err != nil {
		t.Fatal(err)
	}
	for _, project := range manifest.Projects {
		environment, err := readDeployEnvironment(project.Domains.Deploy)
		if err != nil || environment != "staging" {
			t.Fatalf("%s environment = %q, %v", project.Name, environment, err)
		}
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(manifest.Projects[0].Domains.Deploy.Config, &config); err != nil {
		t.Fatal(err)
	}
	if string(config["projectId"]) != `"project-1"` {
		t.Fatalf("backend config was not preserved: %s", manifest.Projects[0].Domains.Deploy.Config)
	}
}

func TestEnvironmentPolicyRejectsUnknownNames(t *testing.T) {
	manifest := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "prod"}},
		Projects: []workspace.ManifestProject{{
			Name: "web",
			Domains: &workspace.ProjectDomains{Deploy: &workspace.ProjectDeployBackend{
				Kind:   workspace.DeployBackendVercel,
				Config: deployConfig(t, map[string]string{"env": "typo"}),
			}},
		}},
	}
	if err := validateProjectEnvironments(manifest); errorCode(err) != "ENV_UNKNOWN_ENVIRONMENT" {
		t.Fatalf("validateProjectEnvironments() = %v", err)
	}
	if err := applyEnvOverride(manifest, "qa"); errorCode(err) != "ENV_UNKNOWN_ENVIRONMENT" {
		t.Fatalf("applyEnvOverride() = %v", err)
	}
}

func TestEnvironmentPolicyAllowsUndeclaredNames(t *testing.T) {
	manifest := &workspace.Manifest{Projects: []workspace.ManifestProject{{
		Name: "web",
		Domains: &workspace.ProjectDomains{Deploy: &workspace.ProjectDeployBackend{
			Kind: workspace.DeployBackendVercel,
		}},
	}}}
	if err := applyEnvOverride(manifest, "preview"); err != nil {
		t.Fatal(err)
	}
	if environment, err := readDeployEnvironment(manifest.Projects[0].Domains.Deploy); err != nil || environment != "preview" {
		t.Fatalf("environment = %q, %v", environment, err)
	}
}

func errorCode(err error) string {
	if coded, ok := err.(interface{ ErrorCode() string }); ok {
		return coded.ErrorCode()
	}
	return ""
}
