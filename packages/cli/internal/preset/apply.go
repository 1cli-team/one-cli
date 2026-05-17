package preset

import (
	"context"
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/ai"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// ApplyResult is what `one create --preset` reports back. The shape is
// stable and feeds directly into createcmd's v3 envelope builder.
type ApplyResult struct {
	// PresetID is the canonical (re-encoded) id, so the envelope echoes
	// it back even if the user passed an equivalent form (different
	// segment order, optional preset: prefix).
	PresetID string
	// EnvProvider is the workspace-level env provider id ("dotenv" /
	// "infisical"). Empty when the preset didn't declare one — caller
	// (createcmd) treats that as the workspace default.
	EnvProvider string
	// Projects is one entry per landed project, in apply order. When
	// Apply returns mid-way through a multi-project preset, this list
	// contains the projects that *did* land before the failure.
	Projects []ProjectResult
	// Guides is the result of the final ai.Refresh call. Empty when no
	// projects landed (Refresh is skipped).
	Guides ai.RefreshResult
	// UnknownSegments mirrors Spec.UnknownSegments so the envelope can
	// echo "this CLI ignored these future-version segments".
	UnknownSegments []string
}

// ApplyOptions carries optional caller-controlled knobs for a preset run.
// ProjectNames, when present, must contain one name per project in the same
// canonical apply order Apply uses: backend, frontend, library.
type ApplyOptions struct {
	ProjectNames []string
}

// applyOrder defines the project apply order: backend first (frontends
// consume backend ports), then frontend, then library. Within a kind,
// projects are applied in canonical iteration order over the ResolvedSpec
// (which Canonicalize already sorted by template code).
var applyOrder = []Kind{KindBackend, KindFrontend, KindLibrary}

// Apply renders every project segment in resolved into projectRoot,
// upserts the manifest, and runs infra + CI sync per project. ai.Refresh
// runs exactly once at the end (skipped if zero projects landed).
//
// Apply assumes:
//   - resolved came from Resolve() against the current registry, so
//     every Item.Template is non-nil and every Item.Deploy is either
//     "" (template default) or already compat-checked.
//   - The workspace skeleton at projectRoot is already scaffolded
//     (scaffold.Generate + workspace.ApplyBackendSelection ran).
//
// On mid-flight failure, Apply returns the partial ApplyResult plus the
// error. createcmd inspects ApplyResult.Projects to set
// envelope.partial_state.
func Apply(ctx context.Context, projectRoot string, resolved ResolvedSpec, options ApplyOptions) (ApplyResult, error) {
	out := ApplyResult{
		EnvProvider:     resolved.EnvProvider,
		UnknownSegments: append([]string(nil), resolved.Spec.UnknownSegments...),
	}

	canonical := canonicalize(resolved.Spec)
	id, err := Encode(canonical)
	if err != nil {
		return out, fmt.Errorf("preset.Apply: encode canonical id: %w", err)
	}
	out.PresetID = id

	// Group items by kind so we can process backends before frontends,
	// frontends before libraries — matching the apply order contract.
	byKind := map[Kind][]ResolvedItem{}
	for _, it := range resolved.Items {
		byKind[it.Item.Kind] = append(byKind[it.Item.Kind], it)
	}

	// Track template-code occurrence count so duplicate segments
	// (`fna.fna`) get deterministic project names: nextjs-app, nextjs-app-2, ...
	// workspace.UpsertManifestProject's existing dedup is overlaid on
	// the filesystem, but the manifest project name and target dir are
	// what we hand to ApplyProject — and they need to be unique up
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

			res, applyErr := ApplyProject(ctx, projectRoot, ProjectInput{
				Template:  it.Template,
				Name:      name,
				Deploy:    it.Deploy,
				Container: it.Container,
			}, false)
			if applyErr != nil {
				return out, applyErr
			}
			out.Projects = append(out.Projects, res)
		}
	}

	if len(out.Projects) > 0 {
		// AI-guides refresh: aggregates each landed project's ai/
		// guides into AGENTS.md / CLAUDE.md. Runs once per Apply.
		out.Guides = ai.Refresh(projectRoot, false)
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
func (r ApplyResult) SummarizeDeploys() map[string]int {
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
// segment falls through to "" so the caller (createcmd) can layer the
// --env-provider flag default ("dotenv").
func (r ApplyResult) EffectiveEnvProvider() string {
	return r.EnvProvider
}

// ResolveEnvWithFlag combines a preset env code with an explicit
// --env-provider flag value, surfacing PRESET_FLAG_CONFLICT when both
// are set and disagree. flagValue may be "" (no flag); presetValue may
// be "" (preset omitted env segment).
func ResolveEnvWithFlag(presetValue, flagValue string) (string, error) {
	preset := presetValue
	flag := flagValue
	if flag == "" {
		return preset, nil
	}
	if preset == "" {
		return flag, nil
	}
	if preset != flag {
		return "", &EnvConflictError{Preset: preset, Flag: flag}
	}
	return preset, nil
}

// EnvConflictError signals that --env-provider was passed alongside a
// preset whose `e` segment named a different provider. Callers
// translate this into PRESET_FLAG_CONFLICT envelope.
type EnvConflictError struct {
	Preset string
	Flag   string
}

func (e *EnvConflictError) Error() string {
	return fmt.Sprintf("preset declared env=%q but --env-provider %q was also passed", e.Preset, e.Flag)
}

// PreflightDirCheck wraps a couple of workspace.* checks Apply needs
// before any filesystem mutation. createcmd already runs the same
// checks, so this is here mostly for symmetry with future call sites.
var _ = workspace.HasManifest // imported for symmetry; remove if unused
