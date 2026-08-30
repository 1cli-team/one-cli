package workspace

import (
	"context"
	"errors"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func seedWorkspaceProfileSettings(t *testing.T, backend string) string {
	t.Helper()
	root := t.TempDir()
	manifest := &workspacecore.Manifest{
		Version:   workspacecore.ManifestVersion,
		Workspace: &workspacecore.ManifestWorkspace{ID: "ws-profile", Name: "Profiles"},
		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
		}},
	}
	if backend != "" {
		manifest.Domains = &workspacecore.WorkspaceDomains{
			Env: &workspacecore.BackendRef{Kind: backend},
		}
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWorkspaceEnvironmentProfileBindsPerEnvironmentWithoutRepositoryWrites(t *testing.T) {
	profiles := projectProfileStub()
	service, err := NewService(catalog.Builtin(), profiles)
	if err != nil {
		t.Fatal(err)
	}
	root := seedWorkspaceProfileSettings(t, catalog.EnvInfisical)
	before := snapshotWorkspaceTree(t, root)

	settings, err := service.UpdateWorkspaceEnvironmentProfile(
		context.Background(), root, "preview", "work",
	)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Schema != WorkspaceProfileSettingsSchema || settings.Environment != "preview" ||
		settings.SelectedProfile != "work" || settings.Profile == nil ||
		settings.Profile.Source != "workspace-environment" {
		t.Fatalf("settings = %#v", settings)
	}
	if profiles.lastMode != "environment-bind" {
		t.Fatalf("binding mode = %q", profiles.lastMode)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)

	settings, err = service.UpdateWorkspaceEnvironmentProfile(
		context.Background(), root, "preview", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if profiles.lastMode != "environment-unbind" || settings.SelectedProfile != "" {
		t.Fatalf("unbind settings = %q %#v", profiles.lastMode, settings)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestWorkspaceEnvironmentProfileSurfacesStaleDirectBindingUntilUnbound(t *testing.T) {
	profiles := projectProfileStub()
	service, err := NewService(catalog.Builtin(), profiles)
	if err != nil {
		t.Fatal(err)
	}
	root := seedWorkspaceProfileSettings(t, catalog.EnvInfisical)
	before := snapshotWorkspaceTree(t, root)
	profiles.bindings[projectBindingKey(
		root, "", "preview", profile.DomainEnv, catalog.EnvInfisical,
	)] = "deleted-profile"

	settings, err := service.WorkspaceEnvironmentProfile(
		context.Background(), root, "preview",
	)
	if err != nil {
		t.Fatal(err)
	}
	if settings.SelectedProfile != "deleted-profile" || settings.Profile != nil {
		t.Fatalf("stale Workspace binding projection = %#v", settings)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)

	settings, err = service.UpdateWorkspaceEnvironmentProfile(
		context.Background(), root, "preview", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if settings.SelectedProfile != "" || settings.Profile == nil ||
		settings.Profile.Name != "work" || settings.Profile.Source != "default" {
		t.Fatalf("automatic Workspace fallback = %#v", settings)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestWorkspaceEnvironmentProfileEmptyEnvironmentUsesLegacyBinding(t *testing.T) {
	profiles := projectProfileStub()
	service, err := NewService(catalog.Builtin(), profiles)
	if err != nil {
		t.Fatal(err)
	}
	root := seedWorkspaceProfileSettings(t, catalog.EnvInfisical)
	before := snapshotWorkspaceTree(t, root)

	settings, err := service.UpdateWorkspaceEnvironmentProfile(
		context.Background(), root, "", "work",
	)
	if err != nil {
		t.Fatal(err)
	}
	if profiles.lastMode != "legacy-bind" || settings.Environment != "" ||
		settings.SelectedProfile != "work" || settings.Profile == nil ||
		settings.Profile.Source != "workspace" {
		t.Fatalf("legacy settings = %q %#v", profiles.lastMode, settings)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestWorkspaceEnvironmentProfileReadHandlesMissingAndNonConfigurableBackend(t *testing.T) {
	service, err := NewService(catalog.Builtin(), projectProfileStub())
	if err != nil {
		t.Fatal(err)
	}
	missingRoot := seedWorkspaceProfileSettings(t, "")
	settings, err := service.WorkspaceEnvironmentProfile(
		context.Background(), missingRoot, "preview",
	)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Environment != "preview" || settings.Backend != "" ||
		settings.Configurable || settings.Profile != nil {
		t.Fatalf("missing backend settings = %#v", settings)
	}

	dotenvRoot := seedWorkspaceProfileSettings(t, catalog.EnvDotenv)
	settings, err = service.WorkspaceEnvironmentProfile(
		context.Background(), dotenvRoot, "staging_us2",
	)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Environment != "staging_us2" || settings.Backend != catalog.EnvDotenv ||
		settings.Configurable || settings.Profile != nil {
		t.Fatalf("dotenv settings = %#v", settings)
	}
	before := snapshotWorkspaceTree(t, dotenvRoot)
	if _, err := service.UpdateWorkspaceEnvironmentProfile(
		context.Background(), dotenvRoot, "preview", "",
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("dotenv update error = %v; want ErrInvalidInput", err)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, dotenvRoot), before)
}

func TestWorkspaceEnvironmentProfileRejectsUnknownBackendProfileAndUnsafeEnvironment(t *testing.T) {
	for _, test := range []struct {
		name        string
		backend     string
		environment string
		requested   string
	}{
		{name: "unknown backend", backend: "vault", environment: "preview", requested: "work"},
		{name: "unknown profile", backend: catalog.EnvInfisical, environment: "preview", requested: "ghost"},
		{name: "unsafe environment", backend: catalog.EnvInfisical, environment: "../preview", requested: "work"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := seedWorkspaceProfileSettings(t, test.backend)
			before := snapshotWorkspaceTree(t, root)
			service, err := NewService(catalog.Builtin(), projectProfileStub())
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.UpdateWorkspaceEnvironmentProfile(
				context.Background(), root, test.environment, test.requested,
			)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v; want ErrInvalidInput", err)
			}
			assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
		})
	}
}
