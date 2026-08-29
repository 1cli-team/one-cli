package infisical

// auth.go is just the credential type now. Credentials are sourced
// exclusively through the machine-level profile system
// (~/.config/one/config.json + credentials.json) — see run_profile.go's
// resolveProfileCreds. The previous env-var path
// (INFISICAL_UNIVERSAL_AUTH_CLIENT_ID/_SECRET) was retired because:
//
//   - Two sources meant subtle "which one wins?" confusion. Users who
//     configured a profile then saw stale env vars from their dotfile
//     beat the explicit profile.
//   - Profiles already cover the CI use-case via the standard
//     precedence chain: `--profile` flag or manifest.domains.env.profile
//     selects which profile to use; the credentials live inside the
//     profile (which is per-user, mode 0600).
//
// CI migration: replace
//   export INFISICAL_UNIVERSAL_AUTH_CLIENT_ID=...
//   export INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET=...
// with one of:
//   one configure add env/infisical --profile ci \
//       --client-id $CID --client-secret $CSEC --use
// or pre-bake config.json + credentials.json into the runner image.

// Credentials is the canonical Universal Auth credential pair. Built
// from a resolved profile, never read from the environment.
type Credentials struct {
	ClientID     string
	ClientSecret string
}
