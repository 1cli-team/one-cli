package serve

// mask.go enforces "credentials never leave the binary in plaintext unless
// the caller explicitly asked for them" on the HTTP boundary. Mirrors the
// shape used by `one configure ... show` (internal/cmd/configurecmd/cmd.go's
// maskCredentials), but operates on the section-typed structs the current schema
// stores directly — not on the resolver's discriminated-union Profile.
//
// Mask everything that's a *secret*. Identity-shaped fields (clientId,
// accessKeyId, registry username, kubeconfig path) stay visible because:
//
//   - the UI needs them to label and disambiguate profiles in the list view;
//   - they're already low-value to attackers without the matching secret;
//   - hiding them would force a reveal=1 round-trip just to render the list.
//
// Don't widen this set without thinking. Each visible field is a
// deliberate trade-off, not an oversight.

import "github.com/torchstellar-team/one-cli/packages/cli/internal/profile"

const masked = "********"

// maskInfisical returns a copy with the client secret redacted.
func maskInfisical(p profile.InfisicalProfile) profile.InfisicalProfile {
	if p.Credentials == nil {
		return p
	}
	creds := *p.Credentials
	creds.ClientSecret = masked
	p.Credentials = &creds
	return p
}

// maskS3 returns a copy with the access-key secret redacted. AccessKeyID
// is left visible so the list view can show "ak: AKID12345" without a
// reveal=1 fetch — it's the AKID + secret pair that's sensitive.
func maskS3(p profile.S3Profile) profile.S3Profile {
	if p.Credentials == nil {
		return p
	}
	creds := *p.Credentials
	creds.AccessKeySecret = masked
	p.Credentials = &creds
	return p
}

// maskContainer returns a copy with the registry password redacted. The
// username stays visible (it's the docker-login `--username` value, often
// just an org name or a robot account id).
func maskContainer(p profile.ContainerProfile) profile.ContainerProfile {
	if p.Credentials == nil {
		return p
	}
	creds := *p.Credentials
	creds.Password = masked
	p.Credentials = &creds
	return p
}

// maskKustomize is a no-op: kustomize profiles store only paths and
// context names, neither of which are secret. Defined for symmetry so
// callers don't have to special-case it.
func maskKustomize(p profile.KustomizeProfile) profile.KustomizeProfile { return p }

// maskVercel returns a copy with the API token redacted. Team is left
// visible (it's an org slug, not a secret).
func maskVercel(p profile.VercelProfile) profile.VercelProfile {
	if p.Credentials == nil {
		return p
	}
	creds := *p.Credentials
	creds.APIToken = masked
	p.Credentials = &creds
	return p
}

// maskDotenv is a no-op: the dotenv profile struct is empty. Same
// rationale as maskKustomize.
func maskDotenv(p profile.DotenvProfile) profile.DotenvProfile { return p }

// maskConfig walks every section and applies its per-backend mask.
// The whole-config GET handler uses this; per-section handlers can
// reach for the typed mask directly.
func maskConfig(c profile.Config) profile.Config {
	c.EnvInfisical.Profiles = maskMap(c.EnvInfisical.Profiles, maskInfisical)
	c.EnvDotenv.Profiles = maskMap(c.EnvDotenv.Profiles, maskDotenv)
	for _, kind := range profile.S3CompatKinds() {
		sec := c.S3CompatSection(kind)
		sec.Profiles = maskMap(sec.Profiles, maskS3)
	}
	c.DeployKustomize.Profiles = maskMap(c.DeployKustomize.Profiles, maskKustomize)
	c.DeployVercel.Profiles = maskMap(c.DeployVercel.Profiles, maskVercel)
	for _, kind := range profile.ContainerKinds() {
		sec := c.ContainerKindSection(kind)
		sec.Profiles = maskMap(sec.Profiles, maskContainer)
	}
	return c
}

// maskMap rebuilds m with each value passed through fn. Returns nil for nil
// input so the on-disk JSON's empty-section omission still kicks in.
func maskMap[T any](m map[string]T, fn func(T) T) map[string]T {
	if m == nil {
		return nil
	}
	out := make(map[string]T, len(m))
	for k, v := range m {
		out[k] = fn(v)
	}
	return out
}
