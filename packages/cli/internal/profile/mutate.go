package profile

// mutate.go — `profile add / remove / use` operations on the on-disk
// config. Each function loads, mutates, and saves; concurrent CLI
// invocations against the same machine config can race, but the file
// is single-user per design and the worst case is one of two
// near-simultaneous edits losing — same as kubectl / aws.
//
// The storage split per (domain, backend) plus the file/credentials
// physical split: every mutator takes a backend dimension alongside
// the domain. The Profile composite is destructured at the boundary
// into the typed sub-profile that belongs in the matching Section,
// and Save handles splitting Credentials out into credentials.json.

import (
	"fmt"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// Add inserts a new profile under (domain, backend). Returns
// PROFILE_ALREADY_EXISTS when name is taken at that section — callers
// should route that to the user as "re-run add to update credentials"
// rather than silently overwriting.
//
// setDefault is honored only when the section currently has no default
// pointer (the first profile added becomes default automatically) OR
// the caller explicitly passes true. This matches the kubectl `--use`
// flag pattern.
//
// The Profile.Backend field is required and must match `backend`; the
// profile struct must carry the matching typed sub-profile (Infisical
// for "infisical", S3 for any S3-compatible deploy backend, etc.) —
// checked by writeProfile.
func Add(domain Domain, backend, name string, profile Profile, setDefault bool) error {
	if name == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"profile 名不能为空。")
	}
	if err := validateBackend(domain, backend); err != nil {
		return err
	}
	if profile.Backend != "" && profile.Backend != backend {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("profile.Backend = %q 与目标 backend %q 不匹配", profile.Backend, backend))
	}
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	if exists, _ := profileExists(cfg, domain, backend, name); exists {
		return cliErrors.New(cliErrors.PROFILE_ALREADY_EXISTS,
			fmt.Sprintf("profile %q 已存在于 %s；要更新凭据请用 `one configure %s/%s add %s`。",
				name, SectionKey(domain, backend), domain, backend, name)).
			WithContext(map[string]any{
				"section": SectionKey(domain, backend),
				"name":    name,
			})
	}
	if err := writeProfile(cfg, domain, backend, name, profile, setDefault); err != nil {
		return err
	}
	return Save(cfg)
}

// Upsert inserts or replaces a profile under (domain, backend).
// Unlike Add it silently overwrites an existing profile of the same
// name — this is the "configure once, re-run to update credentials"
// semantic used by `one configure add <domain>/<backend>`. Returns
// updated=true when an existing profile was replaced, false when a
// fresh entry was created.
//
// setDefault honours the same "first profile becomes default
// automatically" rule as Add: explicit true forces default, otherwise
// default flips only when the section has no default profile yet.
func Upsert(domain Domain, backend, name string, profile Profile, setDefault bool) (updated bool, err error) {
	if name == "" {
		return false, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"profile 名不能为空。")
	}
	if err := validateBackend(domain, backend); err != nil {
		return false, err
	}
	if profile.Backend != "" && profile.Backend != backend {
		return false, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("profile.Backend = %q 与目标 backend %q 不匹配", profile.Backend, backend))
	}
	cfg, _, err := Load()
	if err != nil {
		return false, err
	}
	existed, _ := profileExists(cfg, domain, backend, name)
	if err := writeProfile(cfg, domain, backend, name, profile, setDefault); err != nil {
		return false, err
	}
	if err := Save(cfg); err != nil {
		return false, err
	}
	// Re-saving an existing profile invalidates whatever short-lived
	// token we cached for it (creds may have rotated).
	if existed {
		_ = ClearCache(domain, backend, name)
	}
	return existed, nil
}

// Remove deletes a profile from a (domain, backend) section. When
// backend is empty, the function searches across every backend in the
// domain and disambiguates: a unique match is removed; multiple
// matches return PROFILE_BACKEND_INVALID with the list of candidate
// backends so the caller can re-run with `--backend <b>`. If the
// removed profile was default for its section, default is reset to ""
// (caller can show "no default profile; pick one with `profile use`");
// we deliberately don't auto-pick a new default to avoid surprising
// the user.
func Remove(domain Domain, backend, name string) error {
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	resolvedBackend, err := resolveBackendFromName(cfg, domain, backend, name)
	if err != nil {
		return err
	}
	switch {
	case domain == DomainEnv && resolvedBackend == "infisical":
		delete(cfg.EnvInfisical.Profiles, name)
		if cfg.EnvInfisical.Default == name {
			cfg.EnvInfisical.Default = ""
		}
	case domain == DomainEnv && resolvedBackend == "dotenv":
		delete(cfg.EnvDotenv.Profiles, name)
		if cfg.EnvDotenv.Default == name {
			cfg.EnvDotenv.Default = ""
		}
	case domain == DomainDeploy && IsS3Compatible(resolvedBackend):
		sec := cfg.S3CompatSection(resolvedBackend)
		delete(sec.Profiles, name)
		if sec.Default == name {
			sec.Default = ""
		}
	case domain == DomainDeploy && resolvedBackend == "kustomize":
		delete(cfg.DeployKustomize.Profiles, name)
		if cfg.DeployKustomize.Default == name {
			cfg.DeployKustomize.Default = ""
		}
	case domain == DomainDeploy && resolvedBackend == "vercel":
		delete(cfg.DeployVercel.Profiles, name)
		if cfg.DeployVercel.Default == name {
			cfg.DeployVercel.Default = ""
		}
	case domain == DomainDeploy && resolvedBackend == "cloudflare":
		delete(cfg.DeployCloudflare.Profiles, name)
		if cfg.DeployCloudflare.Default == name {
			cfg.DeployCloudflare.Default = ""
		}
	case domain == DomainDeploy && resolvedBackend == "edgeone":
		delete(cfg.DeployEdgeOne.Profiles, name)
		if cfg.DeployEdgeOne.Default == name {
			cfg.DeployEdgeOne.Default = ""
		}
	case domain == DomainContainer && IsContainerKind(resolvedBackend):
		sec := cfg.ContainerKindSection(resolvedBackend)
		delete(sec.Profiles, name)
		if sec.Default == name {
			sec.Default = ""
		}
	}
	if err := Save(cfg); err != nil {
		return err
	}
	// Best-effort cache cleanup — never block remove on cache errors.
	_ = ClearCache(domain, resolvedBackend, name)
	return nil
}

// SetDefault sets the default profile for a (domain, backend). When backend
// is empty, the function searches across every backend in the domain
// and disambiguates the same way Remove does. Returns PROFILE_NOT_FOUND
// when name doesn't exist in the resolved section.
func SetDefault(domain Domain, backend, name string) error {
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	resolvedBackend, err := resolveBackendFromName(cfg, domain, backend, name)
	if err != nil {
		return err
	}
	switch {
	case domain == DomainEnv && resolvedBackend == "infisical":
		cfg.EnvInfisical.Default = name
	case domain == DomainEnv && resolvedBackend == "dotenv":
		cfg.EnvDotenv.Default = name
	case domain == DomainDeploy && IsS3Compatible(resolvedBackend):
		cfg.S3CompatSection(resolvedBackend).Default = name
	case domain == DomainDeploy && resolvedBackend == "kustomize":
		cfg.DeployKustomize.Default = name
	case domain == DomainDeploy && resolvedBackend == "vercel":
		cfg.DeployVercel.Default = name
	case domain == DomainDeploy && resolvedBackend == "cloudflare":
		cfg.DeployCloudflare.Default = name
	case domain == DomainDeploy && resolvedBackend == "edgeone":
		cfg.DeployEdgeOne.Default = name
	case domain == DomainContainer && IsContainerKind(resolvedBackend):
		cfg.ContainerKindSection(resolvedBackend).Default = name
	}
	return Save(cfg)
}

// BindWorkspaceProfile records a machine-local profile choice for a
// workspace, optionally scoped to a single project. It does not mutate
// the section's default pointer; this is the per-workspace equivalent of
// `SetDefault` and is intentionally kept out of one.manifest.json.
func BindWorkspaceProfile(workspaceID, workspaceName, root, projectName string, domain Domain, backend, name string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"workspace id 不能为空；请确认 one.manifest.json#workspace.id 已设置。")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"profile 名不能为空。")
	}
	if err := validateBackend(domain, backend); err != nil {
		return err
	}
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	if exists, names := profileExists(cfg, domain, backend, name); !exists {
		return profileNotFound(SectionKey(domain, backend), name, "workspace", names)
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]WorkspaceConfig{}
	}
	ws := cfg.Workspaces[workspaceID]
	if workspaceName = strings.TrimSpace(workspaceName); workspaceName != "" {
		ws.Name = workspaceName
	}
	if root = strings.TrimSpace(root); root != "" {
		ws.Root = root
	}
	key := SectionKey(domain, backend)
	if projectName = strings.TrimSpace(projectName); projectName != "" {
		if ws.Projects == nil {
			ws.Projects = map[string]WorkspaceProjectConfig{}
		}
		project := ws.Projects[projectName]
		if project.Profiles == nil {
			project.Profiles = map[string]string{}
		}
		project.Profiles[key] = name
		ws.Projects[projectName] = project
	} else {
		if ws.Profiles == nil {
			ws.Profiles = map[string]string{}
		}
		ws.Profiles[key] = name
	}
	cfg.Workspaces[workspaceID] = ws
	return Save(cfg)
}

// resolveBackendFromName fills in `backend` when the caller didn't
// know which backend a profile name lives under. The new top-level
// `one configure <domain>/<backend> ...` tree always passes an explicit
// backend; this helper survives mainly for resolver callers that
// search by name across a domain. When backend is supplied, the
// function still validates that the profile exists in that section.
//
// When backend is empty, the function searches every backend in the
// domain. A unique match is returned. Multiple matches return
// PROFILE_BACKEND_INVALID listing the candidate backends; no match
// returns PROFILE_NOT_FOUND with the union of available names.
func resolveBackendFromName(cfg *Config, domain Domain, backend, name string) (string, error) {
	if name == "" {
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"profile 名不能为空。")
	}
	if backend != "" {
		if err := validateBackend(domain, backend); err != nil {
			return "", err
		}
		exists, names := profileExists(cfg, domain, backend, name)
		if !exists {
			return "", profileNotFound(SectionKey(domain, backend), name, "lookup", names)
		}
		return backend, nil
	}

	matches := []string{}
	allNames := []string{}
	for _, b := range BackendsForDomain(domain) {
		exists, names := profileExists(cfg, domain, b, name)
		allNames = append(allNames, names...)
		if exists {
			matches = append(matches, b)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", cliErrors.New(cliErrors.PROFILE_NOT_FOUND,
			fmt.Sprintf("%s 域没有名为 %q 的 profile。已配置：%v",
				domain, name, allNames)).
			WithContext(map[string]any{
				"domain":             string(domain),
				"requested":          name,
				"available_profiles": allNames,
			})
	default:
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("%q 在 %s 域多个 backend 下都存在 (%v)；用 `one configure %s/<backend> ...` 形式指定具体 backend",
				name, domain, matches, domain)).
			WithContext(map[string]any{
				"domain":            string(domain),
				"requested":         name,
				"matching_backends": matches,
			})
	}
}

// profileExists returns whether `name` is configured under (domain,
// backend) and the list of currently-configured profile names in that
// section (for diagnostic error messages).
func profileExists(cfg *Config, domain Domain, backend, name string) (bool, []string) {
	switch {
	case domain == DomainEnv && backend == "infisical":
		_, ok := cfg.EnvInfisical.Profiles[name]
		return ok, mapKeys(cfg.EnvInfisical.Profiles)
	case domain == DomainEnv && backend == "dotenv":
		_, ok := cfg.EnvDotenv.Profiles[name]
		return ok, mapKeys(cfg.EnvDotenv.Profiles)
	case domain == DomainDeploy && IsS3Compatible(backend):
		sec := cfg.S3CompatSection(backend)
		_, ok := sec.Profiles[name]
		return ok, mapKeys(sec.Profiles)
	case domain == DomainDeploy && backend == "kustomize":
		_, ok := cfg.DeployKustomize.Profiles[name]
		return ok, mapKeys(cfg.DeployKustomize.Profiles)
	case domain == DomainDeploy && backend == "vercel":
		_, ok := cfg.DeployVercel.Profiles[name]
		return ok, mapKeys(cfg.DeployVercel.Profiles)
	case domain == DomainDeploy && backend == "cloudflare":
		_, ok := cfg.DeployCloudflare.Profiles[name]
		return ok, mapKeys(cfg.DeployCloudflare.Profiles)
	case domain == DomainDeploy && backend == "edgeone":
		_, ok := cfg.DeployEdgeOne.Profiles[name]
		return ok, mapKeys(cfg.DeployEdgeOne.Profiles)
	case domain == DomainContainer && IsContainerKind(backend):
		sec := cfg.ContainerKindSection(backend)
		_, ok := sec.Profiles[name]
		return ok, mapKeys(sec.Profiles)
	}
	return false, nil
}

// writeProfile destructures a Profile into the typed sub-profile that
// belongs in the section keyed by (domain, backend), then writes it
// + (optionally) sets the section's default pointer.
func writeProfile(cfg *Config, domain Domain, backend, name string, profile Profile, setDefault bool) error {
	mismatch := func() error {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("profile 缺 %s 的 sub-profile 数据", backend))
	}
	switch {
	case domain == DomainEnv && backend == "infisical":
		if profile.Infisical == nil {
			return mismatch()
		}
		if cfg.EnvInfisical.Profiles == nil {
			cfg.EnvInfisical.Profiles = map[string]InfisicalProfile{}
		}
		cfg.EnvInfisical.Profiles[name] = *profile.Infisical
		if setDefault || cfg.EnvInfisical.Default == "" {
			cfg.EnvInfisical.Default = name
		}
	case domain == DomainEnv && backend == "dotenv":
		if cfg.EnvDotenv.Profiles == nil {
			cfg.EnvDotenv.Profiles = map[string]DotenvProfile{}
		}
		dp := DotenvProfile{}
		if profile.Dotenv != nil {
			dp = *profile.Dotenv
		}
		cfg.EnvDotenv.Profiles[name] = dp
		if setDefault || cfg.EnvDotenv.Default == "" {
			cfg.EnvDotenv.Default = name
		}
	case domain == DomainDeploy && IsS3Compatible(backend):
		if profile.S3 == nil {
			return mismatch()
		}
		sec := cfg.S3CompatSection(backend)
		if sec.Profiles == nil {
			sec.Profiles = map[string]S3Profile{}
		}
		sec.Profiles[name] = *profile.S3
		if setDefault || sec.Default == "" {
			sec.Default = name
		}
	case domain == DomainDeploy && backend == "kustomize":
		if profile.Kustomize == nil {
			return mismatch()
		}
		if cfg.DeployKustomize.Profiles == nil {
			cfg.DeployKustomize.Profiles = map[string]KustomizeProfile{}
		}
		cfg.DeployKustomize.Profiles[name] = *profile.Kustomize
		if setDefault || cfg.DeployKustomize.Default == "" {
			cfg.DeployKustomize.Default = name
		}
	case domain == DomainDeploy && backend == "vercel":
		if profile.Vercel == nil {
			return mismatch()
		}
		if cfg.DeployVercel.Profiles == nil {
			cfg.DeployVercel.Profiles = map[string]VercelProfile{}
		}
		cfg.DeployVercel.Profiles[name] = *profile.Vercel
		if setDefault || cfg.DeployVercel.Default == "" {
			cfg.DeployVercel.Default = name
		}
	case domain == DomainDeploy && backend == "cloudflare":
		if profile.Cloudflare == nil {
			return mismatch()
		}
		if cfg.DeployCloudflare.Profiles == nil {
			cfg.DeployCloudflare.Profiles = map[string]CloudflareProfile{}
		}
		cfg.DeployCloudflare.Profiles[name] = *profile.Cloudflare
		if setDefault || cfg.DeployCloudflare.Default == "" {
			cfg.DeployCloudflare.Default = name
		}
	case domain == DomainDeploy && backend == "edgeone":
		if profile.EdgeOne == nil {
			return mismatch()
		}
		if cfg.DeployEdgeOne.Profiles == nil {
			cfg.DeployEdgeOne.Profiles = map[string]EdgeOneProfile{}
		}
		cfg.DeployEdgeOne.Profiles[name] = *profile.EdgeOne
		if setDefault || cfg.DeployEdgeOne.Default == "" {
			cfg.DeployEdgeOne.Default = name
		}
	case domain == DomainContainer && IsContainerKind(backend):
		if profile.Container == nil {
			return mismatch()
		}
		sec := cfg.ContainerKindSection(backend)
		if sec.Profiles == nil {
			sec.Profiles = map[string]ContainerProfile{}
		}
		sec.Profiles[name] = *profile.Container
		if setDefault || sec.Default == "" {
			sec.Default = name
		}
	default:
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("(%s, %s) 不是支持的 (domain, backend) 组合", domain, backend))
	}
	return nil
}

// validateBackend checks that backend is a known backend for the
// declared domain. Catches typos early ("infisicaal") and
// cross-domain mistakes ("docker" attached to an env profile).
func validateBackend(domain Domain, backend string) error {
	if backend == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"profile 缺 backend 字段。")
	}
	known := BackendsForDomain(domain)
	for _, b := range known {
		if b == backend {
			return nil
		}
	}
	return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("backend %q 不属于 %s 域（合法值：%v）。",
			backend, domain, known)).
		WithContext(map[string]any{
			"backend":        backend,
			"profile_domain": string(domain),
		})
}
