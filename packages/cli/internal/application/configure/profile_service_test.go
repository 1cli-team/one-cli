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
	if domain == profile.DomainDeploy && backend == "vercel" && value.Vercel != nil {
		if r.config.DeployVercel.Profiles == nil {
			r.config.DeployVercel.Profiles = map[string]profile.VercelProfile{}
		}
		r.config.DeployVercel.Profiles[name] = *value.Vercel
		if setDefault || r.config.DeployVercel.Default == "" {
			r.config.DeployVercel.Default = name
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
		"deploy/aliyun-oss",
		"deploy/tencent-cos",
		"deploy/aws-s3",
		"deploy/minio",
		"deploy/rustfs",
		"deploy/r2",
		"deploy/kustomize",
		"deploy/vercel",
		"deploy/cloudflare",
		"deploy/edgeone",
		"container/docker",
		"container/dockerhub",
		"container/ghcr",
		"container/acr",
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
		"ws-demo", "web", profile.DomainDeploy, catalog.DeployVercel,
	); err != nil {
		t.Fatal(err)
	}
	if repository.unbound.workspaceID != "ws-demo" ||
		repository.unbound.projectName != "web" ||
		repository.unbound.domain != profile.DomainDeploy ||
		repository.unbound.backend != catalog.DeployVercel {
		t.Fatalf("unbind input = %#v", repository.unbound)
	}
}

func TestProfileServiceDecodesTypedProfile(t *testing.T) {
	t.Parallel()

	service := testProfileService(t, &profileRepositoryStub{config: &profile.Config{}})
	value, err := service.DecodeProfile(
		profile.DomainContainer,
		"ghcr",
		json.RawMessage(`{"namespace":"team","credentials":{"username":"octo","password":"token"}}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.Container == nil || value.Container.Namespace != "team" ||
		value.Container.Credentials == nil || value.Container.Credentials.Password != "token" {
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
		DeployVercel: profile.Section[profile.VercelProfile]{
			Default: "production",
			Profiles: map[string]profile.VercelProfile{
				"production": {
					Team:        "old-team",
					Credentials: &profile.VercelCredentials{APIToken: "real-token"},
				},
			},
		},
	}}
	service := testProfileService(t, repository)
	result, err := service.Upsert(UpsertProfileInput{
		Domain:  profile.DomainDeploy,
		Backend: "vercel",
		Name:    "production",
		Profile: profile.Profile{
			Backend: "vercel",
			Vercel: &profile.VercelProfile{
				Team:        "new-team",
				Credentials: &profile.VercelCredentials{APIToken: MaskedCredential},
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
	if got := repository.upsert.Vercel.Credentials.APIToken; got != "real-token" {
		t.Fatalf("saved token = %q, want preserved real token", got)
	}
}

func TestProfileServiceMaskPolicies(t *testing.T) {
	t.Parallel()

	service := testProfileService(t, &profileRepositoryStub{config: &profile.Config{}})
	config := profile.Config{DeployAWSS3: profile.Section[profile.S3Profile]{
		Profiles: map[string]profile.S3Profile{
			"work": {Credentials: &profile.S3Credentials{
				AccessKeyID: "visible-id", AccessKeySecret: "secret",
			}},
		},
	}}
	maskedConfig, err := service.MaskConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	credentials := maskedConfig.DeployAWSS3.Profiles["work"].Credentials
	if credentials.AccessKeyID != "visible-id" || credentials.AccessKeySecret != MaskedCredential {
		t.Fatalf("HTTP mask = %#v", credentials)
	}
	maskedProfile, err := service.MaskProfile(profile.Profile{
		S3: &profile.S3Profile{Credentials: &profile.S3Credentials{
			AccessKeyID: "id", AccessKeySecret: "secret",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := maskedProfile.S3.Credentials; got.AccessKeyID != MaskedCredential || got.AccessKeySecret != MaskedCredential {
		t.Fatalf("CLI mask = %#v", got)
	}
}
