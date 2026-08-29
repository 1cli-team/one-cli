package configurecmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func testProfileForm(t *testing.T, pair string) (*configureapp.ProfileService, catalog.BackendSpec, []*profileFieldInput) {
	t.Helper()
	_, profiles, _ := testServices(t)
	spec, err := profiles.ParsePair(pair)
	if err != nil {
		t.Fatal(err)
	}
	return profiles, spec, bindProfileFields(&cobra.Command{}, spec)
}

func setProfileFormValue(t *testing.T, inputs []*profileFieldInput, path, value string) {
	t.Helper()
	input := fieldInputByPath(inputs, path)
	if input == nil {
		t.Fatalf("profile form has no field %q", path)
	}
	input.stringValue = value
}

func TestBuildCatalogProfileUsesCatalogDefaults(t *testing.T) {
	profiles, spec, inputs := testProfileForm(t, "deploy/minio")
	setProfileFormValue(t, inputs, "credentials/accessKeyId", "key")
	setProfileFormValue(t, inputs, "credentials/accessKeySecret", "secret")

	value, err := buildCatalogProfile(profiles, spec, inputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.S3 == nil || value.S3.Region != "us-east-1" || !value.S3.ForcePathStyle {
		t.Fatalf("Catalog defaults were not decoded: %#v", value.S3)
	}
}

func TestBuildCatalogProfileDerivesFixedRegistryNamespace(t *testing.T) {
	profiles, spec, inputs := testProfileForm(t, "container/ghcr")
	setProfileFormValue(t, inputs, "credentials/username", "octocat")
	setProfileFormValue(t, inputs, "credentials/password", "token")

	value, err := buildCatalogProfile(profiles, spec, inputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.Container == nil || value.Container.Namespace != "octocat" {
		t.Fatalf("fixed-registry namespace = %#v, want octocat", value.Container)
	}
}

func TestBuildCatalogProfileAllowsOptionalEdgeOneToken(t *testing.T) {
	profiles, spec, inputs := testProfileForm(t, "deploy/edgeone")
	value, err := buildCatalogProfile(profiles, spec, inputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.EdgeOne == nil || value.EdgeOne.Credentials != nil {
		t.Fatalf("EdgeOne optional credentials = %#v", value.EdgeOne)
	}
}

func TestBuildCatalogProfileRejectsMissingCatalogRequiredField(t *testing.T) {
	profiles, spec, inputs := testProfileForm(t, "env/infisical")
	_, err := buildCatalogProfile(profiles, spec, inputs, false)
	if err == nil {
		t.Fatal("expected missing required field error")
	}
	coded, ok := err.(interface{ ErrorCode() string })
	if !ok || coded.ErrorCode() != string(cliErrors.PROFILE_BACKEND_INVALID) {
		t.Fatalf("error = %T %v", err, err)
	}
}

func TestBuildCatalogProfileResolvesKubeconfigContext(t *testing.T) {
	profiles, spec, inputs := testProfileForm(t, "deploy/kustomize")
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("current-context: prod\ncontexts:\n  - name: prod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	setProfileFormValue(t, inputs, "kubeconfigPath", path)

	value, err := buildCatalogProfile(profiles, spec, inputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kustomize == nil || value.Kustomize.KubeconfigContext != "prod" {
		t.Fatalf("kustomize profile = %#v", value.Kustomize)
	}
}
