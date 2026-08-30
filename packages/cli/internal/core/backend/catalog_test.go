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
	deploySpecs := c.ForDomain(DomainDeploy)
	deploySpecs[0].Project.Fields[0].Path = "mutated"

	got, ok := c.LookupPair("env/dotenv")
	if !ok {
		t.Fatal("env/dotenv not found")
	}
	if got.Capabilities[0] == "mutated" {
		t.Fatal("All() leaked mutable catalog storage")
	}
	deploy, ok := c.LookupPair("deploy/aliyun-oss")
	if !ok {
		t.Fatal("deploy/aliyun-oss not found")
	}
	if deploy.Project.Fields[0].Path == "mutated" {
		t.Fatal("ForDomain() leaked mutable project field storage")
	}
}

func TestNewRejectsInvalidProjectFieldMetadata(t *testing.T) {
	t.Parallel()

	valid := spec(
		BackendID{Domain: DomainDeploy, Name: "test"},
		[]Capability{CapabilityDeploy},
		ProfileSpec{},
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
			name: "configurable requires deploy capability",
			mutate: func(value *BackendSpec) {
				value.Capabilities = []Capability{CapabilityScaffold}
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

func TestBuiltinDeployProjectFields(t *testing.T) {
	t.Parallel()

	wantS3 := []ProjectFieldSpec{
		{Path: "bucket", InputName: "bucket", Type: ProjectFieldString, LabelKey: "project.fields.bucket", Placeholder: "my-static-site"},
		{Path: "env", InputName: "environment", Type: ProjectFieldEnvironment, LabelKey: "project.fields.environment"},
	}
	for _, name := range []string{
		DeployAliyunOSS, DeployTencentCOS, DeployAWSS3,
		DeployMinIO, DeployRustFS, DeployR2,
	} {
		got, ok := Builtin().Lookup(DomainDeploy, name)
		if !ok {
			t.Fatalf("deploy/%s not found", name)
		}
		if !got.Project.Configurable || !reflect.DeepEqual(got.Project.Fields, wantS3) {
			t.Fatalf("deploy/%s project schema = %#v, want %#v", name, got.Project, wantS3)
		}
	}

	tests := []struct {
		name string
		want []ProjectFieldSpec
	}{
		{
			name: DeployKustomize,
			want: []ProjectFieldSpec{{Path: "env", InputName: "environment", Type: ProjectFieldEnvironment, LabelKey: "project.fields.environment"}},
		},
		{
			name: DeployVercel,
			want: []ProjectFieldSpec{
				{Path: "projectId", InputName: "project-id", Type: ProjectFieldString, LabelKey: "project.fields.projectId", Placeholder: "prj_..."},
				{Path: "projectName", InputName: "project-name", Type: ProjectFieldString, LabelKey: "project.fields.projectName", Placeholder: "my-project"},
				{Path: "env", InputName: "environment", Type: ProjectFieldEnvironment, LabelKey: "project.fields.environment"},
			},
		},
		{
			name: DeployCloudflare,
			want: []ProjectFieldSpec{
				{Path: "workerName", InputName: "worker-name", Type: ProjectFieldString, LabelKey: "project.fields.workerName", Placeholder: "my-worker"},
				{Path: "env", InputName: "environment", Type: ProjectFieldEnvironment, LabelKey: "project.fields.environment"},
			},
		},
		{
			name: DeployEdgeOne,
			want: []ProjectFieldSpec{
				{Path: "projectName", InputName: "project-name", Type: ProjectFieldString, LabelKey: "project.fields.projectName", Placeholder: "my-project"},
				{Path: "env", InputName: "environment", Type: ProjectFieldEnvironment, LabelKey: "project.fields.environment"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Builtin().Lookup(DomainDeploy, tt.name)
			if !ok {
				t.Fatalf("deploy/%s not found", tt.name)
			}
			if !got.Project.Configurable || !reflect.DeepEqual(got.Project.Fields, tt.want) {
				t.Fatalf("deploy/%s project schema = %#v, want %#v", tt.name, got.Project, tt.want)
			}
		})
	}

	for _, domain := range []Domain{DomainEnv, DomainContainer} {
		for _, got := range Builtin().ForDomain(domain) {
			if got.Project.Configurable || len(got.Project.Fields) != 0 {
				t.Fatalf("%s must not declare backend-specific project fields: %#v", got.Pair, got.Project)
			}
		}
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
		ID      string      `json:"id"`
		Domain  Domain      `json:"domain"`
		Name    string      `json:"name"`
		Project ProjectSpec `json:"project"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "deploy/vercel" || got.Domain != DomainDeploy || got.Name != "vercel" {
		t.Fatalf("identity = %#v", got)
	}
	if !got.Project.Configurable || len(got.Project.Fields) != 3 {
		t.Fatalf("project schema = %#v", got.Project)
	}
	if got.Project.Fields[0].Path != "projectId" || got.Project.Fields[2].Type != ProjectFieldEnvironment {
		t.Fatalf("project fields = %#v", got.Project.Fields)
	}
}
