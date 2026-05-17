// Selection write helpers shared by `one create --secrets` (via
// ApplyBackendSelection) and the template default-application flow in
// `one add` (via SetWorkspaceSelection / SetPerProjectSelection).
//
// All helpers operate on the current manifest shape (Manifest.Domains,
// ManifestProject.Domains).
package workspace

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ApplyBackendSelection writes a list of fully-qualified ids
// ("<domain>/<backend>") into the manifest domains block. Selecting two ids for
// the same domain is rejected (mutual exclusion).
func ApplyBackendSelection(projectRoot string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	seen := map[string]string{}
	for _, raw := range ids {
		idx := strings.IndexByte(raw, '/')
		if idx <= 0 || idx == len(raw)-1 {
			return fmt.Errorf("invalid id %q: expected <domain>/<backend>", raw)
		}
		domain := raw[:idx]
		if prev, dupe := seen[domain]; dupe && prev != raw {
			return fmt.Errorf("two selections for domain %q: %q and %q", domain, prev, raw)
		}
		seen[domain] = raw
		applyDomainSelection(m, domain, raw)
	}
	return WriteManifest(projectRoot, m)
}

// SetWorkspaceSelection writes one workspace-scoped selection (env) into
// the manifest domains block. Returns the previous bare-name backend (empty if
// unset). domain is the bare domain name. (ci / dev are not persisted.)
func SetWorkspaceSelection(projectRoot, domain, id string) (previous string, err error) {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return "", err
	}
	previous = previousWorkspaceSelection(m, domain)
	applyDomainSelection(m, domain, id)
	if err := WriteManifest(projectRoot, m); err != nil {
		return "", err
	}
	return previous, nil
}

// SetPerProjectSelection writes a per-project scoped selection
// (Projects[name].Domains.Container or Projects[name].Domains.Deploy).
// Returns the previous bare-name backend (empty if unset). domain must be
// "container" or "deploy".
func SetPerProjectSelection(projectRoot, domain, id, projectName string) (previous string, err error) {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return "", err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return "", err
	}
	ensureProjectDomains(&m.Projects[idx])
	switch domain {
	case "container":
		if m.Projects[idx].Domains.Container != nil {
			previous = ContainerBackendDocker
		}
		if id == "" {
			m.Projects[idx].Domains.Container = nil
		} else {
			// Preserve any existing override fields by leaving the struct
			// intact when one already exists; create a fresh empty
			// override otherwise.
			if m.Projects[idx].Domains.Container == nil {
				m.Projects[idx].Domains.Container = &ProjectContainerOverride{}
			}
		}
	case "deploy":
		if m.Projects[idx].Domains.Deploy != nil {
			previous = m.Projects[idx].Domains.Deploy.Kind
		}
		if id == "" {
			m.Projects[idx].Domains.Deploy = nil
		} else {
			kind := stripDomainPrefix(id)
			if m.Projects[idx].Domains.Deploy == nil {
				m.Projects[idx].Domains.Deploy = &ProjectDeployBackend{Kind: kind}
			} else {
				// Preserve config when re-selecting the same kind; reset
				// config when switching kinds.
				if m.Projects[idx].Domains.Deploy.Kind != kind {
					m.Projects[idx].Domains.Deploy.Config = nil
				}
				m.Projects[idx].Domains.Deploy.Kind = kind
			}
		}
	default:
		return "", fmt.Errorf("domain %q has no per-project scope", domain)
	}
	pruneProjectDomains(&m.Projects[idx])
	if err := WriteManifest(projectRoot, m); err != nil {
		return "", err
	}
	return previous, nil
}

// SetProjectContainerNamespace writes the registry namespace into
// projects[name].domains.container.namespace. The container section must
// already exist (caller adds container backend first via
// SetPerProjectSelection); this helper only fills the field. Empty
// namespace clears the field.
func SetProjectContainerNamespace(projectRoot, projectName, namespace string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return err
	}
	ensureProjectContainer(&m.Projects[idx])
	m.Projects[idx].Domains.Container.Namespace = namespace
	return WriteManifest(projectRoot, m)
}

// SetProjectContainerImage writes the fully-resolved image ref used by
// build / push back into the project's container section so deploy
// backends can consume the same image without asking users to edit
// Kubernetes YAML by hand.
func SetProjectContainerImage(projectRoot, projectName, image string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return err
	}
	ensureProjectContainer(&m.Projects[idx])
	m.Projects[idx].Domains.Container.Image = strings.TrimSpace(image)
	return WriteManifest(projectRoot, m)
}

// SetProjectContainerKind writes projects[name].domains.container.kind.
// The container section is created when missing because selecting a kind
// is itself an explicit opt-in to container builds.
func SetProjectContainerKind(projectRoot, projectName, kind string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return err
	}
	ensureProjectContainer(&m.Projects[idx])
	m.Projects[idx].Domains.Container.Kind = strings.TrimSpace(kind)
	return WriteManifest(projectRoot, m)
}

// SetWorkspaceContainerPlatform writes the target Docker image platform
// used by `one container build`, e.g. "linux/amd64". Empty platform clears
// the field.
func SetWorkspaceContainerPlatform(projectRoot, platform string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	ensureWorkspaceContainer(m)
	cfg := workspaceContainerConfig(m)
	if cfg == nil {
		cfg = &workspaceContainerConfigShape{}
	}
	cfg.Platform = strings.TrimSpace(platform)
	return writeWorkspaceContainerConfig(m, projectRoot, cfg)
}

// SetProjectBuildVersion writes projects[name].buildVersion. Versions are
// stored without a leading "v" even when Docker tags use one.
func SetProjectBuildVersion(projectRoot, projectName, version string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return err
	}
	m.Projects[idx].BuildVersion = NormalizeBuildVersion(version)
	return WriteManifest(projectRoot, m)
}

// SetProjectDeployBucket writes the S3 bucket into
// projects[name].domains.deploy.config.bucket. The deploy section must
// already exist (caller adds deploy backend first via
// SetPerProjectSelection). Empty bucket clears the field.
func SetProjectDeployBucket(projectRoot, projectName, bucket string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return err
	}
	if m.Projects[idx].Domains == nil || m.Projects[idx].Domains.Deploy == nil {
		return fmt.Errorf("project %q has no deploy section; set the deploy backend first", projectName)
	}
	cfg := map[string]json.RawMessage{}
	if len(m.Projects[idx].Domains.Deploy.Config) > 0 {
		if err := json.Unmarshal(m.Projects[idx].Domains.Deploy.Config, &cfg); err != nil {
			return fmt.Errorf("project %q deploy config is malformed: %w", projectName, err)
		}
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		delete(cfg, "bucket")
	} else {
		raw, err := json.Marshal(bucket)
		if err != nil {
			return err
		}
		cfg["bucket"] = raw
	}
	if len(cfg) == 0 {
		m.Projects[idx].Domains.Deploy.Config = nil
	} else {
		raw, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		m.Projects[idx].Domains.Deploy.Config = raw
	}
	return WriteManifest(projectRoot, m)
}

// SetWorkspaceDeployTarget writes workspace-level explicit k8s namespace
// override + kustomize overlay path into manifest.domains.deploy.config.
// Either argument may be empty to clear that single field; empty namespace
// falls back to workspace.id at read time. Empty manifest.domains.deploy
// section is created on first call.
func SetWorkspaceDeployTarget(projectRoot, namespace, kustomizationPath string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	ensureWorkspaceDeploy(m)
	cfg := workspaceDeployConfig(m)
	if cfg == nil {
		cfg = &workspaceDeployConfigShape{}
	}
	cfg.Namespace = namespace
	cfg.KustomizationPath = kustomizationPath
	return writeWorkspaceDeployConfig(m, projectRoot, cfg)
}

// SetWorkspaceDeployK8sTarget writes the k8s deploy target chosen by k8s
// deploy configuration: explicit namespace override + overlay path.
// Empty namespace falls back to workspace.id at read time; empty
// kustomizationPath clears the field. Callers normally pass the canonical
// production overlay.
func SetWorkspaceDeployK8sTarget(projectRoot, namespace, kustomizationPath string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	ensureWorkspaceDeploy(m)
	cfg := workspaceDeployConfig(m)
	if cfg == nil {
		cfg = &workspaceDeployConfigShape{}
	}
	cfg.Namespace = strings.TrimSpace(namespace)
	cfg.KustomizationPath = strings.TrimSpace(kustomizationPath)
	return writeWorkspaceDeployConfig(m, projectRoot, cfg)
}

// applyDomainSelection writes the given namespaced id into the appropriate
// field on m. Unknown domains are silently ignored — the registry
// validation test catches mismatches at build time. CI / Dev are not
// persisted (always-on); selections targeting those domains are
// silently dropped.
func applyDomainSelection(m *Manifest, domain, id string) {
	bare := stripDomainPrefix(id)
	switch domain {
	case "env":
		ensureWorkspaceEnv(m)
		m.Domains.Env.Kind = bare
		// Seed the workspace-level environment list on first selection.
		// Both backends (dotenv / infisical) treat
		// manifest.environments.names as the authoritative list, so
		// stamping defaults here keeps `one env get/set --env <name>`
		// usable from the moment the workspace is created.
		if m.Environments == nil {
			m.Environments = &Environments{}
		}
		if len(m.Environments.Names) == 0 {
			m.Environments.Names = append([]string{}, DefaultEnvironments...)
		}
		if m.Environments.Default == "" && len(m.Environments.Names) > 0 {
			m.Environments.Default = m.Environments.Names[0]
		}
	}
}

// previousWorkspaceSelection returns the prior bare-name backend for a
// workspace-scoped domain, used so callers can roll back on failure. CI /
// Dev domains always report empty because they are not persisted as backend selections.
func previousWorkspaceSelection(m *Manifest, domain string) string {
	switch domain {
	case "env":
		if m != nil && m.Domains != nil && m.Domains.Env != nil {
			return m.Domains.Env.Kind
		}
	}
	return ""
}

// stripDomainPrefix turns "env/dotenv" / "deploy/aws-s3" / "container/docker"
// into the bare backend name. Inputs without a slash pass through.
func stripDomainPrefix(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == '/' {
			if i == len(id)-1 {
				return id
			}
			return id[i+1:]
		}
	}
	return id
}

// projectIndex returns the index of projectName in m.Projects, or an
// error if not found.
func projectIndex(m *Manifest, projectName string) (int, error) {
	for i := range m.Projects {
		if m.Projects[i].Name == projectName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("project %q not found in manifest", projectName)
}

func ensureProjectDomains(p *ManifestProject) {
	if p.Domains == nil {
		p.Domains = &ProjectDomains{}
	}
}

func ensureProjectContainer(p *ManifestProject) {
	ensureProjectDomains(p)
	if p.Domains.Container == nil {
		p.Domains.Container = &ProjectContainerOverride{}
	}
}

// pruneProjectDomains drops the Domains pointer when every override is
// empty, keeping JSON output tidy.
func pruneProjectDomains(p *ManifestProject) {
	if p.Domains == nil {
		return
	}
	if p.Domains.Env == nil && p.Domains.Container == nil && p.Domains.Deploy == nil && p.Domains.Dev == nil {
		p.Domains = nil
	}
}

func ensureWorkspaceEnv(m *Manifest) {
	if m.Domains == nil {
		m.Domains = &WorkspaceDomains{}
	}
	if m.Domains.Env == nil {
		m.Domains.Env = &BackendRef{}
	}
}

func ensureWorkspaceDeploy(m *Manifest) {
	if m.Domains == nil {
		m.Domains = &WorkspaceDomains{}
	}
	if m.Domains.Deploy == nil {
		m.Domains.Deploy = &BackendRef{Kind: DeployBackendKustomize}
	}
}

func ensureWorkspaceContainer(m *Manifest) {
	if m.Domains == nil {
		m.Domains = &WorkspaceDomains{}
	}
	if m.Domains.Container == nil {
		m.Domains.Container = &BackendRef{Kind: ContainerBackendDocker}
	}
}

func writeWorkspaceDeployConfig(m *Manifest, projectRoot string, cfg *workspaceDeployConfigShape) error {
	if cfg == nil || (cfg.Namespace == "" && cfg.KustomizationPath == "") {
		if m.Domains != nil && m.Domains.Deploy != nil {
			m.Domains.Deploy.Config = nil
		}
	} else {
		raw, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		m.Domains.Deploy.Config = raw
	}
	return WriteManifest(projectRoot, m)
}

func writeWorkspaceContainerConfig(m *Manifest, projectRoot string, cfg *workspaceContainerConfigShape) error {
	if cfg == nil || cfg.Platform == "" {
		if m.Domains != nil && m.Domains.Container != nil {
			m.Domains.Container.Config = nil
		}
	} else {
		raw, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		m.Domains.Container.Config = raw
	}
	return WriteManifest(projectRoot, m)
}
