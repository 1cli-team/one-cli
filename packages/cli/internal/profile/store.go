package profile

// store.go is the on-disk I/O for the two-file profile layout:
//
//	~/.config/one/config.json       — non-sensitive (mode 0600)
//	~/.config/one/credentials.json  — secrets only  (mode 0600)
//	~/.config/one/cache/...         — short-lived tokens (per-file 0600)
//
// Two responsibilities split out so callers can mock the path in tests:
//
//   - ConfigPath / CredentialsPath / CacheDir / CachePath — where files live
//   - Load / Save — read / write with mode 0600 and parent dir mkdirs
//
// Either file may be missing on first run — Load returns empty objects
// for the missing side rather than erroring. Save creates the parent
// directory if needed (mode 0700) and writes both files atomically via
// temp-file + rename.
//
// Missing files are not an error: the loader returns empty Config /
// CredentialsFile values so first-run `one configure add` writes a
// fresh pair without any read-modify-write dance.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// marshalConfig is the body of Config.MarshalJSON. Builds the JSON
// object key-by-key so empty (domain/backend) sections drop out —
// encoding/json's `omitempty` on a non-pointer struct field would
// always emit the empty section.
func marshalConfig(c Config) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	emit := func(first *bool, key string, raw []byte) {
		if !*first {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(key)
		buf.WriteString(`":`)
		buf.Write(raw)
		*first = false
	}

	versionRaw, err := json.Marshal(c.Version)
	if err != nil {
		return nil, err
	}
	first := true
	emit(&first, "version", versionRaw)

	if len(c.Workspaces) > 0 {
		raw, err := json.Marshal(c.Workspaces)
		if err != nil {
			return nil, err
		}
		emit(&first, "workspaces", raw)
	}

	emitSection := func(key string, empty bool, value any) error {
		if empty {
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		emit(&first, key, raw)
		return nil
	}
	if err := emitSection("env/infisical", c.EnvInfisical.IsEmpty(), c.EnvInfisical); err != nil {
		return nil, err
	}
	if err := emitSection("env/dotenv", c.EnvDotenv.IsEmpty(), c.EnvDotenv); err != nil {
		return nil, err
	}
	for _, kind := range S3CompatKinds() {
		sec := c.S3CompatSection(kind)
		if err := emitSection("deploy/"+kind, sec.IsEmpty(), *sec); err != nil {
			return nil, err
		}
	}
	if err := emitSection("deploy/kustomize", c.DeployKustomize.IsEmpty(), c.DeployKustomize); err != nil {
		return nil, err
	}
	if err := emitSection("deploy/vercel", c.DeployVercel.IsEmpty(), c.DeployVercel); err != nil {
		return nil, err
	}
	if err := emitSection("deploy/cloudflare", c.DeployCloudflare.IsEmpty(), c.DeployCloudflare); err != nil {
		return nil, err
	}
	if err := emitSection("deploy/edgeone", c.DeployEdgeOne.IsEmpty(), c.DeployEdgeOne); err != nil {
		return nil, err
	}
	for _, kind := range ContainerKinds() {
		sec := c.ContainerKindSection(kind)
		if err := emitSection("container/"+kind, sec.IsEmpty(), *sec); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// marshalCredentialsFile is the credentials.json sibling of
// marshalConfig. Same trick — empty sections drop out so a fresh
// credentials.json is just `{"version":1}`.
func marshalCredentialsFile(c CredentialsFile) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	emit := func(first *bool, key string, raw []byte) {
		if !*first {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(key)
		buf.WriteString(`":`)
		buf.Write(raw)
		*first = false
	}
	versionRaw, err := json.Marshal(c.Version)
	if err != nil {
		return nil, err
	}
	first := true
	emit(&first, "version", versionRaw)

	emitSection := func(key string, empty bool, value any) error {
		if empty {
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		emit(&first, key, raw)
		return nil
	}
	if err := emitSection("env/infisical", c.EnvInfisical.IsEmpty(), c.EnvInfisical); err != nil {
		return nil, err
	}
	for _, kind := range S3CompatKinds() {
		sec := c.S3CompatCredSection(kind)
		if err := emitSection("deploy/"+kind, sec.IsEmpty(), *sec); err != nil {
			return nil, err
		}
	}
	if err := emitSection("deploy/vercel", c.DeployVercel.IsEmpty(), c.DeployVercel); err != nil {
		return nil, err
	}
	if err := emitSection("deploy/cloudflare", c.DeployCloudflare.IsEmpty(), c.DeployCloudflare); err != nil {
		return nil, err
	}
	if err := emitSection("deploy/edgeone", c.DeployEdgeOne.IsEmpty(), c.DeployEdgeOne); err != nil {
		return nil, err
	}
	for _, kind := range ContainerKinds() {
		sec := c.ContainerKindCredSection(kind)
		if err := emitSection("container/"+kind, sec.IsEmpty(), *sec); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// configRoot returns ~/.config/one (XDG-aware on Linux). Used as the
// parent directory for config.json / credentials.json / cache/.
func configRoot() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "one"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "one"), nil
}

// ConfigPath returns the absolute path of config.json (~/.config/one/config.json).
func ConfigPath() (string, error) {
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.json"), nil
}

// CredentialsPath returns the absolute path of credentials.json
// (~/.config/one/credentials.json).
func CredentialsPath() (string, error) {
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "credentials.json"), nil
}

// CacheDir returns the absolute path of the token-cache root
// (~/.config/one/cache).
func CacheDir() (string, error) {
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cache"), nil
}

// Load reads config.json + credentials.json and merges them into an
// in-memory Config (with each profile's `Credentials *T` populated
// from credentials.json when credentialSource is "file"). Either file
// may be absent — empty Config / CredentialsFile values are returned.
//
// Returned Config / CredentialsFile are never nil.
func Load() (*Config, *CredentialsFile, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return nil, nil, err
	}
	credPath, err := CredentialsPath()
	if err != nil {
		return nil, nil, err
	}
	return LoadAt(cfgPath, credPath)
}

// LoadAt is the testable variant that takes explicit paths.
func LoadAt(cfgPath, credPath string) (*Config, *CredentialsFile, error) {
	cfg, _, err := loadConfigAt(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	creds, _, err := loadCredentialsAt(credPath)
	if err != nil {
		return nil, nil, err
	}
	mergeCredentials(cfg, creds)
	return cfg, creds, nil
}

func loadConfigAt(path string) (*Config, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{Version: SchemaVersion}, true, nil
		}
		return nil, false, err
	}
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, false, cliErrors.New(cliErrors.PROFILE_FILE_INVALID,
			"~/.config/one/config.json 解析失败："+err.Error()).
			WithContext(map[string]any{"path": path})
	}
	if probe.Version < MinSupportedVersion || probe.Version > SchemaVersion {
		return nil, false, cliErrors.New(cliErrors.PROFILE_VERSION_UNSUPPORTED,
			fmt.Sprintf("config.json schema version 不支持：要求 v%d-v%d，当前 v%d", MinSupportedVersion, SchemaVersion, probe.Version)).
			WithContext(map[string]any{
				"path":    path,
				"version": probe.Version,
			})
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, false, cliErrors.New(cliErrors.PROFILE_FILE_INVALID,
			"~/.config/one/config.json 解析失败："+err.Error()).
			WithContext(map[string]any{"path": path})
	}
	return &cfg, false, nil
}

func loadCredentialsAt(path string) (*CredentialsFile, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &CredentialsFile{Version: SchemaVersion}, true, nil
		}
		return nil, false, err
	}
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, false, cliErrors.New(cliErrors.PROFILE_FILE_INVALID,
			"~/.config/one/credentials.json 解析失败："+err.Error()).
			WithContext(map[string]any{"path": path})
	}
	if probe.Version < MinSupportedVersion || probe.Version > SchemaVersion {
		return nil, false, cliErrors.New(cliErrors.PROFILE_VERSION_UNSUPPORTED,
			fmt.Sprintf("credentials.json schema version 不支持：要求 v%d-v%d，当前 v%d", MinSupportedVersion, SchemaVersion, probe.Version)).
			WithContext(map[string]any{
				"path":    path,
				"version": probe.Version,
			})
	}
	var creds CredentialsFile
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil, false, cliErrors.New(cliErrors.PROFILE_FILE_INVALID,
			"~/.config/one/credentials.json 解析失败："+err.Error()).
			WithContext(map[string]any{"path": path})
	}
	return &creds, false, nil
}

// mergeCredentials inlines secrets from creds into cfg's profile
// structs. Only profiles whose CredentialSource is empty / "file" get
// merged — other source values are left untouched (resolver will
// surface PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED when consumers need
// the credentials).
func mergeCredentials(cfg *Config, creds *CredentialsFile) {
	for name, p := range cfg.EnvInfisical.Profiles {
		if !IsFileSource(p.CredentialSource) {
			continue
		}
		if cred, ok := creds.EnvInfisical.Profiles[name]; ok {
			c := cred
			p.Credentials = &c
			cfg.EnvInfisical.Profiles[name] = p
		}
	}
	for _, kind := range S3CompatKinds() {
		cfgSec := cfg.S3CompatSection(kind)
		credSec := creds.S3CompatCredSection(kind)
		for name, p := range cfgSec.Profiles {
			if !IsFileSource(p.CredentialSource) {
				continue
			}
			if cred, ok := credSec.Profiles[name]; ok {
				c := cred
				p.Credentials = &c
				cfgSec.Profiles[name] = p
			}
		}
	}
	for name, p := range cfg.DeployVercel.Profiles {
		if !IsFileSource(p.CredentialSource) {
			continue
		}
		if cred, ok := creds.DeployVercel.Profiles[name]; ok {
			c := cred
			p.Credentials = &c
			cfg.DeployVercel.Profiles[name] = p
		}
	}
	for name, p := range cfg.DeployCloudflare.Profiles {
		if !IsFileSource(p.CredentialSource) {
			continue
		}
		if cred, ok := creds.DeployCloudflare.Profiles[name]; ok {
			c := cred
			p.Credentials = &c
			cfg.DeployCloudflare.Profiles[name] = p
		}
	}
	for name, p := range cfg.DeployEdgeOne.Profiles {
		if !IsFileSource(p.CredentialSource) {
			continue
		}
		if cred, ok := creds.DeployEdgeOne.Profiles[name]; ok {
			c := cred
			p.Credentials = &c
			cfg.DeployEdgeOne.Profiles[name] = p
		}
	}
	for _, kind := range ContainerKinds() {
		cfgSec := cfg.ContainerKindSection(kind)
		credSec := creds.ContainerKindCredSection(kind)
		for name, p := range cfgSec.Profiles {
			if !IsFileSource(p.CredentialSource) {
				continue
			}
			if cred, ok := credSec.Profiles[name]; ok {
				c := cred
				p.Credentials = &c
				cfgSec.Profiles[name] = p
			}
		}
	}
}

// extractCredentials is the inverse of mergeCredentials: it splits
// secrets out of cfg's in-memory profiles into a CredentialsFile to
// persist alongside config.json. Only file-sourced profiles
// contribute secrets — others are written as-is to config.json
// without any matching entry in credentials.json.
func extractCredentials(cfg *Config) *CredentialsFile {
	creds := &CredentialsFile{Version: SchemaVersion}
	for name, p := range cfg.EnvInfisical.Profiles {
		if !IsFileSource(p.CredentialSource) || p.Credentials == nil {
			continue
		}
		if creds.EnvInfisical.Profiles == nil {
			creds.EnvInfisical.Profiles = map[string]InfisicalCredentials{}
		}
		creds.EnvInfisical.Profiles[name] = *p.Credentials
	}
	for _, kind := range S3CompatKinds() {
		cfgSec := cfg.S3CompatSection(kind)
		credSec := creds.S3CompatCredSection(kind)
		for name, p := range cfgSec.Profiles {
			if !IsFileSource(p.CredentialSource) || p.Credentials == nil {
				continue
			}
			if credSec.Profiles == nil {
				credSec.Profiles = map[string]S3Credentials{}
			}
			credSec.Profiles[name] = *p.Credentials
		}
	}
	for name, p := range cfg.DeployVercel.Profiles {
		if !IsFileSource(p.CredentialSource) || p.Credentials == nil {
			continue
		}
		if creds.DeployVercel.Profiles == nil {
			creds.DeployVercel.Profiles = map[string]VercelCredentials{}
		}
		creds.DeployVercel.Profiles[name] = *p.Credentials
	}
	for name, p := range cfg.DeployCloudflare.Profiles {
		if !IsFileSource(p.CredentialSource) || p.Credentials == nil {
			continue
		}
		if creds.DeployCloudflare.Profiles == nil {
			creds.DeployCloudflare.Profiles = map[string]CloudflareCredentials{}
		}
		creds.DeployCloudflare.Profiles[name] = *p.Credentials
	}
	for name, p := range cfg.DeployEdgeOne.Profiles {
		if !IsFileSource(p.CredentialSource) || p.Credentials == nil {
			continue
		}
		if creds.DeployEdgeOne.Profiles == nil {
			creds.DeployEdgeOne.Profiles = map[string]EdgeOneCredentials{}
		}
		creds.DeployEdgeOne.Profiles[name] = *p.Credentials
	}
	for _, kind := range ContainerKinds() {
		cfgSec := cfg.ContainerKindSection(kind)
		credSec := creds.ContainerKindCredSection(kind)
		for name, p := range cfgSec.Profiles {
			if !IsFileSource(p.CredentialSource) || p.Credentials == nil {
				continue
			}
			if credSec.Profiles == nil {
				credSec.Profiles = map[string]ContainerCredentials{}
			}
			credSec.Profiles[name] = *p.Credentials
		}
	}
	return creds
}

// Save writes config.json + credentials.json with mode 0600, both
// atomically via temp-file + rename. Credentials are extracted from
// cfg's in-memory profile structs (see extractCredentials), so callers
// only need to mutate the Config and call Save — they don't have to
// keep the two objects in sync manually.
//
// Creates the parent directory if needed (mode 0700, same user-only
// rationale).
//
// Both files are always written, even when one side is empty: writing
// an empty `{"version":1}` skeleton to credentials.json keeps Load
// from treating "no credentials.json" as a sign of a fresh-machine
// state vs an intentional all-non-file-source setup.
func Save(cfg *Config) error {
	cfgPath, err := ConfigPath()
	if err != nil {
		return err
	}
	credPath, err := CredentialsPath()
	if err != nil {
		return err
	}
	return SaveAt(cfg, cfgPath, credPath)
}

// SaveAt is the testable variant.
func SaveAt(cfg *Config, cfgPath, credPath string) error {
	if cfg == nil {
		return errors.New("profile: nil config")
	}
	cfg.Version = SchemaVersion
	creds := extractCredentials(cfg)

	cfgDir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return err
	}
	credDir := filepath.Dir(credPath)
	if cfgDir != credDir {
		if err := os.MkdirAll(credDir, 0o700); err != nil {
			return err
		}
	}

	cfgForFile := configForDisk(cfg)
	if err := atomicWrite(&cfgForFile, cfgPath); err != nil {
		return err
	}
	if err := atomicWrite(creds, credPath); err != nil {
		return fmt.Errorf("profile: config.json saved but credentials.json write failed: %w", err)
	}
	return nil
}

// configForDisk returns a copy of cfg with every profile's
// Credentials field cleared. The result is what gets serialized into
// config.json — keeping inline secrets out of the non-sensitive file
// even if some caller forgot to extractCredentials first.
func configForDisk(cfg *Config) Config {
	out := *cfg
	out.EnvInfisical.Profiles = stripInfisicalCreds(out.EnvInfisical.Profiles)
	for _, kind := range S3CompatKinds() {
		sec := out.S3CompatSection(kind)
		sec.Profiles = stripS3Creds(sec.Profiles)
	}
	out.DeployVercel.Profiles = stripVercelCreds(out.DeployVercel.Profiles)
	out.DeployCloudflare.Profiles = stripCloudflareCreds(out.DeployCloudflare.Profiles)
	out.DeployEdgeOne.Profiles = stripEdgeOneCreds(out.DeployEdgeOne.Profiles)
	for _, kind := range ContainerKinds() {
		sec := out.ContainerKindSection(kind)
		sec.Profiles = stripContainerCreds(sec.Profiles)
	}
	return out
}

func stripInfisicalCreds(in map[string]InfisicalProfile) map[string]InfisicalProfile {
	if in == nil {
		return nil
	}
	out := make(map[string]InfisicalProfile, len(in))
	for k, v := range in {
		v.Credentials = nil
		out[k] = v
	}
	return out
}

func stripS3Creds(in map[string]S3Profile) map[string]S3Profile {
	if in == nil {
		return nil
	}
	out := make(map[string]S3Profile, len(in))
	for k, v := range in {
		v.Credentials = nil
		out[k] = v
	}
	return out
}

func stripVercelCreds(in map[string]VercelProfile) map[string]VercelProfile {
	if in == nil {
		return nil
	}
	out := make(map[string]VercelProfile, len(in))
	for k, v := range in {
		v.Credentials = nil
		out[k] = v
	}
	return out
}

func stripCloudflareCreds(in map[string]CloudflareProfile) map[string]CloudflareProfile {
	if in == nil {
		return nil
	}
	out := make(map[string]CloudflareProfile, len(in))
	for k, v := range in {
		v.Credentials = nil
		out[k] = v
	}
	return out
}

func stripEdgeOneCreds(in map[string]EdgeOneProfile) map[string]EdgeOneProfile {
	if in == nil {
		return nil
	}
	out := make(map[string]EdgeOneProfile, len(in))
	for k, v := range in {
		v.Credentials = nil
		out[k] = v
	}
	return out
}

func stripContainerCreds(in map[string]ContainerProfile) map[string]ContainerProfile {
	if in == nil {
		return nil
	}
	out := make(map[string]ContainerProfile, len(in))
	for k, v := range in {
		v.Credentials = nil
		out[k] = v
	}
	return out
}

// atomicWrite marshals v as pretty JSON, writes to a sibling temp
// file with mode 0600, and renames into place.
func atomicWrite(v any, path string) error {
	dir := filepath.Dir(path)
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".profile-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after successful rename
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
