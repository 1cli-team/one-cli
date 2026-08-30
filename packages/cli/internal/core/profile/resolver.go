package profile

// resolver.go implements the per-call profile lookup chain. Every
// `one env <verb>` / `one deploy <verb>` / `one container <verb>`
// invocation runs Resolve to pick which profile applies, then hands
// the resolved Profile to the backend.
//
// The config schema stores each (domain, backend) in its own section
// with its own default pointer in config.json; secrets live in
// credentials.json. Load merges both so the in-memory Profile shape
// already has Credentials populated for file-source profiles —
// consumers continue to read `resolved.Profile.X.Credentials.Y`
// directly.
//
// Lookup precedence (first non-empty wins):
//
//   1. --profile <name> flag                           (one-shot, doesn't touch default)
//   2. profile-bindings.json#[canonicalRoot][environment].projects[projectName]
//   3. profile-bindings.json#[canonicalRoot][environment].profiles
//   4. config.json#workspaces[workspaceID].projects[projectName].profiles[domain/backend]
//   5. config.json#workspaces[workspaceID].profiles[domain/backend]
//   6. ~/.config/one/config.json#<domain>/<backend>.default (machine default)
//   7. (no profile) → PROFILE_NONE_CONFIGURED if the backend needs one.
//
// CredentialSource handling: only "file" / "" is wired up. Any other
// value surfaces PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED — an explicit
// "feature reserved" error rather than silent fallback to file.

import (
	"fmt"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// ResolveInput collects the inputs the resolver needs from various
// call sites without coupling profile/ to internal/bootstrap/cli or
// internal/core/workspace. Cobra fills FlagOverride; manifest read fills
// WorkspaceID / ProjectName; the rest read by the resolver itself.
//
// Backend is required: each (domain, backend) is a separate section,
// so the resolver always needs to know which backend's default
// pointer + profiles to walk. Callers know the backend at call site
// (envcmd → "infisical", containercmd → "docker", deploycmd → the
// subproject's declared backend).
type ResolveInput struct {
	Domain        Domain
	Backend       string
	FlagOverride  string // value of --profile flag, "" if unset
	WorkspaceID   string // manifest.workspace.id, "" if unavailable
	WorkspaceRoot string // workspace root; paired with Environment for local environment bindings
	ProjectName   string // manifest.projects[].name, "" for workspace scope
	Environment   string // safe environment id (for example dev / preview / prod / staging)
	SkipDefault   bool   // when true, only flag/workspace bindings are considered
}

// Resolved is the answer Resolve hands back. Name is the picked
// profile's id (useful for logging / output envelopes); Profile is
// the discriminated-union shape with only the matching backend field
// populated; Source describes which step in the precedence chain
// matched (for diagnostic output); CredSource records the resolved
// credentialSource ("file" / "env" / ...) so callers can render it.
type Resolved struct {
	Name       string
	Profile    Profile
	Source     string // "flag" / "workspace-project-environment" / "workspace-environment" / legacy / "default"
	CredSource string // "file" / "env" / "command:..." / "keyring"
}

// Resolve walks the precedence chain. Returns PROFILE_NONE_CONFIGURED
// when nothing matches; PROFILE_NOT_FOUND when a name was specified
// (flag / workspace binding / default) but no profile exists by that name;
// PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED when the resolved profile
// names a credentialSource this build cannot honour.
func Resolve(in ResolveInput) (*Resolved, error) {
	if in.Backend == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"resolve: backend 不能为空")
	}
	cfg, _, err := Load()
	if err != nil {
		return nil, err
	}

	policy, ok := schemaPolicy(in.Domain, in.Backend)
	if !ok {
		return nil, invalidProfilePair(in.Domain, in.Backend)
	}
	defaultName := policy.defaultName(cfg)
	names := policy.names(cfg)
	sectionKey := SectionKey(in.Domain, in.Backend)

	finalize := func(name string, source string) (*Resolved, error) {
		p, ok := policy.lookup(cfg, name)
		if !ok {
			return nil, profileNotFound(sectionKey, name, source, names)
		}
		credSource := policy.credentialSource(p)
		if !IsFileSource(credSource) {
			return nil, cliErrors.New(cliErrors.PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED,
				fmt.Sprintf("profile %q 的 credentialSource = %q 当前未实现（仅支持 \"file\"）", name, credSource)).
				WithContext(map[string]any{
					"section":          sectionKey,
					"profile":          name,
					"credentialSource": credSource,
				})
		}
		return &Resolved{Name: name, Profile: p, Source: source, CredSource: SourceFile}, nil
	}

	// 1. --profile flag
	if name := strings.TrimSpace(in.FlagOverride); name != "" {
		return finalize(name, "flag")
	}

	// 2-3. Environment-aware bindings are keyed by canonical checkout root.
	// WorkspaceRoot by itself is allowed for backwards-compatible callers;
	// Environment opts the call into the new binding store and therefore also
	// requires a root.
	if in.Environment != "" {
		if strings.TrimSpace(in.WorkspaceRoot) == "" {
			return nil, invalidBindingValue("workspace root", in.WorkspaceRoot)
		}
		projectBinding, workspaceBinding, err := environmentBindingNamesAt(
			in.WorkspaceRoot, in.Environment, in.ProjectName, sectionKey,
		)
		if err != nil {
			return nil, err
		}
		if projectBinding != "" {
			return finalize(projectBinding, "workspace-project-environment")
		}
		if workspaceBinding != "" {
			return finalize(workspaceBinding, "workspace-environment")
		}
	}

	// 4. legacy per-project workspace binding
	if name := workspaceProjectBinding(cfg, in.WorkspaceID, in.ProjectName, sectionKey); name != "" {
		return finalize(name, "workspace-project")
	}

	// 5. legacy workspace binding
	if name := workspaceBinding(cfg, in.WorkspaceID, sectionKey); name != "" {
		return finalize(name, "workspace")
	}

	// 6. machine default for this (domain, backend)
	if name := strings.TrimSpace(defaultName); name != "" && !in.SkipDefault {
		return finalize(name, "default")
	}

	// 7. nothing matched
	return nil, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
		fmt.Sprintf("没有配置 %s profile。先 `one configure %s/%s add <name>` 创建。",
			sectionKey, in.Domain, in.Backend)).
		WithContext(map[string]any{
			"domain":  string(in.Domain),
			"backend": in.Backend,
		})
}

func workspaceProjectBinding(cfg *Config, workspaceID, projectName, sectionKey string) string {
	ws := workspaceConfig(cfg, workspaceID)
	if ws == nil || strings.TrimSpace(projectName) == "" {
		return ""
	}
	project, ok := ws.Projects[strings.TrimSpace(projectName)]
	if !ok {
		return ""
	}
	return strings.TrimSpace(project.Profiles[sectionKey])
}

func workspaceBinding(cfg *Config, workspaceID, sectionKey string) string {
	ws := workspaceConfig(cfg, workspaceID)
	if ws == nil {
		return ""
	}
	return strings.TrimSpace(ws.Profiles[sectionKey])
}

func workspaceConfig(cfg *Config, workspaceID string) *WorkspaceConfig {
	if cfg == nil || len(cfg.Workspaces) == 0 {
		return nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil
	}
	ws, ok := cfg.Workspaces[workspaceID]
	if !ok {
		return nil
	}
	return &ws
}

func profileNotFound(sectionKey, name, source string, available []string) error {
	return cliErrors.New(cliErrors.PROFILE_NOT_FOUND,
		fmt.Sprintf("没有名为 %q 的 %s profile（来源：%s）。已配置：%v",
			name, sectionKey, source, available)).
		WithContext(map[string]any{
			"section":            sectionKey,
			"requested":          name,
			"source":             source,
			"available_profiles": available,
		})
}
