package preset

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ResolvedItem pairs a parsed Item with its registry template.
type ResolvedItem struct {
	Item     Item
	Template *template.Template
}

// ResolvedSpec is what Apply consumes: the original Spec plus resolved
// pointers + the effective env provider.
type ResolvedSpec struct {
	Spec  Spec
	Items []ResolvedItem
	// EnvProvider resolves a frozen preset code. Empty means Infisical.
	// Retired codes remain decodable but creation rejects unsupported sources.
	EnvProvider string
}

// ResolveError is returned by Resolve when a code is well-formed but
// doesn't map to a known template / backend / provider in the current
// registry. The exported fields let createcmd build a rich error
// envelope without further string-parsing.
type ResolveError struct {
	Reason       string
	Kind         string // "template" / "env" / "extension"
	Segment      string // canonical segment string ("fna", "bgo", "ei", ...) when applicable
	Code         string // the offending code
	TemplateID   string // resolved template id (for category errors)
	UnknownCount int
}

func (e *ResolveError) Error() string {
	return i18n.Tf("preset.resolve_failed", e.Reason)
}

// Resolve looks up every code in spec against registry, validating
// template existence, category, and environment source. Returns a ResolvedSpec ready
// for Apply, or a *ResolveError describing the first failure.
func Resolve(spec Spec, registry *template.Registry) (ResolvedSpec, error) {
	if registry == nil {
		return ResolvedSpec{}, &ResolveError{Reason: i18n.T("preset.registry_required")}
	}
	if len(spec.UnknownSegments) > 0 {
		// Caller decides how strict to be; we surface them so the
		// envelope can echo them back. The first call site (createcmd
		// pre-flight) treats this as fail-fast PRESET_INVALID; future
		// versions may downgrade to warnings.
		return ResolvedSpec{Spec: spec}, &ResolveError{
			Reason:       i18n.T("preset.unknown_segments"),
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
				Reason:  i18n.Tf("preset.template_not_registered", it.TemplateCode),
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
		out.Items = append(out.Items, ri)
	}

	if spec.EnvCode != "" {
		envID := EnvProviderForCode(spec.EnvCode[0])
		if envID == "" {
			return ResolvedSpec{}, &ResolveError{
				Reason:  i18n.Tf("preset.env_not_registered", spec.EnvCode),
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
// error contexts ("fna", "bgo", "ltl"). Note: the *resolved* segment,
// not necessarily what the user typed (parser already validated shape).
func itemSegmentString(it Item) string {
	var sb strings.Builder
	sb.WriteByte(byte(it.Kind))
	sb.WriteString(it.TemplateCode)
	return sb.String()
}

func kindCategoryMismatch(k Kind, tpl *template.Template) string {
	expected := map[Kind]template.Category{
		KindFrontend: template.CategoryFrontend,
		KindBackend:  template.CategoryBackend,
		KindLibrary:  template.CategoryLibrary,
	}[k]
	if tpl.Category != expected {
		return i18n.Tf("preset.category_mismatch",
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
