package createcmd

import (
	"fmt"

	"path/filepath"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// resolveCreateEnables resolves the list of backend ids to enable when
// scaffolding a new workspace. The post-trim policy:
//
//  1. workspaceDefaultEnables baseline (env/dotenv + dev/process) — always
//     applied unless overridden by --env-provider. CI is intentionally not
//     selected by default. env defaults to dotenv because it's the
//     lowest-friction option for solo / OSS users.
//
//  2. --env-provider flag explicitly picks dotenv or infisical.
//     No interactive prompt at create time — users who don't pass the
//     flag get dotenv silently, and can switch later via `one env switch`.
//
//  3. deploy and container Domains are template-driven (registry.json
//     defaults applied at `one add` time). Not asked at create
//     time; not in workspaceDefaultEnables.
//
// Returned ids are namespaced (e.g. "env/dotenv"); per-Domain
// uniqueness is guaranteed by the time this returns.
func resolveCreateEnables(envProvider string, interactive bool) ([]string, error) {
	_ = interactive

	// Build a domain -> id map starting from the workspace defaults.
	byDomain := map[string]string{}
	for _, id := range workspaceDefaultEnables {
		byDomain[domainOf(id)] = id
	}

	// --env-provider accepts the bare backend name ("dotenv" / "infisical").
	// Empty → keep the dotenv default.
	switch strings.TrimSpace(envProvider) {
	case "":
		// keep default (env/dotenv)
	case "dotenv":
		byDomain["env"] = "env/dotenv"
	case "infisical":
		byDomain["env"] = "env/infisical"
	default:
		return nil, cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN,
			fmt.Sprintf("--env-provider 值无效: %q（合法值: dotenv / infisical）", envProvider))
	}

	// Flatten deterministically by canonical domain order.
	out := []string{}
	for _, d := range canonicalDomainOrder {
		if id, ok := byDomain[d]; ok && id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

// domainOf extracts the domain component of a namespaced id
// ("env/dotenv" -> "env"). Returns "" if the id has no slash.
func domainOf(id string) string {
	if i := strings.Index(id, "/"); i > 0 {
		return id[:i]
	}
	return ""
}

func selectedEnvironmentBackend(backends []string) string {
	for _, id := range backends {
		if domainOf(id) == "env" {
			return strings.TrimPrefix(id, "env/")
		}
	}
	return ""
}

// resolveTargetPath turns a user-supplied target ("." | "./foo" | absolute |
// relative) into an absolute path rooted at cwd. Used by both the pre-form
// validator and the post-form scaffold step so they always agree on what
// directory we're talking about.
func resolveTargetPath(cwd, raw string) string {
	switch raw {
	case ".", "./":
		return cwd
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(cwd, raw)
}
