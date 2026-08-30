package template

// Template ↔ workspace backend compatibility checks. The metadata source
// is registry.json's per-template `compat` field (typed as
// `Template.Compat`). The output is a list of human-readable warnings,
// never an error: incompatibility is informative, not fatal.
//
// Caller: `one add <tpl>` — after rendering the subproject, compare its
// `compat` map against the existing workspace selection. Surface any
// mismatch so the user knows e.g. "kustomize won't help your
// starlight-docs subproject deploy".
//
// Semantics of compat[domain]:
//
//   - missing key       → no constraint (every backend in this domain
//                         is fine)
//   - non-empty list    → only these bare backend names are compatible
//   - empty list `[]`   → this template doesn't participate in this
//                         domain (mobile/library/electron for "deploy")

import (
	"fmt"
	"sort"
	"strings"
)

// Warning is one compat-mismatch line surfaced to the user.
type Warning struct {
	// Domain is the affected domain (e.g. "deploy").
	Domain string
	// SelectedID is the workspace-level backend id currently set in the
	// manifest for that domain (namespaced form, e.g. "deploy/k8s").
	SelectedID string
	// AllowedIDs is the template's compat whitelist for that domain.
	AllowedIDs []string
	// TemplateID is the template owning the constraint that fired.
	TemplateID string
	// SubprojectName is non-empty when the warning targets a particular
	// subproject.
	SubprojectName string
}

// Message renders the warning into a one-line UI string.
func (w Warning) Message() string {
	allowed := strings.Join(w.AllowedIDs, ", ")
	switch {
	case w.SubprojectName != "":
		return fmt.Sprintf(
			"工作区当前 %s/%s 与 subproject %q（模板 %s）不兼容；该模板的 %s 域支持: [%s]",
			w.Domain, w.SelectedID, w.SubprojectName, w.TemplateID, w.Domain, allowed)
	default:
		return fmt.Sprintf(
			"工作区当前 %s/%s 不适用于模板 %s；该模板的 %s 域支持: [%s]。建议手动调整 manifest 切到 %s，或保持现状（仅其他 subproject 走 %s）",
			w.Domain, strings.TrimPrefix(w.SelectedID, w.Domain+"/"),
			w.TemplateID, w.Domain, allowed,
			firstID(w.AllowedIDs), w.Domain,
		)
	}
}

// firstID returns the first id in a sorted slice (deterministic
// suggestion for the prompt). Empty string if list is empty.
func firstID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	cp := append([]string{}, ids...)
	sort.Strings(cp)
	return cp[0]
}

// CheckAllowedBackends compares one template's compat map against a
// workspace backend selection (domain → namespaced id). Returns a
// (possibly empty) slice of warnings, in canonical domain order.
//
// The selection map is keyed by domain ("env" / "ci" / "dev" /
// "container" / "deploy") with values being namespaced backend ids
// ("env/dotenv", "deploy/kustomize", ...). A domain missing from
// selection is treated as "user opted out" — never a warning, regardless
// of the template's compat whitelist.
//
// subprojectName, when non-empty, populates the Warning's
// SubprojectName field so the message can name which subproject the
// mismatch affects.
func CheckAllowedBackends(tpl Template, selection map[string]string, subprojectName string) []Warning {
	if len(tpl.Compat) == 0 || len(selection) == 0 {
		return nil
	}
	var warnings []Warning

	// Iterate domains in deterministic alphabetical order so warning
	// output is stable across runs.
	domains := make([]string, 0, len(tpl.Compat))
	for d := range tpl.Compat {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	for _, domain := range domains {
		// Compat values are bare backend names ("kustomize", "s3").
		// Build the namespaced form for comparison against the legacy
		// selection map (which still uses "deploy/kustomize" form).
		bareAllowed := tpl.Compat[domain]
		allowedIDs := make([]string, 0, len(bareAllowed))
		for _, b := range bareAllowed {
			allowedIDs = append(allowedIDs, domain+"/"+b)
		}
		selected, hasSelection := selection[domain]
		if !hasSelection || selected == "" {
			// User skipped this domain — no warning, even if compat is
			// non-empty.
			continue
		}
		if len(bareAllowed) == 0 {
			// `[]` means "this template doesn't participate in this
			// domain at all" (mobile/library/electron for deploy).
			continue
		}
		if contains(allowedIDs, selected) {
			continue
		}
		warnings = append(warnings, Warning{
			Domain:         domain,
			SelectedID:     selected,
			AllowedIDs:     allowedIDs,
			TemplateID:     tpl.ID,
			SubprojectName: subprojectName,
		})
	}
	return warnings
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
