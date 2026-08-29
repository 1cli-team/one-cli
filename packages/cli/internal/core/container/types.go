// Package container owns the transport-neutral inputs and results for OCI
// image inspection, build, push, and registry resolution. Execution lives in
// the compiled container module and Docker adapter; these types are not an
// extension interface.
package container

import (
	"fmt"
	"io"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Registry is the resolved OCI registry endpoint and credentials used to
// compose image tags. An empty Registry means a local-daemon-only build.
type Registry struct {
	Registry      string
	Namespace     string
	Username      string
	Password      string
	ProfileName   string
	ProfileSource string
}

// HasCredentials reports whether the registry is fully populated for login.
func (r *Registry) HasCredentials() bool {
	return r != nil && r.Registry != "" && r.Username != "" && r.Password != ""
}

// ImageTagVersion returns the tag suffix of an OCI image reference. A colon
// in the registry host is ignored, so localhost:5000/team/api:v1 returns v1.
func ImageTagVersion(reference string) string {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ""
	}
	colon := strings.LastIndex(reference, ":")
	slash := strings.LastIndex(reference, "/")
	if colon > slash {
		return reference[colon+1:]
	}
	return ""
}

type InfoInput struct {
	ProjectRoot string
	TargetNames []string
}

type ProjectInfo struct {
	Name          string `json:"name"`
	RelativeDir   string `json:"relative_dir"`
	Backend       string `json:"backend,omitempty"`
	HasArtifact   bool   `json:"has_artifact"`
	ArtifactPath  string `json:"artifact_path,omitempty"`
	WorkloadName  string `json:"workload_name,omitempty"`
	ImageOverride string `json:"image_override,omitempty"`
}

type InfoResult struct {
	Schema           string        `json:"schema"`
	Workspace        string        `json:"workspace"`
	ContainerBackend string        `json:"container_backend"`
	Projects         []ProjectInfo `json:"projects"`
}

func (r *InfoResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, i18n.T("container.info_title")+"\n", r.Workspace)
	for _, project := range r.Projects {
		state := i18n.T("container.artifact_missing")
		if project.HasArtifact {
			state = i18n.T("container.artifact_ready")
		}
		fmt.Fprintf(w, "  %s  %s\n", project.Name, state)
	}
}

type BuildInput struct {
	ProjectRoot string
	Project     string
	TargetNames []string
	Tag         string
	Platform    string
	DryRun      bool
	Registry    *Registry
}

type BuildEntry struct {
	Project string   `json:"project"`
	Image   string   `json:"image"`
	Argv    []string `json:"argv"`
	DryRun  bool     `json:"dry_run"`
}

type BuildResult struct {
	Schema string       `json:"schema"`
	Built  []BuildEntry `json:"built"`
}

func (r *BuildResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	for _, entry := range r.Built {
		fmt.Fprintf(w, i18n.T("container.build_success")+"\n", entry.Project, entry.Image)
	}
	if len(r.Built) == 1 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, i18n.T("container.build_next_project")+"\n", r.Built[0].Project)
	} else if len(r.Built) > 1 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, i18n.T("container.build_next_all"))
	}
}

type PushInput struct {
	ProjectRoot string
	Project     string
	TargetNames []string
	Tag         string
	DryRun      bool
	Registry    *Registry
}

type PushEntry struct {
	Project     string   `json:"project"`
	Image       string   `json:"image"`
	SourceImage string   `json:"source_image,omitempty"`
	Retagged    bool     `json:"retagged,omitempty"`
	Argv        []string `json:"argv"`
	DryRun      bool     `json:"dry_run"`
}

type PushResult struct {
	Schema string      `json:"schema"`
	Pushed []PushEntry `json:"pushed"`
}

func (r *PushResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	for _, entry := range r.Pushed {
		fmt.Fprintf(w, i18n.T("container.push_success")+"\n", entry.Project, entry.Image)
	}
}
