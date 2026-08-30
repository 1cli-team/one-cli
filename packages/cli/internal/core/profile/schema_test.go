package profile

import (
	"encoding/json"
	"slices"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

func TestSchemaAPIUsesCatalogSpecForTypedAccess(t *testing.T) {
	t.Parallel()

	spec, ok := catalog.Builtin().Lookup(catalog.DomainContainer, "ghcr")
	if !ok {
		t.Fatal("container/ghcr is absent from Catalog")
	}
	config := &Config{ContainerGHCR: Section[ContainerProfile]{
		Default: "work",
		Profiles: map[string]ContainerProfile{
			"work": {Namespace: "team", CredentialSource: SourceFile},
		},
	}}
	snapshot, ok := InspectSection(config, spec)
	if !ok || snapshot.Default != "work" || len(snapshot.Names) != 1 || snapshot.Names[0] != "work" {
		t.Fatalf("InspectSection() = (%+v, %v)", snapshot, ok)
	}
	stored, ok := LookupStored(config, spec, "work")
	if !ok || stored.Container == nil || stored.Container.Namespace != "team" {
		t.Fatalf("LookupStored() = (%+v, %v)", stored, ok)
	}
	if got := CredentialSource(spec, stored); got != SourceFile {
		t.Fatalf("CredentialSource() = %q, want %q", got, SourceFile)
	}

	decoded, err := Decode(spec, json.RawMessage(`{"namespace":"next"}`))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Backend != "ghcr" || decoded.Container == nil || decoded.Container.Namespace != "next" {
		t.Fatalf("Decode() = %+v", decoded)
	}
	decoded.Backend = "preserved"
	if err := ReplacePayload(spec, &decoded, json.RawMessage(`{"namespace":"final"}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Backend != "preserved" || decoded.Container.Namespace != "final" {
		t.Fatalf("ReplacePayload() = %+v", decoded)
	}
}

func TestValidateCatalogRejectsTypedFieldDrift(t *testing.T) {
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
	if err := ValidateCatalog(backendCatalog); err == nil {
		t.Fatal("ValidateCatalog() accepted a field absent from the typed payload")
	}
}

func TestSchemaPoliciesCoverCatalogInConfigOrder(t *testing.T) {
	wantOrder := configSchemaPairs()
	gotOrder := make([]string, 0, len(schemaPoliciesOrdered))
	for _, policy := range schemaPoliciesOrdered {
		gotOrder = append(gotOrder, policy.spec.Pair)
	}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("schema policy order = %v, want Config order %v", gotOrder, wantOrder)
	}

	wantCount := 0
	for _, spec := range catalog.Builtin().All() {
		if spec.Profile.Type == "" {
			continue
		}
		wantCount++
		if _, ok := schemaPoliciesByPair[spec.Pair]; !ok {
			t.Errorf("Catalog backend %q has no schema policy", spec.Pair)
		}
	}
	if len(schemaPoliciesOrdered) != wantCount {
		t.Fatalf("schema policy count = %d, want %d Catalog profile backends", len(schemaPoliciesOrdered), wantCount)
	}
}

func TestSchemaPoliciesProvideUniformCRUD(t *testing.T) {
	for _, policy := range schemaPoliciesOrdered {
		policy := policy
		t.Run(policy.spec.Pair, func(t *testing.T) {
			config := &Config{Version: SchemaVersion}
			value := profileForSchemaTest(t, policy.spec)

			if err := policy.write(config, "work", value, false); err != nil {
				t.Fatalf("write: %v", err)
			}
			if got := policy.defaultName(config); got != "work" {
				t.Fatalf("default = %q, want work", got)
			}
			stored, ok := policy.lookup(config, "work")
			if !ok || stored.Backend != policy.spec.ID.Name {
				t.Fatalf("lookup = (%+v, %v), want backend %q", stored, ok, policy.spec.ID.Name)
			}
			if names := policy.names(config); len(names) != 1 || names[0] != "work" {
				t.Fatalf("names = %v, want [work]", names)
			}

			policy.remove(config, "work")
			if _, ok := policy.lookup(config, "work"); ok {
				t.Fatal("removed profile is still present")
			}
			if got := policy.defaultName(config); got != "" {
				t.Fatalf("default after remove = %q, want empty", got)
			}
		})
	}
}

func profileForSchemaTest(t *testing.T, spec catalog.BackendSpec) Profile {
	t.Helper()
	value := Profile{Backend: spec.ID.Name}
	switch spec.Profile.Type {
	case catalog.ProfileTypeDotenv:
		value.Dotenv = &DotenvProfile{}
	case catalog.ProfileTypeInfisical:
		value.Infisical = &InfisicalProfile{}
	case catalog.ProfileTypeS3:
		value.S3 = &S3Profile{}
	case catalog.ProfileTypeKustomize:
		value.Kustomize = &KustomizeProfile{}
	case catalog.ProfileTypeVercel:
		value.Vercel = &VercelProfile{}
	case catalog.ProfileTypeCloudflare:
		value.Cloudflare = &CloudflareProfile{}
	case catalog.ProfileTypeEdgeOne:
		value.EdgeOne = &EdgeOneProfile{}
	case catalog.ProfileTypeContainer:
		value.Container = &ContainerProfile{}
	default:
		t.Fatalf("unsupported profile type %q", spec.Profile.Type)
	}
	return value
}
