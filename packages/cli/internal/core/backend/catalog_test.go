package backend

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuiltinPairs(t *testing.T) {
	t.Parallel()

	got := Builtin().SortedPairs()
	want := []string{
		"container/acr",
		"container/docker",
		"container/dockerhub",
		"container/ghcr",
		"deploy/aliyun-oss",
		"deploy/aws-s3",
		"deploy/cloudflare",
		"deploy/edgeone",
		"deploy/kustomize",
		"deploy/minio",
		"deploy/r2",
		"deploy/rustfs",
		"deploy/tencent-cos",
		"deploy/vercel",
		"env/dotenv",
		"env/infisical",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Builtin().SortedPairs() = %#v, want %#v", got, want)
	}
}

func TestBuiltinProfileBackendsExcludeDotenv(t *testing.T) {
	t.Parallel()

	got := Builtin().ProfileBackends()
	if len(got) != 15 {
		t.Fatalf("len(ProfileBackends()) = %d, want 15", len(got))
	}
	for _, spec := range got {
		if spec.Pair == "env/dotenv" {
			t.Fatal("env/dotenv must not expose a configure profile")
		}
	}
}

func TestNewRejectsDuplicateAndMalformedSpecs(t *testing.T) {
	t.Parallel()

	valid := spec(
		BackendID{Domain: DomainEnv, Name: "test"},
		[]Capability{CapabilityEnvGet},
		ProfileSpec{},
	)
	if _, err := New(valid, valid); err == nil {
		t.Fatal("New() accepted duplicate backend")
	}

	malformed := valid
	malformed.Pair = "deploy/test"
	if _, err := New(malformed); err == nil {
		t.Fatal("New() accepted mismatched pair")
	}
}

func TestNewRejectsConfigurableBackendWithoutProfileType(t *testing.T) {
	t.Parallel()

	_, err := New(spec(
		BackendID{Domain: DomainEnv, Name: "test"},
		[]Capability{CapabilityEnvGet},
		ProfileSpec{Configurable: true},
	))
	if err == nil {
		t.Fatal("New() accepted configurable backend without profile type")
	}
}

func TestNewRejectsInvalidProfileFieldMetadata(t *testing.T) {
	t.Parallel()

	base := spec(
		BackendID{Domain: DomainEnv, Name: "test"},
		[]Capability{CapabilityEnvGet},
		ProfileSpec{Configurable: true, Type: ProfileTypeInfisical},
	)
	base.Profile.Fields = []FieldSpec{
		{Path: "siteUrl", InputName: "site-url", Type: FieldString, LabelKey: "site"},
		{Path: "credentials/clientId", InputName: "site-url", Type: FieldString, LabelKey: "client"},
	}
	if _, err := New(base); err == nil {
		t.Fatal("New() accepted duplicate profile input names")
	}

	base.Profile.Fields = []FieldSpec{{
		Path: "credentials/clientSecret", InputName: "client-secret", Type: FieldSecret,
		LabelKey: "secret", Default: "must-not-be-stored",
	}}
	if _, err := New(base); err == nil {
		t.Fatal("New() accepted a default secret")
	}
}

func TestCatalogReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	c := Builtin()
	specs := c.All()
	specs[0].Capabilities[0] = "mutated"

	got, ok := c.LookupPair("env/dotenv")
	if !ok {
		t.Fatal("env/dotenv not found")
	}
	if got.Capabilities[0] == "mutated" {
		t.Fatal("All() leaked mutable catalog storage")
	}
}

func TestProfileFieldsNeverExposeCredentialValues(t *testing.T) {
	t.Parallel()

	for _, backend := range Builtin().ProfileBackends() {
		for _, field := range backend.Profile.Fields {
			if field.Path == "" || field.InputName == "" || field.LabelKey == "" {
				t.Fatalf("%s has incomplete field metadata: %#v", backend.Pair, field)
			}
			if field.Type == FieldSecret && field.Default != nil {
				t.Fatalf("%s secret %s must not declare a default", backend.Pair, field.Path)
			}
		}
	}
}

func TestBuiltinBackendsDeclareProfileType(t *testing.T) {
	t.Parallel()

	for _, backend := range Builtin().All() {
		if backend.Profile.Type == "" {
			t.Fatalf("%s has no profile type", backend.Pair)
		}
	}
}

func TestBackendSpecJSONIncludesNormalizedIdentity(t *testing.T) {
	t.Parallel()

	backend, ok := Builtin().LookupPair("deploy/vercel")
	if !ok {
		t.Fatal("deploy/vercel not found")
	}
	raw, err := json.Marshal(backend)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ID     string `json:"id"`
		Domain Domain `json:"domain"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "deploy/vercel" || got.Domain != DomainDeploy || got.Name != "vercel" {
		t.Fatalf("identity = %#v", got)
	}
}
