// Package kustomizeplugin generates a Kustomize base + overlays
// structure under kustomize/. base/ holds one <workload>.yaml per
// subproject (Deployment + Service joined by ---); kustomize/base/
// kustomization.yaml lists every resource between sentinel markers,
// so adding a new workload is an idempotent append. Overlays for dev /
// prod are written once and never touched again.
package kustomize

import (
	_ "embed"
	"text/template"
)

const (
	startMarker = "  # one-cli:resources:start"
	endMarker   = "  # one-cli:resources:end"
)

// defaultOverlay is the path used when the profile doesn't pin one.
// prod is the safe default; users targeting dev/staging set
// KustomizationPath in their profile.
const defaultOverlay = "kustomize/overlays/prod"

//go:embed templates/deployment.yaml.tmpl
var deploymentTplRaw string

//go:embed templates/overlay-dev.yaml.tmpl
var overlayDevRaw string

//go:embed templates/overlay-staging.yaml.tmpl
var overlayStagingRaw string

//go:embed templates/overlay-prod.yaml.tmpl
var overlayProdRaw string

var deploymentTpl = template.Must(template.New("deployment").Parse(deploymentTplRaw))
