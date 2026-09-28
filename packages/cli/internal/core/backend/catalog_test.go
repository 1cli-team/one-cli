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
		"env/infisical",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Builtin().SortedPairs() = %#v, want %#v", got, want)
	}
}

func TestNewRejectsDuplicateAndMalformedSpecs(t *testing.T) {
	t.Parallel()

	valid := spec(
		BackendID{Domain: DomainEnv, Name: "test"},
		[]Capability{CapabilityEnvGet},
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

func TestCatalogReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	c := Builtin()
	specs := c.All()
	specs[0].Capabilities[0] = "mutated"
	envSpecs := c.ForDomain(DomainEnv)
	envSpecs[0].Capabilities[0] = "mutated"

	got, ok := c.LookupPair("env/infisical")
	if !ok {
		t.Fatal("env/infisical not found")
	}
	if got.Capabilities[0] == "mutated" {
		t.Fatal("All() leaked mutable catalog storage")
	}
	env, ok := c.LookupPair("env/infisical")
	if !ok {
		t.Fatal("env/infisical not found")
	}
	if env.Capabilities[0] == "mutated" {
		t.Fatal("ForDomain() leaked mutable profile field storage")
	}
}

func TestNewRejectsInvalidProjectFieldMetadata(t *testing.T) {
	t.Parallel()

	valid := spec(
		BackendID{Domain: DomainEnv, Name: "test"},
		[]Capability{CapabilityEnvGet},
	)
	valid.Project = ProjectSpec{Configurable: true, Fields: []ProjectFieldSpec{{
		Path: "env", InputName: "environment", Type: ProjectFieldEnvironment,
		LabelKey: "project.fields.environment",
	}}}

	tests := []struct {
		name   string
		mutate func(*BackendSpec)
	}{
		{
			name: "fields require configurable flag",
			mutate: func(value *BackendSpec) {
				value.Project.Configurable = false
			},
		},
		{
			name: "configurable requires fields",
			mutate: func(value *BackendSpec) {
				value.Project.Fields = nil
			},
		},
		{
			name: "complete metadata",
			mutate: func(value *BackendSpec) {
				value.Project.Fields[0].LabelKey = ""
			},
		},
		{
			name: "safe config path",
			mutate: func(value *BackendSpec) {
				value.Project.Fields[0].Path = "credentials//token"
			},
		},
		{
			name: "known field type",
			mutate: func(value *BackendSpec) {
				value.Project.Fields[0].Type = "secret"
			},
		},
		{
			name: "unique path",
			mutate: func(value *BackendSpec) {
				value.Project.Fields = append(value.Project.Fields, ProjectFieldSpec{
					Path: "env", InputName: "other", Type: ProjectFieldString,
					LabelKey: "project.fields.other",
				})
			},
		},
		{
			name: "unique input name",
			mutate: func(value *BackendSpec) {
				value.Project.Fields = append(value.Project.Fields, ProjectFieldSpec{
					Path: "other", InputName: "environment", Type: ProjectFieldString,
					LabelKey: "project.fields.other",
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			candidate := cloneSpec(valid)
			tt.mutate(&candidate)
			if _, err := New(candidate); err == nil {
				t.Fatalf("New() accepted invalid project schema: %#v", candidate.Project)
			}
		})
	}
}

func TestBackendSpecJSONIncludesNormalizedIdentity(t *testing.T) {
	t.Parallel()

	backend, ok := Builtin().LookupPair("env/infisical")
	if !ok {
		t.Fatal("env/infisical not found")
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
	if got.ID != "env/infisical" || got.Domain != DomainEnv || got.Name != "infisical" {
		t.Fatalf("identity = %#v", got)
	}

}
