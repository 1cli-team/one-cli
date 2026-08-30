// Package application contains transport-neutral use cases. CLI commands and
// HTTP handlers translate inputs and outputs; they do not implement profile
// storage, backend dispatch, masking, or catalog validation themselves.
package configure

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// ProfileRepository is the persistence port used by ProfileService. The local
// adapter keeps the existing v1 two-file storage contract; tests can inject an
// in-memory implementation without changing process-global environment state.
type ProfileRepository interface {
	Load() (*profile.Config, *profile.CredentialsFile, error)
	Upsert(profile.Domain, string, string, profile.Profile, bool) (bool, error)
	Remove(profile.Domain, string, string) error
	SetDefault(profile.Domain, string, string) error
	BindWorkspaceProfile(string, string, string, string, profile.Domain, string, string) error
	UnbindWorkspaceProfile(string, string, profile.Domain, string) error
	BindEnvironmentProfile(string, string, string, string, string, profile.Domain, string, string) error
	UnbindEnvironmentProfile(string, string, string, profile.Domain, string) error
	EnvironmentProfileBinding(string, string, string, profile.Domain, string) (string, error)
	Resolve(profile.ResolveInput) (*profile.Resolved, error)
	ConfigPath() (string, error)
	CredentialsPath() (string, error)
}

// LocalProfileRepository adapts the compatibility profile package to the
// application port. The profile package remains the owner of on-disk v1 JSON.
type LocalProfileRepository struct{}

func (LocalProfileRepository) Load() (*profile.Config, *profile.CredentialsFile, error) {
	return profile.Load()
}

func (LocalProfileRepository) Upsert(
	domain profile.Domain,
	backend, name string,
	value profile.Profile,
	setDefault bool,
) (bool, error) {
	return profile.Upsert(domain, backend, name, value, setDefault)
}

func (LocalProfileRepository) Remove(domain profile.Domain, backend, name string) error {
	return profile.Remove(domain, backend, name)
}

func (LocalProfileRepository) SetDefault(domain profile.Domain, backend, name string) error {
	return profile.SetDefault(domain, backend, name)
}

func (LocalProfileRepository) BindWorkspaceProfile(
	workspaceID, workspaceName, root, projectName string,
	domain profile.Domain,
	backend, name string,
) error {
	return profile.BindWorkspaceProfile(
		workspaceID, workspaceName, root, projectName, domain, backend, name,
	)
}

func (LocalProfileRepository) UnbindWorkspaceProfile(
	workspaceID, projectName string,
	domain profile.Domain,
	backend string,
) error {
	return profile.UnbindWorkspaceProfile(workspaceID, projectName, domain, backend)
}

func (LocalProfileRepository) BindEnvironmentProfile(
	workspaceID, workspaceName, root, projectName, environment string,
	domain profile.Domain,
	backend, name string,
) error {
	return profile.BindEnvironmentProfile(
		workspaceID, workspaceName, root, projectName, environment, domain, backend, name,
	)
}

func (LocalProfileRepository) UnbindEnvironmentProfile(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) error {
	return profile.UnbindEnvironmentProfile(root, projectName, environment, domain, backend)
}

func (LocalProfileRepository) EnvironmentProfileBinding(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) (string, error) {
	return profile.EnvironmentProfileBinding(root, projectName, environment, domain, backend)
}

func (LocalProfileRepository) Resolve(input profile.ResolveInput) (*profile.Resolved, error) {
	return profile.Resolve(input)
}

func (LocalProfileRepository) ConfigPath() (string, error) { return profile.ConfigPath() }

func (LocalProfileRepository) CredentialsPath() (string, error) {
	return profile.CredentialsPath()
}

// ProfileService is the single profile use-case boundary shared by Cobra and
// the local HTTP API.
type ProfileService struct {
	catalog    *catalog.Catalog
	repository ProfileRepository
}

func NewProfileService(
	backendCatalog *catalog.Catalog,
	repository ProfileRepository,
) (*ProfileService, error) {
	if backendCatalog == nil {
		return nil, errors.New("application: profile catalog is required")
	}
	if repository == nil {
		return nil, errors.New("application: profile repository is required")
	}
	if err := profile.ValidateCatalog(backendCatalog); err != nil {
		return nil, err
	}
	return &ProfileService{catalog: backendCatalog, repository: repository}, nil
}

func (s *ProfileService) Load() (*profile.Config, error) {
	config, _, err := s.repository.Load()
	return config, err
}

func (s *ProfileService) Paths() (configPath, credentialsPath string, err error) {
	configPath, err = s.repository.ConfigPath()
	if err != nil {
		return "", "", err
	}
	credentialsPath, err = s.repository.CredentialsPath()
	return configPath, credentialsPath, err
}

func (s *ProfileService) Lookup(domain profile.Domain, backend string) (catalog.BackendSpec, error) {
	spec, ok := s.catalog.Lookup(catalog.Domain(domain), backend)
	if !ok {
		return catalog.BackendSpec{}, cliErrors.New(
			cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("(%s, %s) 不是支持的 (domain, backend) 组合", domain, backend),
		).WithContext(map[string]any{"domain": string(domain), "backend": backend})
	}
	return spec, nil
}

func (s *ProfileService) ParsePair(pair string) (catalog.BackendSpec, error) {
	spec, ok := s.catalog.LookupPair(pair)
	if !ok {
		return catalog.BackendSpec{}, cliErrors.New(
			cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("未知 (domain, backend) pair %q；可选：%v。", pair, s.catalog.SortedPairs()),
		)
	}
	return spec, nil
}

// ProfileBackends preserves the catalog's product display order while
// excluding non-configurable implementation backends such as env/dotenv.
func (s *ProfileService) ProfileBackends() []catalog.BackendSpec {
	return s.catalog.ProfileBackends()
}

type ProfileSection struct {
	Spec    catalog.BackendSpec
	Payload any
	Names   []string
	Default string
}

func (s *ProfileService) Section(
	config *profile.Config,
	domain profile.Domain,
	backend string,
) (ProfileSection, error) {
	spec, err := s.Lookup(domain, backend)
	if err != nil {
		return ProfileSection{}, err
	}
	section, ok := profile.InspectSection(config, spec)
	if !ok {
		return ProfileSection{}, cliErrors.New(
			cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("profile schema 尚未实现 %s", spec.Pair),
		)
	}
	sort.Strings(section.Names)
	return ProfileSection{
		Spec: spec, Payload: section.Payload, Names: section.Names, Default: section.Default,
	}, nil
}

func (s *ProfileService) CredentialSource(
	config *profile.Config,
	domain profile.Domain,
	backend, name string,
) string {
	spec, err := s.Lookup(domain, backend)
	if err != nil {
		return ""
	}
	value, ok := profile.LookupStored(config, spec, name)
	if !ok {
		return ""
	}
	return profile.CredentialSource(spec, value)
}

func (s *ProfileService) DecodeProfile(
	domain profile.Domain,
	backend string,
	raw json.RawMessage,
) (profile.Profile, error) {
	spec, err := s.Lookup(domain, backend)
	if err != nil {
		return profile.Profile{}, err
	}
	return profile.Decode(spec, raw)
}

type UpsertProfileInput struct {
	Domain         profile.Domain
	Backend        string
	Name           string
	Profile        profile.Profile
	SetDefault     bool
	PreserveMasked bool
}

type UpsertProfileResult struct {
	Updated bool
	Default bool
}

func (s *ProfileService) Upsert(input UpsertProfileInput) (UpsertProfileResult, error) {
	spec, err := s.Lookup(input.Domain, input.Backend)
	if err != nil {
		return UpsertProfileResult{}, err
	}
	containsMasked, err := s.containsMaskedCredential(spec, input.Profile)
	if err != nil {
		return UpsertProfileResult{}, err
	}
	if input.PreserveMasked && containsMasked {
		config, err := s.Load()
		if err != nil {
			return UpsertProfileResult{}, err
		}
		if err := s.preserveMaskedCredentials(config, spec, input.Name, &input.Profile); err != nil {
			return UpsertProfileResult{}, err
		}
	}
	updated, err := s.repository.Upsert(
		input.Domain, input.Backend, input.Name, input.Profile, input.SetDefault,
	)
	if err != nil {
		return UpsertProfileResult{}, err
	}
	config, err := s.Load()
	if err != nil {
		return UpsertProfileResult{}, err
	}
	section, err := s.Section(config, input.Domain, input.Backend)
	if err != nil {
		return UpsertProfileResult{}, err
	}
	return UpsertProfileResult{Updated: updated, Default: section.Default == input.Name}, nil
}

func (s *ProfileService) Remove(domain profile.Domain, backend, name string) error {
	if _, err := s.Lookup(domain, backend); err != nil {
		return err
	}
	return s.repository.Remove(domain, backend, name)
}

func (s *ProfileService) SetDefault(domain profile.Domain, backend, name string) error {
	if _, err := s.Lookup(domain, backend); err != nil {
		return err
	}
	return s.repository.SetDefault(domain, backend, name)
}

func (s *ProfileService) BindWorkspaceProfile(
	workspaceID, workspaceName, root, projectName string,
	domain profile.Domain,
	backend, name string,
) error {
	if _, err := s.Lookup(domain, backend); err != nil {
		return err
	}
	return s.repository.BindWorkspaceProfile(
		workspaceID, workspaceName, root, projectName, domain, backend, name,
	)
}

func (s *ProfileService) UnbindWorkspaceProfile(
	workspaceID, projectName string,
	domain profile.Domain,
	backend string,
) error {
	if _, err := s.Lookup(domain, backend); err != nil {
		return err
	}
	return s.repository.UnbindWorkspaceProfile(workspaceID, projectName, domain, backend)
}

func (s *ProfileService) BindEnvironmentProfile(
	workspaceID, workspaceName, root, projectName, environment string,
	domain profile.Domain,
	backend, name string,
) error {
	if _, err := s.Lookup(domain, backend); err != nil {
		return err
	}
	return s.repository.BindEnvironmentProfile(
		workspaceID, workspaceName, root, projectName, environment, domain, backend, name,
	)
}

func (s *ProfileService) UnbindEnvironmentProfile(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) error {
	if _, err := s.Lookup(domain, backend); err != nil {
		return err
	}
	return s.repository.UnbindEnvironmentProfile(root, projectName, environment, domain, backend)
}

func (s *ProfileService) EnvironmentProfileBinding(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
) (string, error) {
	if _, err := s.Lookup(domain, backend); err != nil {
		return "", err
	}
	return s.repository.EnvironmentProfileBinding(root, projectName, environment, domain, backend)
}

func (s *ProfileService) Resolve(input profile.ResolveInput) (*profile.Resolved, error) {
	if _, err := s.Lookup(input.Domain, input.Backend); err != nil {
		return nil, err
	}
	return s.repository.Resolve(input)
}

func (s *ProfileService) HasCredentialFields(domain profile.Domain, backend string) bool {
	spec, err := s.Lookup(domain, backend)
	if err != nil {
		return false
	}
	for _, field := range spec.Profile.Fields {
		if field.Type == catalog.FieldSecret {
			return true
		}
	}
	return false
}
