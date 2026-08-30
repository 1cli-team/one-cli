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

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
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
	if err := ValidateName(name); err != nil {
		return err
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
	if err := ValidateName(name); err != nil {
		return false, err
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
// the user. An environment-aware binding blocks deletion with PROFILE_IN_USE;
// callers must explicitly unbind it first so the independent binding store and
// Profile files never need a non-atomic cross-file cascade.
func Remove(domain Domain, backend, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	resolvedBackend, err := resolveBackendFromName(cfg, domain, backend, name)
	if err != nil {
		return err
	}
	policy, ok := schemaPolicy(domain, resolvedBackend)
	if !ok {
		return invalidProfilePair(domain, resolvedBackend)
	}
	err = withEnvironmentProfileBindingReferences(
		domain, resolvedBackend, name,
		func(references []environmentProfileBindingReference) error {
			if len(references) > 0 {
				return cliErrors.New(cliErrors.PROFILE_IN_USE,
					fmt.Sprintf("profile %q 仍被 %d 个环境绑定引用；请先在 Dashboard 中选择 Automatic 解绑。", name, len(references))).
					WithContext(map[string]any{
						"section":       SectionKey(domain, resolvedBackend),
						"name":          name,
						"binding_count": len(references),
						"bindings":      references,
					})
			}
			removeLegacyProfileBindings(cfg, domain, resolvedBackend, name)
			policy.remove(cfg, name)
			return Save(cfg)
		},
	)
	if err != nil {
		return err
	}
	// Best-effort cache cleanup — never block remove on cache errors.
	_ = ClearCache(domain, resolvedBackend, name)
	return nil
}

// removeLegacyProfileBindings drops the environment-agnostic Workspace and
// Project references from the same Config value that Remove saves with the
// Profile deletion. Workspace registration metadata (name/root) is retained.
func removeLegacyProfileBindings(cfg *Config, domain Domain, backend, name string) {
	if cfg == nil || len(cfg.Workspaces) == 0 {
		return
	}
	sectionKey := SectionKey(domain, backend)
	for workspaceID, workspace := range cfg.Workspaces {
		if workspace.Profiles[sectionKey] == name {
			delete(workspace.Profiles, sectionKey)
		}
		if len(workspace.Profiles) == 0 {
			workspace.Profiles = nil
		}
		for projectName, project := range workspace.Projects {
			if project.Profiles[sectionKey] == name {
				delete(project.Profiles, sectionKey)
			}
			if project.IsEmpty() {
				delete(workspace.Projects, projectName)
			} else {
				workspace.Projects[projectName] = project
			}
		}
		if len(workspace.Projects) == 0 {
			workspace.Projects = nil
		}
		if workspace.IsEmpty() {
			delete(cfg.Workspaces, workspaceID)
		} else {
			cfg.Workspaces[workspaceID] = workspace
		}
	}
	if len(cfg.Workspaces) == 0 {
		cfg.Workspaces = nil
	}
}

// SetDefault sets the default profile for a (domain, backend). When backend
// is empty, the function searches across every backend in the domain
// and disambiguates the same way Remove does. Returns PROFILE_NOT_FOUND
// when name doesn't exist in the resolved section.
func SetDefault(domain Domain, backend, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	resolvedBackend, err := resolveBackendFromName(cfg, domain, backend, name)
	if err != nil {
		return err
	}
	policy, ok := schemaPolicy(domain, resolvedBackend)
	if !ok {
		return invalidProfilePair(domain, resolvedBackend)
	}
	policy.setDefault(cfg, name)
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
	if err := ValidateName(name); err != nil {
		return err
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

// UnbindWorkspaceProfile removes one machine-local workspace or project
// profile choice. It is idempotent and only edits Config.Workspaces; profile
// definitions and credentials remain untouched. Empty projectName removes the
// workspace-level choice, while a non-empty projectName removes only that
// project's override so resolution falls back to workspace/default precedence.
func UnbindWorkspaceProfile(
	workspaceID, projectName string,
	domain Domain,
	backend string,
) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"workspace id 不能为空；请确认 one.manifest.json#workspace.id 已设置。")
	}
	if err := validateBackend(domain, backend); err != nil {
		return err
	}
	cfg, _, err := Load()
	if err != nil {
		return err
	}
	if cfg.Workspaces == nil {
		return nil
	}
	ws, ok := cfg.Workspaces[workspaceID]
	if !ok {
		return nil
	}
	key := SectionKey(domain, backend)
	changed := false
	if projectName = strings.TrimSpace(projectName); projectName != "" {
		project, exists := ws.Projects[projectName]
		if exists {
			if _, exists := project.Profiles[key]; exists {
				delete(project.Profiles, key)
				changed = true
			}
			if project.IsEmpty() {
				delete(ws.Projects, projectName)
			} else {
				ws.Projects[projectName] = project
			}
		}
	} else if _, exists := ws.Profiles[key]; exists {
		delete(ws.Profiles, key)
		changed = true
	}
	if !changed {
		return nil
	}
	if ws.IsEmpty() {
		delete(cfg.Workspaces, workspaceID)
	} else {
		cfg.Workspaces[workspaceID] = ws
	}
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
	if err := ValidateName(name); err != nil {
		return "", err
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
	policy, ok := schemaPolicy(domain, backend)
	if !ok {
		return false, nil
	}
	_, exists := policy.lookup(cfg, name)
	return exists, policy.names(cfg)
}

// writeProfile destructures a Profile into the typed sub-profile that
// belongs in the section keyed by (domain, backend), then writes it
// + (optionally) sets the section's default pointer.
func writeProfile(cfg *Config, domain Domain, backend, name string, profile Profile, setDefault bool) error {
	policy, ok := schemaPolicy(domain, backend)
	if !ok {
		return invalidProfilePair(domain, backend)
	}
	return policy.write(cfg, name, profile, setDefault)
}

// validateBackend checks that backend is a known backend for the
// declared domain. Catches typos early ("infisicaal") and
// cross-domain mistakes ("docker" attached to an env profile).
func validateBackend(domain Domain, backend string) error {
	if backend == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"profile 缺 backend 字段。")
	}
	if _, ok := schemaPolicy(domain, backend); ok {
		return nil
	}
	known := BackendsForDomain(domain)
	return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("backend %q 不属于 %s 域（合法值：%v）。",
			backend, domain, known)).
		WithContext(map[string]any{
			"backend":        backend,
			"profile_domain": string(domain),
		})
}
