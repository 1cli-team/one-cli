package serve

// handlers_workspace_mutate.go is the write half of /api/workspace/*: it
// patches one.manifest.json on disk so the dashboard can drive the same
// selection flows users would otherwise run via CLI env / deploy / container
// commands. Kept narrow on purpose — these endpoints
// only set the *kind* (and the bare minimum to make the Overview's
// missing-config check flip green). Deep backend config (vercel team,
// kustomize namespace, S3 bucket) still lives in the profile editor + the
// CLI init wizards.
//
// Concurrency: read-modify-write isn't atomic across requests; two
// simultaneous mutations could lose an update. `one serve` is a
// single-user local UI, so we accept that and rely on WriteManifest's
// atomic rename for partial-write protection (the file is never half-
// written; the worst case is the second writer wins).

import (
	"errors"
	"net/http"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// known kinds, kept inline so the serve package doesn't grow a dep on
// internal/profile. Mirrors the lists in workspace/backend.go and
// profile/types.go; the wire contract here is "valid current kind strings".

var knownEnvKinds = map[string]struct{}{
	workspace.EnvBackendDotenv:    {},
	workspace.EnvBackendInfisical: {},
}

var knownDeployKinds = map[string]struct{}{
	workspace.DeployBackendKustomize:  {},
	workspace.DeployBackendAliyunOSS:  {},
	workspace.DeployBackendTencentCOS: {},
	workspace.DeployBackendAWSS3:      {},
	workspace.DeployBackendMinIO:      {},
	workspace.DeployBackendRustFS:     {},
	workspace.DeployBackendR2:         {},
	workspace.DeployBackendVercel:     {},
	workspace.DeployBackendCloudflare: {},
	workspace.DeployBackendEdgeOne:    {},
}

var knownContainerKinds = map[string]struct{}{
	"docker":    {},
	"dockerhub": {},
	"ghcr":      {},
	"acr":       {},
}

func registerWorkspaceMutateRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("PUT /workspace/domains/env", handlePutWorkspaceEnv(opts))
	mux.HandleFunc("PUT /workspace/projects/{name}/deploy", handlePutProjectDeploy(opts))
	mux.HandleFunc("PUT /workspace/projects/{name}/container", handlePutProjectContainer(opts))
}

type envInitReq struct {
	Kind string `json:"kind"`
}

func handlePutWorkspaceEnv(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		root := opts.WorkspaceRoot
		if root == "" {
			writeNoWorkspace(w)
			return
		}
		var body envInitReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		kind := strings.TrimSpace(body.Kind)
		if _, ok := knownEnvKinds[kind]; !ok {
			writeBadPayload(w, "unknown env backend kind: "+body.Kind)
			return
		}

		m, err := workspace.ReadManifest(root)
		if err != nil {
			writeManifestErr(w, err)
			return
		}
		if m.Domains == nil {
			m.Domains = &workspace.WorkspaceDomains{}
		}
		if m.Domains.Env == nil {
			m.Domains.Env = &workspace.BackendRef{}
		}
		m.Domains.Env.Kind = kind
		if err := workspace.WriteManifest(root, m); err != nil {
			writeManifestErr(w, err)
			return
		}
		writeOverviewAfterMutate(w, root)
	}
}

type deployInitReq struct {
	Kind string `json:"kind"`
}

func handlePutProjectDeploy(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		root := opts.WorkspaceRoot
		if root == "" {
			writeNoWorkspace(w)
			return
		}
		name := r.PathValue("name")
		var body deployInitReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		kind := strings.TrimSpace(body.Kind)
		if _, ok := knownDeployKinds[kind]; !ok {
			writeBadPayload(w, "unknown deploy backend kind: "+body.Kind)
			return
		}
		m, err := workspace.ReadManifest(root)
		if err != nil {
			writeManifestErr(w, err)
			return
		}
		p := findProject(m, name)
		if p == nil {
			writeNotFound(w, "project not found: "+name)
			return
		}
		if p.Domains == nil {
			p.Domains = &workspace.ProjectDomains{}
		}
		if p.Domains.Deploy == nil {
			p.Domains.Deploy = &workspace.ProjectDeployBackend{}
		}
		p.Domains.Deploy.Kind = kind
		if err := workspace.WriteManifest(root, m); err != nil {
			writeManifestErr(w, err)
			return
		}
		writeOverviewAfterMutate(w, root)
	}
}

type containerInitReq struct {
	Kind  string `json:"kind,omitempty"`
	Image string `json:"image,omitempty"`
}

func handlePutProjectContainer(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		root := opts.WorkspaceRoot
		if root == "" {
			writeNoWorkspace(w)
			return
		}
		name := r.PathValue("name")
		var body containerInitReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		kind := strings.TrimSpace(body.Kind)
		// Container kind is OPTIONAL — empty falls back to workspace
		// default (or "docker" implicit). Only validate when the client
		// is explicit. This matches the current resolution chain in
		// workspace.ContainerKindForProject.
		if kind != "" {
			if _, ok := knownContainerKinds[kind]; !ok {
				writeBadPayload(w, "unknown container backend kind: "+body.Kind)
				return
			}
		}
		m, err := workspace.ReadManifest(root)
		if err != nil {
			writeManifestErr(w, err)
			return
		}
		p := findProject(m, name)
		if p == nil {
			writeNotFound(w, "project not found: "+name)
			return
		}
		if p.Domains == nil {
			p.Domains = &workspace.ProjectDomains{}
		}
		if p.Domains.Container == nil {
			p.Domains.Container = &workspace.ProjectContainerOverride{}
		}
		p.Domains.Container.Kind = kind
		p.Domains.Container.Image = strings.TrimSpace(body.Image)
		if err := workspace.WriteManifest(root, m); err != nil {
			writeManifestErr(w, err)
			return
		}
		writeOverviewAfterMutate(w, root)
	}
}

// findProject returns a pointer into m.Projects matching name (case
// sensitive). nil when not found.
func findProject(m *workspace.Manifest, name string) *workspace.ManifestProject {
	if m == nil {
		return nil
	}
	for i := range m.Projects {
		if m.Projects[i].Name == name {
			return &m.Projects[i]
		}
	}
	return nil
}

// writeOverviewAfterMutate rebuilds the Overview from disk and returns
// it. The dashboard's SWR cache uses this response to repaint without a
// second round-trip.
func writeOverviewAfterMutate(w http.ResponseWriter, root string) {
	ov, err := workspace.BuildOverview(root)
	if err != nil {
		writeManifestErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func writeNoWorkspace(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, cliErrors.NOT_ONE_PROJECT,
		"`one serve` was launched outside a One workspace — there is no manifest to mutate.",
		nil)
}

func writeBadPayload(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, msg, nil)
}

func writeNotFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, cliErrors.ONE_CLI_ERROR, msg, nil)
}

func writeManifestErr(w http.ResponseWriter, err error) {
	// ReadManifest / WriteManifest can return either a typed CLI error or
	// a vanilla error; surface either way via the generic envelope.
	msg := err.Error()
	if errors.Is(err, workspace.ErrEnvBackendNotConfigured) {
		writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, msg, nil)
		return
	}
	writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID, msg, nil)
}
