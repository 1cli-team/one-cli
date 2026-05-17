package infisical

// run_profile.go is the single source of Infisical credentials for
// every internal caller (run-loader, init, verbs). Profiles are the
// only credential source — env vars (INFISICAL_UNIVERSAL_AUTH_*) were
// retired because two sources meant "which one wins?" confusion.
//
// Resolution chain (handled by profile.Resolve):
//
//   1. --profile <name>              one-shot flag override
//   2. workspace binding             ~/.config/one/config.json#workspaces
//   3. machine default               ~/.config/one/config.json#env/infisical.default

import (
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// resolveProfileCreds returns the resolved env profile's siteUrl + creds
// + the resolved profile name. Returns ("", nil, "", nil) — no error —
// when no profile is configured anywhere; caller decides whether the
// absence is fatal. Errors are surfaced only for "name was specified
// somewhere but doesn't exist" / corrupted-file conditions.
//
// profileFlag is the value of --profile (one-shot override); pass ""
// when the caller has no such flag.
func resolveProfileCreds(projectRoot, profileFlag string) (profileName string, creds *Credentials, siteURL string, err error) {
	workspaceID := ""
	if m, mErr := workspace.ReadManifest(projectRoot); mErr == nil {
		workspaceID = workspace.WorkspaceID(m)
	}
	resolved, rErr := profile.Resolve(profile.ResolveInput{
		Domain:       profile.DomainEnv,
		Backend:      "infisical",
		FlagOverride: profileFlag,
		WorkspaceID:  workspaceID,
	})
	if rErr != nil {
		// "no profile configured anywhere" is the cheap-path expected case
		// — translate to ("", nil, "", nil) so callers don't need to type-check.
		if cliErr, ok := rErr.(interface{ ErrorCode() string }); ok &&
			cliErr.ErrorCode() == "PROFILE_NONE_CONFIGURED" {
			return "", nil, "", nil
		}
		return "", nil, "", rErr
	}
	if resolved.Profile.Backend != "infisical" || resolved.Profile.Infisical == nil ||
		resolved.Profile.Infisical.Credentials == nil {
		// Resolved profile isn't infisical or has no creds — nothing for us.
		return "", nil, "", nil
	}
	ip := resolved.Profile.Infisical
	if strings.TrimSpace(ip.Credentials.ClientID) == "" ||
		strings.TrimSpace(ip.Credentials.ClientSecret) == "" {
		return "", nil, "", nil
	}
	return resolved.Name, &Credentials{
		ClientID:     ip.Credentials.ClientID,
		ClientSecret: ip.Credentials.ClientSecret,
	}, ip.SiteURL, nil
}

// runCredsAvailable reports whether `one run` can authenticate to
// Infisical for projectRoot — i.e. there's a default env profile that
// supplies a credential pair. Cheap probe: no network.
func runCredsAvailable(projectRoot string) bool {
	_, creds, _, _ := resolveProfileCreds(projectRoot, "")
	return creds != nil
}

// requireProfileCreds is the must-have variant: every caller that
// needs to talk to Infisical (init, run, verbs) goes through here.
// Returns INFISICAL_AUTH_MISSING with an actionable remediation when
// no profile is configured / the resolved profile lacks credentials.
func requireProfileCreds(projectRoot, profileFlag string) (string, *Credentials, string, error) {
	name, creds, siteURL, err := resolveProfileCreds(projectRoot, profileFlag)
	if err != nil {
		return "", nil, "", err
	}
	if creds == nil {
		return "", nil, "", cliErrors.New(cliErrors.INFISICAL_AUTH_MISSING,
			"未找到可用的 env profile 凭据。先 `one configure add env/infisical --profile <name> --client-id ... --client-secret ... --use`，或者 `one configure use env/infisical --profile <name>` 切到一个已配置的 profile。")
	}
	if siteURL == "" {
		siteURL = DefaultSiteURL
	}
	return name, creds, siteURL, nil
}
