package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// ManifestFilename is the on-disk location of the workspace manifest at the
// project root.
const ManifestFilename = "one.manifest.json"

// ManifestVersion is the current manifest schema generation.
const ManifestVersion = 1

// Manifest is the parsed one.manifest.json document.
//
// Current layout:
//   - workspace: identity only (id, name)
//   - environments: top-level environment-name list + default name; consumed
//     by secrets backends, deploy --env validation, and per-project
//     deploy.config.env validation alike
//   - domains: workspace-level backend selections, keyed by domain name
//     ("env", "deploy", "container"). Each value carries kind +
//     a kind-specific config blob (json.RawMessage, decoded by callers via
//     typed accessors).
//   - projects[]: each project carries identity (name, relativeDir,
//     templateId, toolchain, buildVersion, packageManager) plus an optional
//     domains override block. Project-scope env / container override carry
//     no kind (always inherited from workspace); project-scope deploy
//     carries full kind+config because deploy is genuinely
//     per-project polymorphic.
type Manifest struct {
	Version      int                `json:"version"`
	Workspace    *ManifestWorkspace `json:"workspace,omitempty"`
	Environments *Environments      `json:"environments,omitempty"`
	Domains      *WorkspaceDomains  `json:"domains,omitempty"`
	Projects     []ManifestProject  `json:"projects"`
}

// ManifestWorkspace describes the workspace identity. The shared manifest no longer carries
// roots (apps/services/packages are hard-wired in roots.go) or the
// packageManager (which was never read at workspace scope; the field still
// exists per-project on ManifestProject).
type ManifestWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Environments is the workspace-level environment-name registry. Names are
// the deployment target names ("dev" / "staging" / "prod" by default), used
// in three independent places:
//   - secrets backends enumerate `Names` to know which env files / Infisical
//     environments exist
//   - `one deploy --env <name>` validates against `Names`
//   - projects[].domains.deploy.config.env validates against `Names`
//
// Default is the env name used when --env is omitted; it must appear in
// Names. New workspaces seed `["dev","staging","prod"]` with default "dev".
type Environments struct {
	Names   []string `json:"names,omitempty"`
	Default string   `json:"default,omitempty"`
}

// DefaultEnvironments is the canonical environment list stamped into a
// fresh manifest.environments.names. Same value is mirrored by the
// infisical package; defining it here keeps the workspace layer
// independent of any specific secrets backend.
var DefaultEnvironments = []string{"dev", "staging", "prod"}

// WorkspaceDomains is the workspace-level backend selection block. Each
// field is optional and represents the selected backend for that domain.
// Marshalled as {"env": {...}, "deploy": {...}, "container": {...}} so the
// JSON shape mirrors per-project ProjectDomains.
type WorkspaceDomains struct {
	Env       *BackendRef `json:"env,omitempty"`
	Deploy    *BackendRef `json:"deploy,omitempty"`
	Container *BackendRef `json:"container,omitempty"`
}

// BackendRef is the workspace-level "selected backend" for a single domain.
// `Kind` is the bare backend name (e.g. "infisical", "kustomize", "docker").
// `Config` is a kind-specific JSON blob; callers decode via typed accessors per domain
// (see internal/workspace/domains/{env,deploy,container}.go).
type BackendRef struct {
	Kind   string          `json:"kind,omitempty"`
	Config json.RawMessage `json:"config,omitempty"`
}

// ManifestProject is one project entry in manifest.projects[]. Identity
// fields are flat at the top; backend overrides live inside the optional
// `domains` block (mirroring the workspace shape).
type ManifestProject struct {
	Name           string          `json:"name"`
	RelativeDir    string          `json:"relativeDir"`
	TemplateID     string          `json:"templateId"`
	Toolchain      string          `json:"toolchain"`
	BuildVersion   string          `json:"buildVersion"`
	PackageManager string          `json:"packageManager,omitempty"`
	Domains        *ProjectDomains `json:"domains,omitempty"`
}

// ProjectDomains is the per-project override block. Keys mirror
// WorkspaceDomains. The shape of each value differs by domain because the
// scopes carry different state:
//   - env: an override (path / inherits / disabled / keys); kind is
//     always inherited from workspace
//   - container: an override (image / namespace); kind is always
//     inherited from workspace (only "docker" is supported today)
//   - deploy: a full BackendRef (kind + config) because the
//     workspace may host one project on Vercel and another on kustomize
type ProjectDomains struct {
	Env       *ProjectEnvOverride       `json:"env,omitempty"`
	Container *ProjectContainerOverride `json:"container,omitempty"`
	Deploy    *ProjectDeployBackend     `json:"deploy,omitempty"`
	Dev       *ProjectDevOverride       `json:"dev,omitempty"`
}

// ProjectDevOverride is the per-project dev command for `one dev`.
// Written by `one add` at scaffold time (derived from package.json
// scripts + toolchain). Users can hand-edit Command in the manifest to
// customise — there's no auto-sync if package.json scripts change after
// scaffold; manifest is the source of truth.
//
// Empty Command (or missing block) means "this project is not part of
// `one dev`" — the supervisor will skip it.
type ProjectDevOverride struct {
	// Command is the full shell line executed via `sh -c <cmd>`.
	Command string `json:"command,omitempty"`
}

// ProjectEnvOverride is the per-project env override. Carries no `kind`
// field because secrets backends are workspace-scoped.
//
// Keys is the sorted union of variable names ever set against this project
// (across every environment). Writing here on every `one env set` lets
// `one env check` lint every declared environment for completeness — i.e.
// catch the "added FOO to dev, forgot prod" case before deploy. Values
// themselves never live in the manifest; only names.
type ProjectEnvOverride struct {
	Path     string   `json:"path,omitempty"`
	Inherits *bool    `json:"inherits,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	Keys     []string `json:"keys,omitempty"`
}

// ProjectContainerOverride marks a project as having an owned Dockerfile
// (presence of the section means "build this project with `one container
// build`") and carries optional per-project image / registry overrides.
type ProjectContainerOverride struct {
	// Kind selects the container backend implementation. Empty means
	// "docker" (generic Docker registry protocol). Other recognised
	// values: "dockerhub" / "ghcr" / "acr" (Aliyun ACR). Mirrors
	// ProjectDeployBackend.Kind. The resolver falls back to the
	// workspace-level manifest.domains.container.kind, then to
	// "docker".
	Kind string `json:"kind,omitempty"`

	// Image records or overrides the
	// `<registry>/[<namespace>/]<workload>:<version>` tag used by
	// `one container build` / `one deploy`. Optional.
	Image string `json:"image,omitempty"`

	// Namespace is the registry namespace (org / team prefix). Lives
	// per-project because the same registry credential frequently hosts
	// multiple workloads under different namespaces. Empty means "use the
	// container profile default namespace".
	Namespace string `json:"namespace,omitempty"`
}

// ProjectDeployBackend is the per-project deploy backend selection.
// Mirrors BackendRef shape because deploy is the only domain where
// projects in the same workspace genuinely choose different
// implementations (web → s3, api → kustomize). `Config` carries
// kind-specific fields (e.g. the Vercel projectId, the S3 bucket, the
// per-deploy env name); decoded via accessors in
// internal/workspace/domains/deploy/.
type ProjectDeployBackend struct {
	Kind   string          `json:"kind,omitempty"`
	Config json.RawMessage `json:"config,omitempty"`
}

// ManifestPath returns the absolute path to one.manifest.json under
// projectRoot.
func ManifestPath(projectRoot string) string {
	return filepath.Join(projectRoot, ManifestFilename)
}

// ResolveProjectRoot turns a possibly-empty -d flag value into an
// absolute workspace root. When dirFlag is empty the function walks up
// from cwd looking for one.manifest.json; falling back to cwd if no
// workspace marker is found. Used by per-domain CLI commands so each
// verb's RunE can resolve its working root the same way.
func ResolveProjectRoot(dirFlag string) (string, error) {
	if dirFlag == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root, err := WalkUpToManifest(cwd)
		if err == nil && root != "" {
			return root, nil
		}
		return cwd, nil
	}
	if filepath.IsAbs(dirFlag) {
		return dirFlag, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, dirFlag), nil
}

// HasManifest reports whether the manifest file exists.
func HasManifest(projectRoot string) bool {
	_, err := os.Stat(ManifestPath(projectRoot))
	return err == nil
}

// IsOneProjectRoot is an alias for HasManifest.
func IsOneProjectRoot(projectRoot string) bool {
	return HasManifest(projectRoot)
}

// ReadManifest loads and validates the manifest. Returns an empty manifest
// (no error) when the file does not exist. Only the current ManifestVersion
// is accepted; older manifests must be migrated by hand (see CHANGELOG).
func ReadManifest(projectRoot string) (*Manifest, error) {
	path := ManifestPath(projectRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyManifest(), nil
		}
		return nil, cliErrors.New(cliErrors.MANIFEST_INVALID, "one.manifest.json 解析失败。")
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, cliErrors.New(cliErrors.MANIFEST_INVALID, "one.manifest.json 解析失败。")
	}
	if m.Version != ManifestVersion {
		msg := fmt.Sprintf("one.manifest.json 版本 %d 不支持，当前 CLI 仅认 v%d。", m.Version, ManifestVersion)
		if m.Version > ManifestVersion {
			msg += " 请升级 one CLI，或按当前 manifest schema 手动迁移后再重试。"
		}
		if m.Version == 0 {
			msg += " 旧 manifest 需要手动迁移：" +
				"(1) 顶层 env / deploy / container 三个 section 合并到 domains: " +
				"`env.backend → domains.env.kind`，" +
				"`env.{projectId,projectName,rootPath,keys} → domains.env.config.{...}`；" +
				"`deploy.{namespace,kustomizationPath} → domains.deploy.config.{...}`；" +
				"`container.platform → domains.container.config.platform`；" +
				"`preferredProfile` 不进 manifest，改写 ~/.config/one/config.json#workspaces。" +
				"(2) 顶层 env.environments / env.defaultEnv 提到顶层 environments: " +
				"`env.environments → environments.names`，`env.defaultEnv → environments.default`。" +
				"(3) 每个 project 的 env/container/deploy 包到 domains 下: " +
				"`projects[].env → projects[].domains.env`，`projects[].container → projects[].domains.container`，" +
				"`projects[].deploy.target → projects[].domains.deploy.kind`，" +
				"`projects[].deploy.{vercel,cloudflare,edgeone,kustomize}.* → projects[].domains.deploy.config.*`。" +
				"(4) 删除字段：ci / dev（始终启用，不再 opt-in）、ai（默认全启用所有 provider）、" +
				"顶层 packageManager、workspace.roots、environments（旧的 dead map）、所有 profile 字段。" +
				"(5) 顶层 \"version\" 字段改为 1。"
		}
		return nil, cliErrors.New(cliErrors.MANIFEST_INVALID, msg)
	}
	for i := range m.Projects {
		m.Projects[i].RelativeDir = ToPosixPath(m.Projects[i].RelativeDir)
		if m.Projects[i].Toolchain == "" {
			m.Projects[i].Toolchain = "node"
		}
	}
	return &m, nil
}

func emptyManifest() *Manifest {
	return &Manifest{
		Version:  ManifestVersion,
		Projects: []ManifestProject{},
	}
}
