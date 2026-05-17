package workspace

// overview.go builds the workspace-shaped payload consumed by the dashboard
// Overview page. It reads the shared manifest plus the machine-local profile
// store, but never returns credential values. Drift-vs-disk checks (e.g. "the
// Dockerfile this manifest claims doesn't exist") live elsewhere — this is
// the first read for "does this workspace have the four key configs filled
// in".

import (
	"errors"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
)

// OverviewSchema is the JSON envelope version stamp.
const OverviewSchema = "one-cli/workspace-overview/v1"

// Project kinds derived from RelativeDir prefix.
const (
	ProjectKindApp     = "app"
	ProjectKindService = "service"
	ProjectKindPackage = "package"
)

// Issue domains and severities the dashboard knows how to render.
//
// Note: dev command is intentionally NOT a domain here. `one add` writes
// projects[].domains.dev.command from a package.json-scripts heuristic
// (see workspace.ResolveDevCommand); an empty Command is a *valid* state
// meaning "this project does not participate in `one dev`" (the supervisor
// skips it silently). Flagging that as a missing-config issue would be a
// false positive.
const (
	IssueDomainContainer = "container"
	IssueDomainDeploy    = "deploy"
	IssueDomainEnv       = "env"

	IssueSeverityMissing = "missing"

	IssueReasonBackend = "backend"
	IssueReasonProfile = "profile"
)

// Overview is the response shape for GET /api/workspace/overview. Present is
// false when there is no manifest at the captured root; in that case all
// other fields are zero.
type Overview struct {
	Schema    string             `json:"schema"`
	Present   bool               `json:"present"`
	Root      string             `json:"root,omitempty"`
	Workspace *OverviewWorkspace `json:"workspace,omitempty"`
	Projects  []OverviewProject  `json:"projects,omitempty"`
	Issues    []OverviewIssue    `json:"issues,omitempty"`
}

// OverviewWorkspace surfaces workspace-level identity and the bare backend
// kinds selected at workspace scope. The kind map only includes domains
// that have a backend chosen — missing keys mean "not configured at
// workspace scope".
type OverviewWorkspace struct {
	ID                 string            `json:"id,omitempty"`
	Name               string            `json:"name,omitempty"`
	ManifestVersion    int               `json:"manifestVersion"`
	DefaultEnvironment string            `json:"defaultEnvironment,omitempty"`
	Environments       []string          `json:"environments,omitempty"`
	Domains            map[string]string `json:"domains,omitempty"`
}

// OverviewProject is one entry in manifest.projects[], plus per-project
// issues. Domains carries the resolved per-project backend kinds (workspace
// default merged with per-project overrides) so the UI can render the
// effective state without duplicating selector logic.
type OverviewProject struct {
	Name        string            `json:"name"`
	RelativeDir string            `json:"relativeDir"`
	Kind        string            `json:"kind"`
	TemplateID  string            `json:"templateId,omitempty"`
	Toolchain   string            `json:"toolchain,omitempty"`
	Domains     map[string]string `json:"domains,omitempty"`
	Issues      []OverviewIssue   `json:"issues,omitempty"`
}

// OverviewIssue is one "missing configuration" finding. Message is a short
// English fallback; the dashboard maps (domain, severity) to localized
// strings via i18n.
type OverviewIssue struct {
	Domain   string `json:"domain"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Reason   string `json:"reason,omitempty"`
	Backend  string `json:"backend,omitempty"`
	Section  string `json:"section,omitempty"`
	Profile  string `json:"profile,omitempty"`
}

// BuildOverview reads the manifest at root and produces the Overview
// payload. Empty root or missing manifest is not an error — returns
// {Present: false}. Any malformed-manifest error from ReadManifest bubbles
// up unchanged so the handler can return a proper error envelope.
func BuildOverview(root string) (Overview, error) {
	if strings.TrimSpace(root) == "" {
		return Overview{Schema: OverviewSchema, Present: false}, nil
	}
	m, err := ReadManifest(root)
	if err != nil {
		return Overview{Schema: OverviewSchema, Present: false}, err
	}
	if m == nil || !HasManifest(root) {
		return Overview{Schema: OverviewSchema, Present: false}, nil
	}
	profiles, _, err := profile.Load()
	if err != nil {
		return Overview{Schema: OverviewSchema, Present: false}, err
	}

	ov := Overview{
		Schema:    OverviewSchema,
		Present:   true,
		Root:      root,
		Workspace: buildWorkspaceSummary(m),
		Projects:  make([]OverviewProject, 0, len(m.Projects)),
	}

	if m.Domains == nil || m.Domains.Env == nil || strings.TrimSpace(m.Domains.Env.Kind) == "" {
		ov.Issues = append(ov.Issues, OverviewIssue{
			Domain:   IssueDomainEnv,
			Severity: IssueSeverityMissing,
			Message:  "workspace env backend is not selected",
			Reason:   IssueReasonBackend,
		})
	} else if issue := profileIssue(profiles, m, IssueDomainEnv, m.Domains.Env.Kind, ""); issue != nil {
		ov.Issues = append(ov.Issues, *issue)
	}

	for i := range m.Projects {
		ov.Projects = append(ov.Projects, buildProject(m, profiles, &m.Projects[i]))
	}
	return ov, nil
}

func buildWorkspaceSummary(m *Manifest) *OverviewWorkspace {
	s := &OverviewWorkspace{ManifestVersion: m.Version}
	if m.Workspace != nil {
		s.ID = m.Workspace.ID
		s.Name = m.Workspace.Name
	}
	if m.Environments != nil {
		s.DefaultEnvironment = m.Environments.Default
		if len(m.Environments.Names) > 0 {
			s.Environments = append([]string(nil), m.Environments.Names...)
		}
	}
	if m.Domains != nil {
		domains := map[string]string{}
		if m.Domains.Env != nil && m.Domains.Env.Kind != "" {
			domains[IssueDomainEnv] = m.Domains.Env.Kind
		}
		if m.Domains.Deploy != nil && m.Domains.Deploy.Kind != "" {
			domains[IssueDomainDeploy] = m.Domains.Deploy.Kind
		}
		if m.Domains.Container != nil && m.Domains.Container.Kind != "" {
			domains[IssueDomainContainer] = m.Domains.Container.Kind
		}
		if len(domains) > 0 {
			s.Domains = domains
		}
	}
	return s
}

func buildProject(m *Manifest, profiles *profile.Config, p *ManifestProject) OverviewProject {
	kind := projectKindFromDir(p.RelativeDir)
	out := OverviewProject{
		Name:        p.Name,
		RelativeDir: p.RelativeDir,
		Kind:        kind,
		TemplateID:  p.TemplateID,
		Toolchain:   p.Toolchain,
		Domains:     projectResolvedDomains(m, p),
	}

	// packages aren't expected to deploy / build containers / have a dev
	// command, so we suppress those three checks for them.
	if kind == ProjectKindPackage {
		return out
	}

	if projectNeedsContainer(m, p) {
		hasContainerWorkspaceDefault := m.Domains != nil && m.Domains.Container != nil
		if enabled, _ := ContainerForProject(m, p.Name); !enabled && !hasContainerWorkspaceDefault {
			out.Issues = append(out.Issues, OverviewIssue{
				Domain:   IssueDomainContainer,
				Severity: IssueSeverityMissing,
				Message:  "no container backend selected for this project",
				Reason:   IssueReasonBackend,
			})
		}
	}
	if backend := out.Domains[IssueDomainContainer]; backend != "" {
		if issue := profileIssue(profiles, m, IssueDomainContainer, backend, p.Name); issue != nil {
			out.Issues = append(out.Issues, *issue)
		}
	}

	hasDeployWorkspaceDefault := m.Domains != nil && m.Domains.Deploy != nil && m.Domains.Deploy.Kind != ""
	if DeployForProject(m, p.Name).Backend == "" && !hasDeployWorkspaceDefault {
		out.Issues = append(out.Issues, OverviewIssue{
			Domain:   IssueDomainDeploy,
			Severity: IssueSeverityMissing,
			Message:  "no deploy backend selected for this project",
			Reason:   IssueReasonBackend,
		})
	}
	if backend := out.Domains[IssueDomainDeploy]; backend != "" {
		if issue := profileIssue(profiles, m, IssueDomainDeploy, backend, p.Name); issue != nil {
			out.Issues = append(out.Issues, *issue)
		}
	}

	return out
}

func profileIssue(cfg *profile.Config, m *Manifest, domain, backend, projectName string) *OverviewIssue {
	if cfg == nil || backend == "" {
		return nil
	}
	if backend == EnvBackendDotenv || backend == DeployBackendKustomize || backend == DeployBackendEdgeOne {
		return nil
	}
	section := profile.SectionKey(profile.Domain(domain), backend)
	resolved, err := profile.Resolve(profile.ResolveInput{
		Domain:      profile.Domain(domain),
		Backend:     backend,
		WorkspaceID: manifestWorkspaceID(m),
		ProjectName: projectName,
	})
	if err != nil {
		requestedProfile := profileNameFromResolveError(err)
		return &OverviewIssue{
			Domain:   domain,
			Severity: IssueSeverityMissing,
			Reason:   IssueReasonProfile,
			Backend:  backend,
			Section:  section,
			Profile:  requestedProfile,
			Message:  "no credential profile configured for " + section,
		}
	}
	if !profileComplete(backend, resolved.Profile) {
		return &OverviewIssue{
			Domain:   domain,
			Severity: IssueSeverityMissing,
			Reason:   IssueReasonProfile,
			Backend:  backend,
			Section:  section,
			Profile:  resolved.Name,
			Message:  "credential profile " + resolved.Name + " for " + section + " is missing required credentials",
		}
	}
	return nil
}

func profileNameFromResolveError(err error) string {
	var cliErr *output.Error
	if !errors.As(err, &cliErr) || cliErr.Context == nil {
		return ""
	}
	for _, key := range []string{"requested", "profile"} {
		if value, ok := cliErr.Context[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func manifestWorkspaceID(m *Manifest) string {
	if m == nil || m.Workspace == nil {
		return ""
	}
	return strings.TrimSpace(m.Workspace.ID)
}

func profileComplete(backend string, p profile.Profile) bool {
	switch {
	case p.Infisical != nil:
		c := p.Infisical.Credentials
		return c != nil && strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
	case p.S3 != nil:
		c := p.S3.Credentials
		return c != nil && strings.TrimSpace(c.AccessKeyID) != "" && strings.TrimSpace(c.AccessKeySecret) != ""
	case p.Vercel != nil:
		return p.Vercel.Credentials != nil && strings.TrimSpace(p.Vercel.Credentials.APIToken) != ""
	case p.Cloudflare != nil:
		return p.Cloudflare.Credentials != nil && strings.TrimSpace(p.Cloudflare.Credentials.APIToken) != ""
	case p.Container != nil:
		c := p.Container.Credentials
		return c != nil && strings.TrimSpace(c.Username) != "" && strings.TrimSpace(c.Password) != ""
	case p.Dotenv != nil:
		return true
	case p.Kustomize != nil:
		return strings.TrimSpace(p.Kustomize.KubeconfigPath) != ""
	case p.EdgeOne != nil:
		// EdgeOne supports `edgeone login`; an inline token is useful but not mandatory.
		return true
	default:
		_ = backend
		return false
	}
}

// projectResolvedDomains collapses workspace defaults + per-project
// overrides into a single domain→kind map (same logic the UI would re-do).
// "container" only appears when the project actually opted in via its own
// override OR a workspace-level default exists; that matches what
// container-related commands actually run.
func projectResolvedDomains(m *Manifest, p *ManifestProject) map[string]string {
	out := map[string]string{}
	if env := EnvBackend(m); env != "" {
		out[IssueDomainEnv] = env
	}
	if sel := DeployForProject(m, p.Name); sel.Backend != "" {
		out[IssueDomainDeploy] = sel.Backend
	} else if m.Domains != nil && m.Domains.Deploy != nil && m.Domains.Deploy.Kind != "" {
		out[IssueDomainDeploy] = m.Domains.Deploy.Kind
	}
	if enabled, _ := ContainerForProject(m, p.Name); enabled {
		out[IssueDomainContainer] = ContainerKindForProject(m, p.Name)
	} else if projectNeedsContainer(m, p) && m.Domains != nil && m.Domains.Container != nil && m.Domains.Container.Kind != "" {
		out[IssueDomainContainer] = m.Domains.Container.Kind
	}
	if len(out) == 0 {
		return nil
	}
	// Stable order for snapshot-friendly callers; map iteration is random.
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(out))
	for _, k := range keys {
		ordered[k] = out[k]
	}
	return ordered
}

func projectEffectiveDeploy(m *Manifest, p *ManifestProject) string {
	if m == nil || p == nil {
		return ""
	}
	if sel := DeployForProject(m, p.Name); sel.Backend != "" {
		return sel.Backend
	}
	if m.Domains != nil && m.Domains.Deploy != nil {
		return strings.TrimSpace(m.Domains.Deploy.Kind)
	}
	return ""
}

func projectNeedsContainer(m *Manifest, p *ManifestProject) bool {
	return projectEffectiveDeploy(m, p) == DeployBackendKustomize
}

// projectKindFromDir maps the manifest's RelativeDir to a coarse "app /
// service / package" label. Anything outside the hard-wired roots
// (apps/services/packages) falls through to "app" — workspaces only carry
// projects under those three trees, so the fallback is a defensive default
// for hand-written manifests rather than an expected path.
func projectKindFromDir(relativeDir string) string {
	dir := strings.TrimSpace(relativeDir)
	dir = strings.TrimPrefix(dir, "./")
	switch {
	case strings.HasPrefix(dir, "apps/") || dir == "apps":
		return ProjectKindApp
	case strings.HasPrefix(dir, "services/") || dir == "services":
		return ProjectKindService
	case strings.HasPrefix(dir, "packages/") || dir == "packages":
		return ProjectKindPackage
	default:
		return ProjectKindApp
	}
}
