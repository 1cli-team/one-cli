package preset

import (
	"fmt"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
)

// ResolvedItem pairs a parsed Item with its registry-resolved values:
// the actual *template.Template (so the caller can drive Render /
// applyTemplateDefaults / etc) and the deploy backend id ("" =
// template default).
type ResolvedItem struct {
	Item     Item
	Template *template.Template
	// Deploy is the resolved deploy backend id (e.g. "kustomize",
	// "vercel"). "" means: caller should use the template's own
	// default. Always "" for KindLibrary.
	Deploy string
	// Container is the resolved container backend id (e.g. "dockerhub",
	// "ghcr"). "" means: caller should use the preset default when the
	// effective deploy backend is kustomize.
	Container string
}

// ResolvedSpec is what Apply consumes: the original Spec plus resolved
// pointers + the effective env provider.
type ResolvedSpec struct {
	Spec  Spec
	Items []ResolvedItem
	// EnvProvider is the workspace-level env backend id ("dotenv" or
	// "infisical"). "" when the spec didn't declare one — callers should
	// treat that as the workspace default ("dotenv").
	EnvProvider string
}

// ResolveError is returned by Resolve when a code is well-formed but
// doesn't map to a known template / backend / provider in the current
// registry. The exported fields let createcmd build a rich error
// envelope without further string-parsing.
type ResolveError struct {
	Reason       string
	Kind         string // "template" / "deploy" / "env"
	Segment      string // canonical segment string ("fna", "bgok", "ei", ...) when applicable
	Code         string // the offending code
	TemplateID   string // resolved template id (for deploy-compat errors)
	Compat       []string
	UnknownCount int
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("preset resolve: %s", e.Reason)
}

// Resolve looks up every code in spec against registry, validating
// existence and (for deploy codes) compat. Returns a ResolvedSpec ready
// for Apply, or a *ResolveError describing the first failure.
func Resolve(spec Spec, registry *template.Registry) (ResolvedSpec, error) {
	if registry == nil {
		return ResolvedSpec{}, &ResolveError{Reason: "registry is nil"}
	}
	if len(spec.UnknownSegments) > 0 {
		// Caller decides how strict to be; we surface them so the
		// envelope can echo them back. The first call site (createcmd
		// pre-flight) treats this as fail-fast PRESET_INVALID; future
		// versions may downgrade to warnings.
		return ResolvedSpec{Spec: spec}, &ResolveError{
			Reason:       "preset references unknown segments (CLI may be out of date)",
			Kind:         "extension",
			UnknownCount: len(spec.UnknownSegments),
		}
	}

	byCode := map[string]*template.Template{}
	for i := range registry.Templates {
		t := &registry.Templates[i]
		if t.Code != "" {
			byCode[t.Code] = t
		}
	}

	out := ResolvedSpec{Spec: spec}
	for _, it := range spec.Items {
		tpl := byCode[it.TemplateCode]
		if tpl == nil {
			return ResolvedSpec{}, &ResolveError{
				Reason:  fmt.Sprintf("template code %q is not registered", it.TemplateCode),
				Kind:    "template",
				Segment: itemSegmentString(it),
				Code:    it.TemplateCode,
			}
		}
		// Category match: kind 'f' = frontend, 'b' = backend, 'l' = library.
		if mismatch := kindCategoryMismatch(it.Kind, tpl); mismatch != "" {
			return ResolvedSpec{}, &ResolveError{
				Reason:     mismatch,
				Kind:       "template",
				Segment:    itemSegmentString(it),
				Code:       it.TemplateCode,
				TemplateID: tpl.ID,
			}
		}
		ri := ResolvedItem{Item: it, Template: tpl}
		deployID := ""
		if it.DeployCode != "" {
			deployID = DeployBackendForCode(it.DeployCode[0])
			if deployID == "" {
				return ResolvedSpec{}, &ResolveError{
					Reason:     fmt.Sprintf("deploy code %q is not registered", it.DeployCode),
					Kind:       "deploy",
					Segment:    itemSegmentString(it),
					Code:       it.DeployCode,
					TemplateID: tpl.ID,
				}
			}
			compat := []string{}
			if tpl.Compat != nil {
				compat = append(compat, tpl.Compat["deploy"]...)
			}
			if !containsString(compat, deployID) {
				return ResolvedSpec{}, &ResolveError{
					Reason:     fmt.Sprintf("template %s does not support deploy backend %q", tpl.ID, deployID),
					Kind:       "deploy",
					Segment:    itemSegmentString(it),
					Code:       it.DeployCode,
					TemplateID: tpl.ID,
					Compat:     compat,
				}
			}
			ri.Deploy = deployID
		}
		if deployID == "" && tpl.Defaults != nil {
			deployID = tpl.Defaults["deploy"]
		}
		if it.ContainerCode != "" {
			containerID := ContainerBackendForCode(it.ContainerCode[0])
			if containerID == "" {
				return ResolvedSpec{}, &ResolveError{
					Reason:     fmt.Sprintf("container code %q is not registered", it.ContainerCode),
					Kind:       "container",
					Segment:    itemSegmentString(it),
					Code:       it.ContainerCode,
					TemplateID: tpl.ID,
				}
			}
			if deployID != "kustomize" {
				return ResolvedSpec{}, &ResolveError{
					Reason:     fmt.Sprintf("container backend %q requires deploy backend %q", containerID, "kustomize"),
					Kind:       "container",
					Segment:    itemSegmentString(it),
					Code:       it.ContainerCode,
					TemplateID: tpl.ID,
				}
			}
			if tpl.Defaults == nil || tpl.Defaults["container"] == "" {
				return ResolvedSpec{}, &ResolveError{
					Reason:     fmt.Sprintf("template %s does not support container backends", tpl.ID),
					Kind:       "container",
					Segment:    itemSegmentString(it),
					Code:       it.ContainerCode,
					TemplateID: tpl.ID,
				}
			}
			ri.Container = containerID
		}
		out.Items = append(out.Items, ri)
	}

	if spec.EnvCode != "" {
		envID := EnvProviderForCode(spec.EnvCode[0])
		if envID == "" {
			return ResolvedSpec{}, &ResolveError{
				Reason:  fmt.Sprintf("env code %q is not registered", spec.EnvCode),
				Kind:    "env",
				Segment: "e" + spec.EnvCode,
				Code:    spec.EnvCode,
			}
		}
		out.EnvProvider = envID
	}

	return out, nil
}

// itemSegmentString rebuilds the on-wire segment for an Item, useful in
// error contexts ("fnav", "bgok", "ltl"). Note: the *resolved* segment,
// not necessarily what the user typed (parser already validated shape).
func itemSegmentString(it Item) string {
	var sb strings.Builder
	sb.WriteByte(byte(it.Kind))
	sb.WriteString(it.TemplateCode)
	sb.WriteString(it.DeployCode)
	sb.WriteString(it.ContainerCode)
	return sb.String()
}

func kindCategoryMismatch(k Kind, tpl *template.Template) string {
	expected := map[Kind]template.Category{
		KindFrontend: template.CategoryFrontend,
		KindBackend:  template.CategoryBackend,
		KindLibrary:  template.CategoryLibrary,
	}[k]
	if tpl.Category != expected {
		return fmt.Sprintf("template %s is %s; cannot be used as %s segment",
			tpl.ID, tpl.Category, kindLongName(k))
	}
	return ""
}

func kindLongName(k Kind) string {
	switch k {
	case KindFrontend:
		return "frontend"
	case KindBackend:
		return "backend"
	case KindLibrary:
		return "library"
	default:
		return string(k)
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
