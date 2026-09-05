package creation

import (
	"context"
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/preset"
)

// PresetResult is what `one create --preset` reports back. The shape is
// stable and feeds directly into the create/v3 envelope builder.
type PresetResult struct {
	// PresetID is the canonical (re-encoded) id, so the envelope echoes
	// it back even if the user passed an equivalent form (different
	// segment order, optional preset: prefix).
	PresetID string
	// EnvProvider is the workspace-level env provider id ("dotenv" /
	// "infisical"). Empty when the preset didn't declare one — caller
	// treats that as the workspace default.
	EnvProvider string
	// Projects is one entry per landed project, in apply order. When
	// Apply returns mid-way through a multi-project preset, this list
	// contains the projects that *did* land before the failure.
	Projects []ProjectResult
	// UnknownSegments mirrors Spec.UnknownSegments so the envelope can
	// echo "this CLI ignored these future-version segments".
	UnknownSegments []string
}

// PresetOptions carries optional caller-controlled knobs for a preset run.
// ProjectNames, when present, must contain one name per project in the same
// canonical apply order Apply uses: backend, frontend, library.
type PresetOptions struct {
	ProjectNames []string
}

// applyOrder defines the project apply order: backend first (frontends
// consume backend ports), then frontend, then library. Within a kind,
// projects are applied in canonical iteration order over the ResolvedSpec
// (which Canonicalize already sorted by template code).
var applyOrder = []preset.Kind{preset.KindBackend, preset.KindFrontend, preset.KindLibrary}

// ApplyPreset renders every project segment in resolved into projectRoot,
// upserts the manifest, and runs local/deployment infra sync per project. CI
// is not generated implicitly.
//
// Apply assumes:
//   - resolved came from Resolve() against the current registry, so
//     every Item.Template is non-nil and every Item.Deploy is either
//     "" (template default) or already compat-checked.
//   - The workspace skeleton and Backend selection already exist.
//
// On mid-flight failure, Apply returns the partial ApplyResult plus the
// error. The creation service inspects PresetResult.Projects to set
// envelope.partial_state.
func ApplyPreset(ctx context.Context, projectRoot string, resolved preset.ResolvedSpec, options PresetOptions) (PresetResult, error) {
	out := PresetResult{
		EnvProvider:     resolved.EnvProvider,
		UnknownSegments: append([]string(nil), resolved.Spec.UnknownSegments...),
	}

	id, err := preset.Encode(resolved.Spec)
	if err != nil {
		return out, fmt.Errorf("creation: encode canonical preset id: %w", err)
	}
	out.PresetID = id

	// Group items by kind so we can process backends before frontends,
	// frontends before libraries — matching the apply order contract.
	byKind := map[preset.Kind][]preset.ResolvedItem{}
	for _, it := range resolved.Items {
		byKind[it.Item.Kind] = append(byKind[it.Item.Kind], it)
	}

	// Track template-code occurrence count so duplicate segments
	// (`fna.fna`) get deterministic project names: nextjs-app, nextjs-app-2, ...
	// workspace.UpsertManifestProject's existing dedup is overlaid on
	// the filesystem, but the manifest project name and target dir are
	// what we hand to materializeProject — and they need to be unique up
	// front.
	seenByCode := map[string]int{}

	customNameIndex := 0
	for _, k := range applyOrder {
		for _, it := range byKind[k] {
			n := seenByCode[it.Item.TemplateCode]
			seenByCode[it.Item.TemplateCode]++
			name := projectNameFor(it.Template, n)
			if len(options.ProjectNames) > 0 {
				name = options.ProjectNames[customNameIndex]
				customNameIndex++
			}

			res, applyErr := materializeProject(ctx, projectRoot, ProjectInput{
				Template:  it.Template,
				Name:      name,
				Deploy:    it.Deploy,
				Container: it.Container,
			})
			if applyErr != nil {
				return out, applyErr
			}
			out.Projects = append(out.Projects, res)
		}
	}

	return out, nil
}

// projectNameFor picks the default subproject name for a template,
// suffixed with "-2", "-3", ... when the same template code appears
// multiple times in the preset. The base name uses the template id
// (kebab-case, already validated).
//
// Examples:
//   - one go-api segment            → "go-api"
//   - two go-api segments           → "go-api", "go-api-2"
//   - three nextjs-app segments     → "nextjs-app", "nextjs-app-2", "nextjs-app-3"
func projectNameFor(tpl *template.Template, occurrence int) string {
	if tpl == nil {
		return ""
	}
	base := tpl.ID
	if occurrence == 0 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, occurrence+1)
}

// SummarizeDeploys flattens the result's project deploy backends into a
// {"kustomize": N, "vercel": M, ...} count map for the envelope. Empty
// deploy backends (templates with no deploy domain) are excluded.
func (r PresetResult) SummarizeDeploys() map[string]int {
	out := map[string]int{}
	for _, p := range r.Projects {
		if p.DeployBackend != "" {
			out[p.DeployBackend]++
		}
	}
	return out
}

// EffectiveEnvProvider returns the workspace env provider that should
// be written to the manifest. preset's `e<code>` segment wins; absent
// segment falls through to "" so the caller can layer the
// --env-provider flag default ("dotenv").
func (r PresetResult) EffectiveEnvProvider() string {
	return r.EnvProvider
}
