package profile

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// sectionPolicy is the single typed access path for one schema-v1
// (domain/backend) section. The table below is built in Config field order and
// checked against the Backend Catalog during package initialization.
type sectionPolicy struct {
	spec catalog.BackendSpec

	valueType  reflect.Type
	payload    func(Profile) (any, bool)
	setPayload func(*Profile, json.RawMessage) error

	configValue func(*Config) any
	configEmpty func(*Config) bool
	defaultName func(*Config) string
	names       func(*Config) []string
	lookup      func(*Config, string) (Profile, bool)
	write       func(*Config, string, Profile, bool) error
	remove      func(*Config, string)
	setDefault  func(*Config, string)

	credentialSource   func(Profile) string
	credentialValue    func(*CredentialsFile) any
	credentialEmpty    func(*CredentialsFile) bool
	mergeCredentials   func(*Config, *CredentialsFile)
	extractCredentials func(*Config, *CredentialsFile)
	stripCredentials   func(*Config)
}

func newSectionPolicy[T any](
	spec catalog.BackendSpec,
	selectSection func(*Config) *Section[T],
	getPayload func(Profile) *T,
	setPayload func(*Profile, *T),
	allowZeroPayload bool,
) *sectionPolicy {
	return &sectionPolicy{
		spec:      spec,
		valueType: reflect.TypeOf((*T)(nil)).Elem(),
		payload: func(value Profile) (any, bool) {
			payload := getPayload(value)
			return payload, payload != nil
		},
		setPayload: func(value *Profile, raw json.RawMessage) error {
			var payload T
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &payload); err != nil {
					return err
				}
			}
			setPayload(value, &payload)
			return nil
		},
		configValue: func(config *Config) any {
			section := selectSection(config)
			if section == nil {
				return nil
			}
			return *section
		},
		configEmpty: func(config *Config) bool {
			section := selectSection(config)
			return section == nil || section.IsEmpty()
		},
		defaultName: func(config *Config) string {
			section := selectSection(config)
			if section == nil {
				return ""
			}
			return section.Default
		},
		names: func(config *Config) []string {
			section := selectSection(config)
			if section == nil {
				return nil
			}
			return mapKeys(section.Profiles)
		},
		lookup: func(config *Config, name string) (Profile, bool) {
			section := selectSection(config)
			if section == nil {
				return Profile{}, false
			}
			payload, ok := section.Profiles[name]
			if !ok {
				return Profile{}, false
			}
			value := Profile{Backend: spec.ID.Name}
			setPayload(&value, &payload)
			return value, true
		},
		write: func(config *Config, name string, value Profile, setDefault bool) error {
			section := selectSection(config)
			if section == nil {
				return invalidProfilePair(spec.ID.Domain, spec.ID.Name)
			}
			var payload T
			selected := getPayload(value)
			if selected == nil && !allowZeroPayload {
				return cliErrors.New(
					cliErrors.PROFILE_BACKEND_INVALID,
					fmt.Sprintf("profile 缺 %s 的 sub-profile 数据", spec.ID.Name),
				)
			}
			if selected != nil {
				payload = *selected
			}
			if section.Profiles == nil {
				section.Profiles = map[string]T{}
			}
			section.Profiles[name] = payload
			if setDefault || section.Default == "" {
				section.Default = name
			}
			return nil
		},
		remove: func(config *Config, name string) {
			section := selectSection(config)
			if section == nil {
				return
			}
			delete(section.Profiles, name)
			if section.Default == name {
				section.Default = ""
			}
		},
		setDefault: func(config *Config, name string) {
			section := selectSection(config)
			if section != nil {
				section.Default = name
			}
		},
		credentialSource: func(Profile) string { return "" },
	}
}

func newCredentialSectionPolicy[T, C any](
	spec catalog.BackendSpec,
	selectSection func(*Config) *Section[T],
	getPayload func(Profile) *T,
	setPayload func(*Profile, *T),
	selectCredentialSection func(*CredentialsFile) *CredSection[C],
	credentialSource func(T) string,
	getCredentials func(T) *C,
	setCredentials func(*T, *C),
) *sectionPolicy {
	policy := newSectionPolicy(spec, selectSection, getPayload, setPayload, false)
	policy.credentialSource = func(value Profile) string {
		payload := getPayload(value)
		if payload == nil {
			return ""
		}
		return credentialSource(*payload)
	}
	policy.credentialValue = func(credentials *CredentialsFile) any {
		section := selectCredentialSection(credentials)
		if section == nil {
			return nil
		}
		return *section
	}
	policy.credentialEmpty = func(credentials *CredentialsFile) bool {
		section := selectCredentialSection(credentials)
		return section == nil || section.IsEmpty()
	}
	policy.mergeCredentials = func(config *Config, credentials *CredentialsFile) {
		section := selectSection(config)
		credentialSection := selectCredentialSection(credentials)
		if section == nil || credentialSection == nil {
			return
		}
		for name, payload := range section.Profiles {
			if !IsFileSource(credentialSource(payload)) {
				continue
			}
			stored, ok := credentialSection.Profiles[name]
			if !ok {
				continue
			}
			copy := stored
			setCredentials(&payload, &copy)
			section.Profiles[name] = payload
		}
	}
	policy.extractCredentials = func(config *Config, credentials *CredentialsFile) {
		section := selectSection(config)
		credentialSection := selectCredentialSection(credentials)
		if section == nil || credentialSection == nil {
			return
		}
		for name, payload := range section.Profiles {
			value := getCredentials(payload)
			if !IsFileSource(credentialSource(payload)) || value == nil {
				continue
			}
			if credentialSection.Profiles == nil {
				credentialSection.Profiles = map[string]C{}
			}
			credentialSection.Profiles[name] = *value
		}
	}
	policy.stripCredentials = func(config *Config) {
		section := selectSection(config)
		if section == nil || section.Profiles == nil {
			return
		}
		profiles := make(map[string]T, len(section.Profiles))
		for name, payload := range section.Profiles {
			setCredentials(&payload, nil)
			profiles[name] = payload
		}
		section.Profiles = profiles
	}
	return policy
}

type sectionPolicyFactory func(catalog.BackendSpec) *sectionPolicy

var sectionPolicyFactories = map[catalog.ProfileType]sectionPolicyFactory{
	catalog.ProfileTypeDotenv: func(spec catalog.BackendSpec) *sectionPolicy {
		return newSectionPolicy(
			spec,
			func(config *Config) *Section[DotenvProfile] { return &config.EnvDotenv },
			func(value Profile) *DotenvProfile { return value.Dotenv },
			func(value *Profile, payload *DotenvProfile) { value.Dotenv = payload },
			true,
		)
	},
	catalog.ProfileTypeInfisical: func(spec catalog.BackendSpec) *sectionPolicy {
		return newCredentialSectionPolicy(
			spec,
			func(config *Config) *Section[InfisicalProfile] { return &config.EnvInfisical },
			func(value Profile) *InfisicalProfile { return value.Infisical },
			func(value *Profile, payload *InfisicalProfile) { value.Infisical = payload },
			func(credentials *CredentialsFile) *CredSection[InfisicalCredentials] {
				return &credentials.EnvInfisical
			},
			func(value InfisicalProfile) string { return value.CredentialSource },
			func(value InfisicalProfile) *InfisicalCredentials { return value.Credentials },
			func(value *InfisicalProfile, credentials *InfisicalCredentials) { value.Credentials = credentials },
		)
	},
	catalog.ProfileTypeS3: func(spec catalog.BackendSpec) *sectionPolicy {
		return newCredentialSectionPolicy(
			spec,
			func(config *Config) *Section[S3Profile] { return config.S3CompatSection(spec.ID.Name) },
			func(value Profile) *S3Profile { return value.S3 },
			func(value *Profile, payload *S3Profile) { value.S3 = payload },
			func(credentials *CredentialsFile) *CredSection[S3Credentials] {
				return credentials.S3CompatCredSection(spec.ID.Name)
			},
			func(value S3Profile) string { return value.CredentialSource },
			func(value S3Profile) *S3Credentials { return value.Credentials },
			func(value *S3Profile, credentials *S3Credentials) { value.Credentials = credentials },
		)
	},
	catalog.ProfileTypeKustomize: func(spec catalog.BackendSpec) *sectionPolicy {
		return newSectionPolicy(
			spec,
			func(config *Config) *Section[KustomizeProfile] { return &config.DeployKustomize },
			func(value Profile) *KustomizeProfile { return value.Kustomize },
			func(value *Profile, payload *KustomizeProfile) { value.Kustomize = payload },
			false,
		)
	},
	catalog.ProfileTypeVercel: func(spec catalog.BackendSpec) *sectionPolicy {
		return newCredentialSectionPolicy(
			spec,
			func(config *Config) *Section[VercelProfile] { return &config.DeployVercel },
			func(value Profile) *VercelProfile { return value.Vercel },
			func(value *Profile, payload *VercelProfile) { value.Vercel = payload },
			func(credentials *CredentialsFile) *CredSection[VercelCredentials] {
				return &credentials.DeployVercel
			},
			func(value VercelProfile) string { return value.CredentialSource },
			func(value VercelProfile) *VercelCredentials { return value.Credentials },
			func(value *VercelProfile, credentials *VercelCredentials) { value.Credentials = credentials },
		)
	},
	catalog.ProfileTypeCloudflare: func(spec catalog.BackendSpec) *sectionPolicy {
		return newCredentialSectionPolicy(
			spec,
			func(config *Config) *Section[CloudflareProfile] { return &config.DeployCloudflare },
			func(value Profile) *CloudflareProfile { return value.Cloudflare },
			func(value *Profile, payload *CloudflareProfile) { value.Cloudflare = payload },
			func(credentials *CredentialsFile) *CredSection[CloudflareCredentials] {
				return &credentials.DeployCloudflare
			},
			func(value CloudflareProfile) string { return value.CredentialSource },
			func(value CloudflareProfile) *CloudflareCredentials { return value.Credentials },
			func(value *CloudflareProfile, credentials *CloudflareCredentials) { value.Credentials = credentials },
		)
	},
	catalog.ProfileTypeEdgeOne: func(spec catalog.BackendSpec) *sectionPolicy {
		return newCredentialSectionPolicy(
			spec,
			func(config *Config) *Section[EdgeOneProfile] { return &config.DeployEdgeOne },
			func(value Profile) *EdgeOneProfile { return value.EdgeOne },
			func(value *Profile, payload *EdgeOneProfile) { value.EdgeOne = payload },
			func(credentials *CredentialsFile) *CredSection[EdgeOneCredentials] {
				return &credentials.DeployEdgeOne
			},
			func(value EdgeOneProfile) string { return value.CredentialSource },
			func(value EdgeOneProfile) *EdgeOneCredentials { return value.Credentials },
			func(value *EdgeOneProfile, credentials *EdgeOneCredentials) { value.Credentials = credentials },
		)
	},
	catalog.ProfileTypeContainer: func(spec catalog.BackendSpec) *sectionPolicy {
		return newCredentialSectionPolicy(
			spec,
			func(config *Config) *Section[ContainerProfile] { return config.ContainerKindSection(spec.ID.Name) },
			func(value Profile) *ContainerProfile { return value.Container },
			func(value *Profile, payload *ContainerProfile) { value.Container = payload },
			func(credentials *CredentialsFile) *CredSection[ContainerCredentials] {
				return credentials.ContainerKindCredSection(spec.ID.Name)
			},
			func(value ContainerProfile) string { return value.CredentialSource },
			func(value ContainerProfile) *ContainerCredentials { return value.Credentials },
			func(value *ContainerProfile, credentials *ContainerCredentials) { value.Credentials = credentials },
		)
	},
}

var schemaPoliciesOrdered, schemaPoliciesByPair = mustBuildSchemaPolicies()

func mustBuildSchemaPolicies() ([]*sectionPolicy, map[string]*sectionPolicy) {
	backendCatalog := catalog.Builtin()
	pairs := configSchemaPairs()
	ordered := make([]*sectionPolicy, 0, len(pairs))
	byPair := make(map[string]*sectionPolicy, len(pairs))
	for _, pair := range pairs {
		spec, ok := backendCatalog.LookupPair(pair)
		if !ok {
			panic(fmt.Sprintf("profile: schema section %q is absent from Backend Catalog", pair))
		}
		factory, ok := sectionPolicyFactories[spec.Profile.Type]
		if !ok {
			panic(fmt.Sprintf("profile: schema section %q has unsupported profile type %q", pair, spec.Profile.Type))
		}
		policy := factory(spec)
		if policy.configValue(&Config{}) == nil {
			panic(fmt.Sprintf("profile: schema section %q has no typed Config accessor", pair))
		}
		if policy.credentialValue != nil && policy.credentialValue(&CredentialsFile{}) == nil {
			panic(fmt.Sprintf("profile: schema section %q has no typed CredentialsFile accessor", pair))
		}
		ordered = append(ordered, policy)
		byPair[pair] = policy
	}

	expected := 0
	for _, spec := range backendCatalog.All() {
		if spec.Profile.Type == "" {
			continue
		}
		expected++
		if _, ok := byPair[spec.Pair]; !ok {
			panic(fmt.Sprintf("profile: Catalog backend %q has no schema-v1 section", spec.Pair))
		}
	}
	if len(ordered) != expected {
		panic(fmt.Sprintf("profile: schema has %d sections, Catalog has %d profile backends", len(ordered), expected))
	}
	return ordered, byPair
}

func configSchemaPairs() []string {
	typeOfConfig := reflect.TypeOf(Config{})
	pairs := make([]string, 0, typeOfConfig.NumField())
	for index := 0; index < typeOfConfig.NumField(); index++ {
		name := strings.Split(typeOfConfig.Field(index).Tag.Get("json"), ",")[0]
		if strings.Contains(name, "/") {
			pairs = append(pairs, name)
		}
	}
	return pairs
}

func schemaPolicy(domain Domain, backend string) (*sectionPolicy, bool) {
	policy, ok := schemaPoliciesByPair[SectionKey(domain, backend)]
	return policy, ok
}

func invalidProfilePair(domain Domain, backend string) error {
	return cliErrors.New(
		cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("(%s, %s) 不是支持的 (domain, backend) 组合", domain, backend),
	)
}

func mapKeys[T any](values map[string]T) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}
