package configure

import (
	"encoding/json"
	"reflect"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
)

type profileRepositoryStub struct {
	config  *profile.Config
	upsert  profile.Profile
	updated bool
	unbound struct {
		workspaceID string
		projectName string
		domain      profile.Domain
		backend     string
	}
}

func (r *profileRepositoryStub) Load() (*profile.Config, *profile.CredentialsFile, error) {
	return r.config, &profile.CredentialsFile{Version: profile.SchemaVersion}, nil
}

func (r *profileRepositoryStub) Upsert(
	domain profile.Domain,
	backend, name string,
	value profile.Profile,
	setDefault bool,
) (bool, error) {
	r.upsert = value
	if domain == profile.DomainEnv && backend == "infisical" && value.Infisical != nil {
		if r.config.EnvInfisical.Profiles == nil {
			r.config.EnvInfisical.Profiles = map[string]profile.InfisicalProfile{}
		}
		r.config.EnvInfisical.Profiles[name] = *value.Infisical
		if setDefault || r.config.EnvInfisical.Default == "" {
			r.config.EnvInfisical.Default = name
		}
	}
	return r.updated, nil
}

func (*profileRepositoryStub) Remove(profile.Domain, string, string) error     { return nil }
func (*profileRepositoryStub) SetDefault(profile.Domain, string, string) error { return nil }
func (*profileRepositoryStub) BindWorkspaceProfile(
	string, string, string, string, profile.Domain, string, string,
) error {
	return nil
}
func (r *profileRepositoryStub) UnbindWorkspaceProfile(
	workspaceID, projectName string, domain profile.Domain, backend string,
) error {
	r.unbound.workspaceID = workspaceID
	r.unbound.projectName = projectName
	r.unbound.domain = domain
	r.unbound.backend = backend
	return nil
}

func (*profileRepositoryStub) BindEnvironmentProfile(
	string, string, string, string, string, profile.Domain, string, string,
) error {
	return nil
}

func (*profileRepositoryStub) UnbindEnvironmentProfile(
	string, string, string, profile.Domain, string,
) error {
	return nil
}
func (*profileRepositoryStub) EnvironmentProfileBinding(
	string, string, string, profile.Domain, string,
) (string, error) {
	return "", nil
}
func (*profileRepositoryStub) Resolve(profile.ResolveInput) (*profile.Resolved, error) {
	return nil, nil
}
func (*profileRepositoryStub) ConfigPath() (string, error)      { return "/config.json", nil }
func (*profileRepositoryStub) CredentialsPath() (string, error) { return "/credentials.json", nil }

func testProfileService(t *testing.T, repository ProfileRepository) *ProfileService {
	t.Helper()
	service, err := NewProfileService(catalog.Builtin(), repository)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestProfileServiceUsesCatalogOrder(t *testing.T) {
	t.Parallel()

	service := testProfileService(t, &profileRepositoryStub{config: &profile.Config{}})
	got := make([]string, 0)
	for _, backend := range service.ProfileBackends() {
		got = append(got, backend.Pair)
	}
	want := []string{
		"env/infisical",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProfileBackends() = %#v, want %#v", got, want)
	}
}

func TestProfileServiceUnbindsProjectProfileThroughRepository(t *testing.T) {
	t.Parallel()
	repository := &profileRepositoryStub{config: &profile.Config{}}
	service := testProfileService(t, repository)
	if err := service.UnbindWorkspaceProfile(
		"ws-demo", "web", profile.DomainEnv, catalog.EnvInfisical,
	); err != nil {
		t.Fatal(err)
	}
	if repository.unbound.workspaceID != "ws-demo" ||
		repository.unbound.projectName != "web" ||
		repository.unbound.domain != profile.DomainEnv ||
		repository.unbound.backend != catalog.EnvInfisical {
		t.Fatalf("unbind input = %#v", repository.unbound)
	}
}

func TestProfileServiceDecodesTypedProfile(t *testing.T) {
	t.Parallel()

	service := testProfileService(t, &profileRepositoryStub{config: &profile.Config{}})
	value, err := service.DecodeProfile(
		profile.DomainEnv,
		"infisical",
		json.RawMessage(`{"siteUrl":"https://app.infisical.com","credentials":{"clientId":"octo","clientSecret":"token"}}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.Infisical == nil || value.Infisical.SiteURL != "https://app.infisical.com" ||
		value.Infisical.Credentials == nil || value.Infisical.Credentials.ClientSecret != "token" {
		t.Fatalf("decoded profile = %#v", value)
	}
}

func TestProfileServiceRejectsCatalogProfileDrift(t *testing.T) {
	t.Parallel()

	backendCatalog, err := catalog.New(catalog.BackendSpec{
		ID:           catalog.BackendID{Domain: catalog.DomainEnv, Name: "infisical"},
		Pair:         "env/infisical",
		Capabilities: []catalog.Capability{catalog.CapabilityEnvGet},
		Profile: catalog.ProfileSpec{
			Configurable: true,
			Type:         catalog.ProfileTypeInfisical,
			Fields: []catalog.FieldSpec{{
				Path: "credentials/notARealField", InputName: "invalid", Type: catalog.FieldSecret, LabelKey: "test",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewProfileService(backendCatalog, &profileRepositoryStub{config: &profile.Config{}}); err == nil {
		t.Fatal("NewProfileService() accepted a Catalog field absent from the typed profile")
	}
}

func TestMaskConfigUsesCatalogFieldPolicy(t *testing.T) {
	t.Parallel()

	backendCatalog, err := catalog.New(catalog.BackendSpec{
		ID:           catalog.BackendID{Domain: catalog.DomainEnv, Name: "infisical"},
		Pair:         "env/infisical",
		Capabilities: []catalog.Capability{catalog.CapabilityEnvGet},
		Profile: catalog.ProfileSpec{
			Configurable: true,
			Type:         catalog.ProfileTypeInfisical,
			Fields: []catalog.FieldSpec{
				{Path: "credentials/clientId", InputName: "client-id", Type: catalog.FieldSecret, LabelKey: "test.clientId"},
				{Path: "credentials/clientSecret", InputName: "client-secret", Type: catalog.FieldString, LabelKey: "test.clientSecret"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewProfileService(backendCatalog, &profileRepositoryStub{config: &profile.Config{}})
	if err != nil {
		t.Fatal(err)
	}
	masked, err := service.MaskConfig(profile.Config{
		EnvInfisical: profile.Section[profile.InfisicalProfile]{
			Profiles: map[string]profile.InfisicalProfile{
				"work": {Credentials: &profile.InfisicalCredentials{
					ClientID: "catalog-secret", ClientSecret: "catalog-visible",
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	credentials := masked.EnvInfisical.Profiles["work"].Credentials
	if credentials.ClientID != MaskedCredential || credentials.ClientSecret != "catalog-visible" {
		t.Fatalf("Catalog mask policy was not applied: %#v", credentials)
	}
}

func TestProfileServicePreservesMaskedCredential(t *testing.T) {
	t.Parallel()

	repository := &profileRepositoryStub{config: &profile.Config{
		EnvInfisical: profile.Section[profile.InfisicalProfile]{
			Default: "production",
			Profiles: map[string]profile.InfisicalProfile{
				"production": {
					SiteURL:     "https://old.example.com",
					Credentials: &profile.InfisicalCredentials{ClientID: "client", ClientSecret: "real-token"},
				},
			},
		},
	}}
	service := testProfileService(t, repository)
	result, err := service.Upsert(UpsertProfileInput{
		Domain:  profile.DomainEnv,
		Backend: "infisical",
		Name:    "production",
		Profile: profile.Profile{
			Backend: "infisical",
			Infisical: &profile.InfisicalProfile{
				SiteURL:     "https://new.example.com",
				Credentials: &profile.InfisicalCredentials{ClientID: "client", ClientSecret: MaskedCredential},
			},
		},
		PreserveMasked: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Default {
		t.Fatal("updated default profile no longer reported as default")
	}
	if got := repository.upsert.Infisical.Credentials.ClientSecret; got != "real-token" {
		t.Fatalf("saved token = %q, want preserved real token", got)
	}
}

func TestProfileServiceMaskPolicies(t *testing.T) {
	t.Parallel()

	service := testProfileService(t, &profileRepositoryStub{config: &profile.Config{}})
	config := profile.Config{EnvInfisical: profile.Section[profile.InfisicalProfile]{
		Profiles: map[string]profile.InfisicalProfile{
			"work": {Credentials: &profile.InfisicalCredentials{
				ClientID: "visible-id", ClientSecret: "secret",
			}},
		},
	}}
	maskedConfig, err := service.MaskConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	credentials := maskedConfig.EnvInfisical.Profiles["work"].Credentials
	if credentials.ClientID != "visible-id" || credentials.ClientSecret != MaskedCredential {
		t.Fatalf("HTTP mask = %#v", credentials)
	}
	maskedProfile, err := service.MaskProfile(profile.Profile{
		Infisical: &profile.InfisicalProfile{Credentials: &profile.InfisicalCredentials{
			ClientID: "id", ClientSecret: "secret",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := maskedProfile.Infisical.Credentials; got.ClientID != MaskedCredential || got.ClientSecret != MaskedCredential {
		t.Fatalf("CLI mask = %#v", got)
	}
}
