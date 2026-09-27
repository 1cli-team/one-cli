package configurecmd

import (
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
	profiles, spec, inputs := testProfileForm(t, "env/infisical")
	setProfileFormValue(t, inputs, "credentials/clientId", "key")
	setProfileFormValue(t, inputs, "credentials/clientSecret", "secret")

	value, err := buildCatalogProfile(profiles, spec, inputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if value.Infisical == nil || value.Infisical.SiteURL != "https://app.infisical.com" {
		t.Fatalf("Catalog defaults were not decoded: %#v", value.Infisical)
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
