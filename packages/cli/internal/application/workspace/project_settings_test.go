package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

type projectProfileAccessStub struct {
	available map[string]struct{}
	bindings  map[string]string
	defaults  map[string]string
	bindErr   error
	lastMode  string
}

func projectBindingKey(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) string {
	return strings.Join([]string{root, projectName, environment, profile.SectionKey(domain, backend)}, "|")
}

func (s *projectProfileAccessStub) Resolve(input profile.ResolveInput) (*profile.Resolved, error) {
	section := profile.SectionKey(input.Domain, input.Backend)
	name := strings.TrimSpace(input.FlagOverride)
	source := "flag"
	if name == "" && input.Environment != "" {
		name = s.bindings[projectBindingKey(
			input.WorkspaceRoot, input.ProjectName, input.Environment, input.Domain, input.Backend,
		)]
		if name != "" {
			if input.ProjectName != "" {
				source = "workspace-project-environment"
			} else {
				source = "workspace-environment"
			}
		}
	}
	if name == "" {
		name = s.bindings[projectBindingKey(
			input.WorkspaceID, input.ProjectName, "", input.Domain, input.Backend,
		)]
		if name != "" {
			if input.ProjectName != "" {
				source = "workspace-project"
			} else {
				source = "workspace"
			}
		}
	}
	if name == "" && !input.SkipDefault {
		name = s.defaults[section]
		source = "default"
	}
	if name == "" {
		return nil, errors.New("profile not configured")
	}
	if _, ok := s.available[section+"/"+name]; !ok {
		return nil, errors.New("profile not found")
	}
	return &profile.Resolved{
		Name: name, Source: source,
		Profile: profile.Profile{Vercel: &profile.VercelProfile{
			Credentials: &profile.VercelCredentials{APIToken: "never-return-this-token"},
		}},
	}, nil
}

func (s *projectProfileAccessStub) BindWorkspaceProfile(
	workspaceID, _, _ string,
	projectName string,
	domain profile.Domain,
	backend, name string,
) error {
	if s.bindErr != nil {
		return s.bindErr
	}
	s.lastMode = "legacy-bind"
	s.bindings[projectBindingKey(workspaceID, projectName, "", domain, backend)] = name
	return nil
}

func (s *projectProfileAccessStub) UnbindWorkspaceProfile(
	workspaceID, projectName string,
	domain profile.Domain,
	backend string,
) error {
	if s.bindErr != nil {
		return s.bindErr
	}
	s.lastMode = "legacy-unbind"
	delete(s.bindings, projectBindingKey(workspaceID, projectName, "", domain, backend))
	return nil
}

func (s *projectProfileAccessStub) BindEnvironmentProfile(
	_, _, root, projectName, environment string,
	domain profile.Domain,
	backend, name string,
) error {
	if s.bindErr != nil {
		return s.bindErr
	}
	s.lastMode = "environment-bind"
	s.bindings[projectBindingKey(root, projectName, environment, domain, backend)] = name
	return nil
}

func (s *projectProfileAccessStub) UnbindEnvironmentProfile(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) error {
	if s.bindErr != nil {
		return s.bindErr
	}
	s.lastMode = "environment-unbind"
	delete(s.bindings, projectBindingKey(root, projectName, environment, domain, backend))
	return nil
}

func (s *projectProfileAccessStub) EnvironmentProfileBinding(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) (string, error) {
	return s.bindings[projectBindingKey(root, projectName, environment, domain, backend)], nil
}

func seedProjectSettingsWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	deployConfig, err := json.Marshal(map[string]any{
		"projectId": "prj_demo",
		"env":       "prod",
		"apiToken":  "must-not-leak",
	})
	if err != nil {
		t.Fatal(err)
	}
	inherits := false
	manifest := &workspacecore.Manifest{
		Version:      workspacecore.ManifestVersion,
		Workspace:    &workspacecore.ManifestWorkspace{ID: "ws-demo", Name: "Demo"},
		Environments: &workspacecore.Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},
		Domains: &workspacecore.WorkspaceDomains{
			Env: &workspacecore.BackendRef{Kind: catalog.EnvInfisical},
		},
		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
			BuildVersion: "1.2.3", PackageManager: "pnpm",
			Domains: &workspacecore.ProjectDomains{
				Env: &workspacecore.ProjectEnvOverride{
					Path: "/apps/web", Inherits: &inherits, Keys: []string{"Z_KEY", "A_KEY"},
				},
				Container: &workspacecore.ProjectContainerOverride{
					Kind: catalog.ContainerGHCR, Image: "ghcr.io/acme/web:1.2.3", Namespace: "acme",
				},
				Deploy: &workspacecore.ProjectDeployBackend{Kind: catalog.DeployVercel, Config: deployConfig},
				Dev:    &workspacecore.ProjectDevOverride{Command: "pnpm dev"},
			},
		}},
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("repository data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func projectProfileStub() *projectProfileAccessStub {
	available := map[string]struct{}{}
	for _, pair := range [][2]string{
		{profile.SectionKey(profile.DomainEnv, catalog.EnvInfisical), "work"},
		{profile.SectionKey(profile.DomainContainer, catalog.ContainerGHCR), "work"},
		{profile.SectionKey(profile.DomainDeploy, catalog.DeployVercel), "work"},
	} {
		available[pair[0]+"/"+pair[1]] = struct{}{}
	}
	return &projectProfileAccessStub{
		available: available,
		bindings:  map[string]string{},
		defaults: map[string]string{
			profile.SectionKey(profile.DomainEnv, catalog.EnvInfisical): "work",
		},
	}
}

func snapshotWorkspaceTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertWorkspaceTreeEqual(t *testing.T, got, want map[string][]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("workspace file count changed: got %d want %d", len(got), len(want))
	}
	for path, expected := range want {
		if !bytes.Equal(got[path], expected) {
			t.Fatalf("workspace file %q changed", path)
		}
	}
}

func TestProjectSettingsReturnsEnvironmentAwareSafeProjection(t *testing.T) {
	root := seedProjectSettingsWorkspace(t)
	profiles := projectProfileStub()
	profiles.bindings[projectBindingKey(
		root, "web", "staging", profile.DomainEnv, catalog.EnvInfisical,
	)] = "work"
	service, err := NewService(catalog.Builtin(), profiles)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := service.ProjectSettings(context.Background(), root, "web", "preview")
	if err != nil {
		t.Fatal(err)
	}
	project := settings.Project
	if settings.Schema != ProjectSettingsSchema || settings.Environment != "preview" ||
		project.Kind != workspacecore.ProjectKindApp {
		t.Fatalf("unexpected envelope: %#v", settings)
	}
	if project.Environment.SelectedProfile != "work" || project.Environment.Profile == nil ||
		project.Environment.Profile.Source != "workspace-project-environment" {
		t.Fatalf("environment profile = %#v", project.Environment)
	}
	if project.Container.SelectedProfile != "" || project.Deploy.SelectedProfile != "" {
		t.Fatalf("inherited profiles reported as direct: %#v %#v", project.Container, project.Deploy)
	}
	if got := strings.Join(project.Environment.Keys, ","); got != "A_KEY,Z_KEY" {
		t.Fatalf("environment keys = %q", got)
	}
	if project.Deploy.Config["projectId"] != "prj_demo" {
		t.Fatalf("deploy config = %#v", project.Deploy.Config)
	}
	if _, leaked := project.Deploy.Config["apiToken"]; leaked {
		t.Fatal("unknown token-like manifest field was reflected")
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "must-not-leak") || strings.Contains(string(raw), "never-return") {
		t.Fatalf("settings leaked a credential: %s", raw)
	}
}

func TestProjectSettingsSurfacesStaleDirectBindingAndAllowsAutomaticFallback(t *testing.T) {
	root := seedProjectSettingsWorkspace(t)
	before := snapshotWorkspaceTree(t, root)
	profiles := projectProfileStub()
	profiles.bindings[projectBindingKey(
		root, "web", "staging", profile.DomainEnv, catalog.EnvInfisical,
	)] = "deleted-profile"
	service, err := NewService(catalog.Builtin(), profiles)
	if err != nil {
		t.Fatal(err)
	}

	settings, err := service.ProjectSettings(context.Background(), root, "web", "preview")
	if err != nil {
		t.Fatal(err)
	}
	if settings.Project.Environment.SelectedProfile != "deleted-profile" {
		t.Fatalf("stale direct binding was hidden: %#v", settings.Project.Environment)
	}
	if settings.Project.Environment.Profile != nil {
		t.Fatalf("stale direct binding was reported as effective: %#v", settings.Project.Environment)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)

	settings, err = service.UpdateProjectProfileBinding(
		context.Background(), root, "web", "env", "preview", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Project.Environment.SelectedProfile != "" ||
		settings.Project.Environment.Profile == nil ||
		settings.Project.Environment.Profile.Name != "work" ||
		settings.Project.Environment.Profile.Source != "default" {
		t.Fatalf("automatic fallback = %#v", settings.Project.Environment)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestProjectProfileBindingsOnlyChangeMachineLocalState(t *testing.T) {
	root := seedProjectSettingsWorkspace(t)
	before := snapshotWorkspaceTree(t, root)
	profiles := projectProfileStub()
	service, err := NewService(catalog.Builtin(), profiles)
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"env", "container", "deploy"} {
		settings, err := service.UpdateProjectProfileBinding(
			context.Background(), root, "web", domain, "preview", "work",
		)
		if err != nil {
			t.Fatalf("bind %s: %v", domain, err)
		}
		if profiles.lastMode != "environment-bind" || settings.Environment != "preview" {
			t.Fatalf("%s binding mode/settings = %q %#v", domain, profiles.lastMode, settings)
		}
		assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
	}

	settings, err := service.UpdateProjectProfileBinding(
		context.Background(), root, "web", "container", "preview", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if profiles.lastMode != "environment-unbind" || settings.Project.Container.SelectedProfile != "" {
		t.Fatalf("unbind result = %q %#v", profiles.lastMode, settings.Project.Container)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)

	settings, err = service.UpdateProjectProfileBinding(
		context.Background(), root, "web", "container", "", "work",
	)
	if err != nil {
		t.Fatal(err)
	}
	if profiles.lastMode != "legacy-bind" || settings.Project.Container.SelectedProfile != "work" ||
		settings.Project.Container.Profile == nil ||
		settings.Project.Container.Profile.Source != "workspace-project" {
		t.Fatalf("legacy binding result = %q %#v", profiles.lastMode, settings.Project.Container)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestLegacyUnbindDoesNotUpgradeManifestMissingWorkspaceID(t *testing.T) {
	root := seedProjectSettingsWorkspace(t)
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Workspace.ID = ""
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	before := snapshotWorkspaceTree(t, root)
	service, err := NewService(catalog.Builtin(), projectProfileStub())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateProjectProfileBinding(
		context.Background(), root, "web", "env", "", "",
	); err != nil {
		t.Fatalf("legacy unbind: %v", err)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestProjectProfileBindingRejectsInvalidInputWithoutChangingRepository(t *testing.T) {
	for _, test := range []struct {
		name        string
		mutate      func(*workspacecore.Manifest)
		project     string
		domain      string
		environment string
		profileName string
	}{
		{name: "unknown domain", project: "web", domain: "ci", environment: "preview", profileName: "work"},
		{name: "unknown project", project: "ghost", domain: "env", environment: "preview", profileName: "work"},
		{name: "unsafe environment", project: "web", domain: "env", environment: "../prod", profileName: "work"},
		{name: "padded environment", project: "web", domain: "env", environment: " preview ", profileName: "work"},
		{name: "unknown profile", project: "web", domain: "env", environment: "preview", profileName: "ghost"},
		{
			name: "unknown manifest backend", project: "web", domain: "env", environment: "preview", profileName: "work",
			mutate: func(manifest *workspacecore.Manifest) {
				manifest.Domains.Env.Kind = "vault"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := seedProjectSettingsWorkspace(t)
			if test.mutate != nil {
				manifest, err := workspacecore.ReadManifest(root)
				if err != nil {
					t.Fatal(err)
				}
				test.mutate(manifest)
				if err := workspacecore.WriteManifest(root, manifest); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotWorkspaceTree(t, root)
			service, err := NewService(catalog.Builtin(), projectProfileStub())
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.UpdateProjectProfileBinding(
				context.Background(), root, test.project, test.domain, test.environment, test.profileName,
			)
			if err == nil {
				t.Fatal("expected error")
			}
			if test.project != "ghost" && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v; want ErrInvalidInput", err)
			}
			assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
		})
	}
}
